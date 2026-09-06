package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr))
}

func runCLI(args []string, stdout, stderr io.Writer) int {
	configPath, commandArgs, err := extractConfigFlag(args)
	if err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err)
		printUsage(stderr)
		return 2
	}
	if len(commandArgs) == 0 || hasHelpFlag(commandArgs) {
		printUsage(stdout)
		return 0
	}
	cfg, err := loadConfig(configPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %s\n", err)
		return 1
	}
	command := commandArgs[0]
	commandArgs = commandArgs[1:]
	var commandErr error
	switch command {
	case "doctor":
		commandErr = commandDoctor(cfg, configPath, commandArgs, stdout, stderr)
	case "goods":
		commandErr = commandGoods(cfg, commandArgs, stdout, stderr)
	case "config":
		commandErr = commandConfig(cfg, configPath, commandArgs, stdout, stderr)
	case "restock":
		commandErr = commandRestock(cfg, commandArgs, stdout, stderr)
	case "jobs":
		commandErr = commandJobs(cfg, commandArgs, stdout, stderr)
	case "export":
		commandErr = commandExport(cfg, commandArgs, stdout, stderr)
	default:
		commandErr = fmt.Errorf("unknown command %q", command)
	}
	if commandErr != nil {
		fmt.Fprintf(stderr, "error: %s\n", redactText(commandErr.Error(), cfg.LDXP.MerchantToken, cfg.Sub2API.AdminToken))
		return 1
	}
	return 0
}

func extractConfigFlag(args []string) (string, []string, error) {
	var configPath string
	remaining := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch {
		case arg == "--config":
			if index+1 >= len(args) || strings.TrimSpace(args[index+1]) == "" {
				return "", nil, errors.New("--config requires a path")
			}
			configPath = args[index+1]
			index++
		case strings.HasPrefix(arg, "--config="):
			configPath = strings.TrimPrefix(arg, "--config=")
			if strings.TrimSpace(configPath) == "" {
				return "", nil, errors.New("--config requires a path")
			}
		default:
			remaining = append(remaining, arg)
		}
	}
	if strings.TrimSpace(configPath) == "" {
		return "", nil, errors.New("explicit --config path is required")
	}
	return configPath, remaining, nil
}

func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" {
			return true
		}
	}
	return false
}

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage: ldxp-toolkit --config PATH <command>")
	_, _ = fmt.Fprintln(w, "Commands:")
	_, _ = fmt.Fprintln(w, "  doctor")
	_, _ = fmt.Fprintln(w, "  goods list")
	_, _ = fmt.Fprintln(w, "  config validate")
	_, _ = fmt.Fprintln(w, "  restock preview")
	_, _ = fmt.Fprintln(w, "  restock run")
	_, _ = fmt.Fprintln(w, "  jobs status --id ID")
	_, _ = fmt.Fprintln(w, "  jobs resume --id ID")
	_, _ = fmt.Fprintln(w, "  export --id ID")
}

func commandDoctor(cfg *Config, configPath string, args []string, stdout, stderr io.Writer) error {
	if len(args) != 0 {
		return errors.New("doctor does not accept positional arguments")
	}
	if err := validateConfig(cfg, true, true); err != nil {
		return err
	}
	result := configSummary(cfg, configPath)
	if info, err := os.Stat(configPath); err == nil {
		result["config_mode"] = fmt.Sprintf("%04o", info.Mode().Perm())
		result["config_mode_secure"] = info.Mode().Perm()&0o077 == 0
	} else {
		return fmt.Errorf("stat config: %w", err)
	}
	if info, err := os.Stat(cfg.DataDir); err == nil {
		result["data_dir_exists"] = true
		result["data_dir_is_directory"] = info.IsDir()
		result["data_dir_mode"] = fmt.Sprintf("%04o", info.Mode().Perm())
	} else if os.IsNotExist(err) {
		result["data_dir_exists"] = false
	} else {
		return fmt.Errorf("stat data directory: %w", err)
	}
	return writeJSON(stdout, result, cfg.LDXP.MerchantToken, cfg.Sub2API.AdminToken)
}

