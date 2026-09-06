package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultTargetStock   = 50000
	defaultUploadSegment = 1000
	maxConfigBytes       = 4 << 20
)

// LDXPConfig contains the merchant API origin and its server-side token.
type LDXPConfig struct {
	BaseURL       string `json:"base_url"`
	MerchantToken string `json:"merchant_token"`
}

// Sub2APIConfig contains the administrative API origin and bearer token.
type Sub2APIConfig struct {
	BaseURL    string `json:"base_url"`
	AdminToken string `json:"admin_token"`
}

// ProductMapping is the fixed commercial mapping used by the preview plan.
type ProductMapping struct {
	GoodsID   int64   `json:"goods_id"`
	CNYAmount int     `json:"cny_amount"`
	USDCredit float64 `json:"usd_credit"`
	Enabled   bool    `json:"enabled"`
}

// Config is intentionally small and JSON-based so the CLI has no dependency
// on the backend's Go module or its encryption implementation.
type Config struct {
	LDXP            LDXPConfig       `json:"ldxp"`
	Sub2API         Sub2APIConfig    `json:"sub2api"`
	DataDir         string           `json:"data_dir"`
	TargetStock     int              `json:"target_stock"`
	UploadSegment   int              `json:"upload_segment"`
	ProductMappings []ProductMapping `json:"product_mappings"`

	configPath string
}

// UnmarshalJSON accepts the documented nested form and the flat names that
// are convenient for shell-generated configuration files.
func (c *Config) UnmarshalJSON(input []byte) error {
	var raw struct {
		LDXP              *LDXPConfig      `json:"ldxp"`
		Sub2API           *Sub2APIConfig   `json:"sub2api"`
		LDXPBaseURL       string           `json:"ldxp_base_url"`
		MerchantToken     string           `json:"merchant_token"`
		LDXPMerchantToken string           `json:"ldxp_merchant_token"`
		Sub2APIBaseURL    string           `json:"sub2api_base_url"`
		AdminToken        string           `json:"admin_token"`
		Sub2APIAdminToken string           `json:"sub2api_admin_token"`
		DataDir           string           `json:"data_dir"`
		DataDirectory     string           `json:"data_directory"`
		TargetStock       *int             `json:"target_stock"`
		UploadSegment     *int             `json:"upload_segment"`
		ProductMappings   []ProductMapping `json:"product_mappings"`
		Mappings          []ProductMapping `json:"mappings"`
		Products          []ProductMapping `json:"products"`
	}
	if err := json.Unmarshal(input, &raw); err != nil {
		return err
	}

	var cfg Config
	if raw.LDXP != nil {
		cfg.LDXP = *raw.LDXP
	}
	if strings.TrimSpace(cfg.LDXP.BaseURL) == "" {
		cfg.LDXP.BaseURL = raw.LDXPBaseURL
	}
	if strings.TrimSpace(cfg.LDXP.MerchantToken) == "" {
		cfg.LDXP.MerchantToken = raw.LDXPMerchantToken
		if strings.TrimSpace(cfg.LDXP.MerchantToken) == "" {
			cfg.LDXP.MerchantToken = raw.MerchantToken
		}
	}
	if raw.Sub2API != nil {
		cfg.Sub2API = *raw.Sub2API
	}
	if strings.TrimSpace(cfg.Sub2API.BaseURL) == "" {
		cfg.Sub2API.BaseURL = raw.Sub2APIBaseURL
	}
	if strings.TrimSpace(cfg.Sub2API.AdminToken) == "" {
		cfg.Sub2API.AdminToken = raw.Sub2APIAdminToken
		if strings.TrimSpace(cfg.Sub2API.AdminToken) == "" {
			cfg.Sub2API.AdminToken = raw.AdminToken
		}
	}
	cfg.DataDir = raw.DataDir
	if strings.TrimSpace(cfg.DataDir) == "" {
		cfg.DataDir = raw.DataDirectory
	}
	if raw.TargetStock != nil {
		cfg.TargetStock = *raw.TargetStock
	} else {
		cfg.TargetStock = defaultTargetStock
	}
	if raw.UploadSegment != nil {
		cfg.UploadSegment = *raw.UploadSegment
	} else {
		cfg.UploadSegment = defaultUploadSegment
	}
	cfg.ProductMappings = raw.ProductMappings
	if cfg.ProductMappings == nil {
		cfg.ProductMappings = raw.Mappings
	}
	if cfg.ProductMappings == nil {
		cfg.ProductMappings = raw.Products
	}

	*c = cfg
	return nil
}

func loadConfig(path string) (*Config, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("config path is required")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve config path: %w", err)
	}
	file, err := os.Open(absPath)
	if err != nil {
		return nil, fmt.Errorf("open config: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(io.LimitReader(file, maxConfigBytes))
	var cfg Config
	if err := decoder.Decode(&cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, errors.New("config contains more than one JSON value")
		}
		return nil, fmt.Errorf("decode config trailer: %w", err)
	}
	cfg.configPath = absPath
	if cfg.DataDir != "" && !filepath.IsAbs(cfg.DataDir) {
		cfg.DataDir = filepath.Join(filepath.Dir(absPath), cfg.DataDir)
	}
	return &cfg, nil
}

func parseOrigin(raw, label string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("%s base URL is required", label)
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("%s base URL must be an absolute HTTP(S) origin", label)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("%s base URL must use http or https", label)
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return nil, fmt.Errorf("%s base URL must contain only an origin", label)
	}
	u.Path = ""
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u, nil
}

func validateConfig(cfg *Config, needSub2API, needDataDir bool) error {
	if cfg == nil {
		return errors.New("config is required")
	}
	if _, err := parseOrigin(cfg.LDXP.BaseURL, "LDXP"); err != nil {
		return err
	}
	if strings.TrimSpace(cfg.LDXP.MerchantToken) == "" {
		return errors.New("LDXP merchant token is required")
	}
	if needSub2API {
		if _, err := parseOrigin(cfg.Sub2API.BaseURL, "Sub2API"); err != nil {
			return err
		}
		if strings.TrimSpace(cfg.Sub2API.AdminToken) == "" {
			return errors.New("Sub2API admin token is required")
		}
	}
	if needDataDir && strings.TrimSpace(cfg.DataDir) == "" {
		return errors.New("data_dir is required for this command")
	}
	if cfg.TargetStock < 0 {
		return errors.New("target_stock must be non-negative")
	}
	if cfg.UploadSegment <= 0 {
		return errors.New("upload_segment must be positive")
	}
	seenGoods := make(map[int64]struct{}, len(cfg.ProductMappings))
	seenCNY := make(map[int]struct{}, len(cfg.ProductMappings))
	for _, mapping := range cfg.ProductMappings {
		if mapping.GoodsID <= 0 {
			return errors.New("each product mapping requires a positive goods_id")
		}
		if mapping.CNYAmount <= 0 {
			return errors.New("each product mapping requires a positive cny_amount")
		}
		if mapping.USDCredit <= 0 {
			return errors.New("each product mapping requires a positive usd_credit")
		}
		if _, exists := seenGoods[mapping.GoodsID]; exists {
			return fmt.Errorf("duplicate product mapping goods_id %d", mapping.GoodsID)
		}
		if _, exists := seenCNY[mapping.CNYAmount]; exists {
			return fmt.Errorf("duplicate product mapping cny_amount %d", mapping.CNYAmount)
		}
		seenGoods[mapping.GoodsID] = struct{}{}
		seenCNY[mapping.CNYAmount] = struct{}{}
	}
	return nil
}

func configSummary(cfg *Config, configPath string) map[string]any {
	ldxpOrigin, _ := parseOrigin(cfg.LDXP.BaseURL, "LDXP")
	subOrigin, _ := parseOrigin(cfg.Sub2API.BaseURL, "Sub2API")
	result := map[string]any{
		"valid":                     true,
		"config_path":               configPath,
		"merchant_token_configured": strings.TrimSpace(cfg.LDXP.MerchantToken) != "",
		"admin_token_configured":    strings.TrimSpace(cfg.Sub2API.AdminToken) != "",
		"data_dir":                  cfg.DataDir,
		"target_stock":              cfg.TargetStock,
		"upload_segment":            cfg.UploadSegment,
		"product_mappings":          len(cfg.ProductMappings),
	}
	if ldxpOrigin != nil {
		result["ldxp_base_origin"] = ldxpOrigin.String()
	}
	if subOrigin != nil {
		result["sub2api_base_origin"] = subOrigin.String()
	}
	return result
}