func commandGoods(cfg *Config, args []string, stdout, _ io.Writer) error {
	if len(args) != 1 || args[0] != "list" {
		return errors.New("usage: goods list")
	}
	if err := validateConfig(cfg, false, false); err != nil {
		return err
	}
	client, err := newLDXPClient(cfg)
	if err != nil {
		return err
	}
	goods, err := client.listGoods(context.Background())
	if err != nil {
		return err
	}
	return writeJSON(stdout, map[string]any{"goods": goods}, cfg.LDXP.MerchantToken, cfg.Sub2API.AdminToken)
}

func commandConfig(cfg *Config, configPath string, args []string, stdout, _ io.Writer) error {
	if len(args) != 1 || args[0] != "validate" {
		return errors.New("usage: config validate")
	}
	if err := validateConfig(cfg, true, true); err != nil {
		return err
	}
	return writeJSON(stdout, configSummary(cfg, configPath), cfg.LDXP.MerchantToken, cfg.Sub2API.AdminToken)
}

func commandRestock(cfg *Config, args []string, stdout, _ io.Writer) error {
	if len(args) != 1 {
		return errors.New("usage: restock preview|run")
	}
	switch args[0] {
	case "preview":
		if err := validateConfig(cfg, true, false); err != nil {
			return err
		}
		if enabledMappingCount(cfg) == 0 {
			return errors.New("restock preview requires at least one enabled product mapping")
		}
		ldxp, err := newLDXPClient(cfg)
		if err != nil {
			return err
		}
		preview, err := buildRestockPreview(context.Background(), cfg, ldxp)
		if err != nil {
			return err
		}
		// This endpoint is a protocol-level dry run. It receives the computed
		// plan but cannot issue codes or upload inventory.
		sub2api, err := newSub2APIClient(cfg)
		if err != nil {
			return err
		}
		serverPreview, err := sub2api.preview(context.Background(), requestForConfig(cfg, preview.Items))
		if err != nil {
			return err
		}
		return writeJSON(stdout, map[string]any{
			"preview":        preview,
			"server_preview": envelopeForOutput(serverPreview, cfg.LDXP.MerchantToken, cfg.Sub2API.AdminToken),
		}, cfg.LDXP.MerchantToken, cfg.Sub2API.AdminToken)
	case "run":
		if err := validateConfig(cfg, true, true); err != nil {
			return err
		}
		if enabledMappingCount(cfg) == 0 {
			return errors.New("restock run requires at least one enabled product mapping")
		}
		sub2api, err := newSub2APIClient(cfg)
		if err != nil {
			return err
		}
		envelope, err := sub2api.run(context.Background(), requestForConfig(cfg, nil))
		if err != nil {
			return err
		}
		jobID := jobIDFromData(envelope.Data)
		summaryPath, err := saveJobSummary(cfg, "restock run", jobsRunPath, jobID, envelope)
		if err != nil {
			return err
		}
		return writeJSON(stdout, map[string]any{
			"job_id":       jobID,
			"summary_path": summaryPath,
			"response":     envelopeForOutput(envelope, cfg.LDXP.MerchantToken, cfg.Sub2API.AdminToken),
		}, cfg.LDXP.MerchantToken, cfg.Sub2API.AdminToken)
	default:
		return errors.New("usage: restock preview|run")
	}
}

func enabledMappingCount(cfg *Config) int {
	count := 0
	for _, mapping := range cfg.ProductMappings {
		if mapping.Enabled {
			count++
		}
	}
	return count
}

func commandJobs(cfg *Config, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: jobs status|resume --id ID")
	}
	if err := validateConfig(cfg, true, true); err != nil {
		return err
	}
	jobID, err := parseJobID(args[0], args[1:], stderr)
	if err != nil {
		return err
	}
	sub2api, err := newSub2APIClient(cfg)
	if err != nil {
		return err
	}
	var envelope *apiEnvelope
	var endpoint string
	switch args[0] {
	case "status":
		envelope, err = sub2api.status(context.Background(), jobID)
		endpoint, _ = jobPath(jobID, "")
	case "resume":
		envelope, err = sub2api.resume(context.Background(), jobID)
		endpoint, _ = jobPath(jobID, "/resume")
	default:
		return errors.New("usage: jobs status|resume --id ID")
	}
	if err != nil {
		return err
	}
	summaryPath, err := saveJobSummary(cfg, "jobs "+args[0], endpoint, jobID, envelope)
	if err != nil {
		return err
	}
	return writeJSON(stdout, map[string]any{
		"job_id":       jobID,
		"summary_path": summaryPath,
		"response":     envelopeForOutput(envelope, cfg.LDXP.MerchantToken, cfg.Sub2API.AdminToken),
	}, cfg.LDXP.MerchantToken, cfg.Sub2API.AdminToken)
}

func parseJobID(command string, args []string, stderr io.Writer) (string, error) {
	flags := flag.NewFlagSet("jobs "+command, flag.ContinueOnError)
	flags.SetOutput(stderr)
	id := flags.String("id", "", "job ID")
	if err := flags.Parse(args); err != nil {
		return "", err
	}
	idFlagProvided := false
	flags.Visit(func(value *flag.Flag) {
		if value.Name == "id" {
			idFlagProvided = true
		}
	})
	if idFlagProvided && flags.NArg() > 0 {
		return "", errors.New("jobs command does not accept positional arguments with --id")
	}
	if !idFlagProvided && flags.NArg() == 1 {
		*id = flags.Arg(0)
	}
	if flags.NArg() > 1 || strings.TrimSpace(*id) == "" {
		return "", errors.New("jobs command requires exactly one --id ID")
	}
	return *id, nil
}

func commandExport(cfg *Config, args []string, stdout, stderr io.Writer) error {
	if err := validateConfig(cfg, true, true); err != nil {
		return err
	}
	jobID, err := parseJobID("export", args, stderr)
	if err != nil {
		return err
	}
	sub2api, err := newSub2APIClient(cfg)
	if err != nil {
		return err
	}
	envelope, response, attachmentData, err := sub2api.export(context.Background(), jobID)
	if err != nil {
		return err
	}
	exportPath := ""
	data := attachmentData
	extension := ""
	if envelope != nil {
		data, extension, err = exportDataBytes(envelope.Data)
		if err != nil {
			return err
		}
	} else if data != nil {
		extension = exportExtension(response.Header.Get("Content-Type"))
	}
	if data != nil {
		prefix := "ldxp-export-" + safeFilePart(jobID, cfg.LDXP.MerchantToken, cfg.Sub2API.AdminToken)
		if strings.Contains(strings.ToLower(response.Header.Get("Content-Type")), "csv") {
			extension = ".csv"
		}
		if extension == "" {
			extension = ".dat"
		}
		exportPath, err = writePrivateFile(cfg.DataDir, prefix, extension, data)
		if err != nil {
			return err
		}
	}
	endpoint, _ := jobPath(jobID, "/export")
	if envelope == nil {
		envelope = &apiEnvelope{Code: 1, Msg: "export attachment received", Data: json.RawMessage("null")}
	}
	summaryPath, err := saveJobSummary(cfg, "export", endpoint, jobID, envelope)
	if err != nil {
		return err
	}
	return writeJSON(stdout, map[string]any{
		"job_id":       jobID,
		"summary_path": summaryPath,
		"export_path":  exportPath,
		"response":     envelopeForOutput(envelope, cfg.LDXP.MerchantToken, cfg.Sub2API.AdminToken),
	}, cfg.LDXP.MerchantToken, cfg.Sub2API.AdminToken)
}

func exportExtension(contentType string) string {
	contentType = strings.ToLower(contentType)
	switch {
	case strings.Contains(contentType, "csv"):
		return ".csv"
	case strings.Contains(contentType, "json"):
		return ".json"
	case strings.Contains(contentType, "text/plain"):
		return ".txt"
	default:
		return ".dat"
	}
}

func saveJobSummary(cfg *Config, command, endpoint, jobID string, envelope *apiEnvelope) (string, error) {
	data, err := marshalSummary(command, endpoint, jobID, envelope, cfg.LDXP.MerchantToken, cfg.Sub2API.AdminToken)
	if err != nil {
		return "", err
	}
	prefix := "ldxp-job-" + safeFilePart(jobID, cfg.LDXP.MerchantToken, cfg.Sub2API.AdminToken) + "-summary"
	return writePrivateFile(cfg.DataDir, prefix, ".json", data)
}

func writeJSON(w io.Writer, value any, secrets ...string) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return err
	}
	redacted := redactedJSON(encoded, secrets...)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(redacted)
}
