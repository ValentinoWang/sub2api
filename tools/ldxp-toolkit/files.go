package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var timeNowUTC = func() time.Time { return time.Now().UTC() }

func ensurePrivateDataDir(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return errors.New("data directory is required")
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat data directory: %w", err)
	}
	if !info.IsDir() {
		return errors.New("configured data directory is not a directory")
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("secure data directory: %w", err)
	}
	return nil
}

func writePrivateJSON(dir, prefix string, value any) (string, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode local summary: %w", err)
	}
	return writePrivateFile(dir, prefix, ".json", data)
}

func writePrivateFile(dir, prefix, extension string, data []byte) (string, error) {
	if err := ensurePrivateDataDir(dir); err != nil {
		return "", err
	}
	prefix = safeFilePart(prefix)
	if prefix == "" {
		prefix = "ldxp"
	}
	if extension == "" || !strings.HasPrefix(extension, ".") {
		extension = ".dat"
	}
	if filepath.Base(extension) != extension || strings.ContainsAny(extension, `/\\`) {
		return "", errors.New("private file extension must not contain a path")
	}
	for attempt := 0; attempt < 10; attempt++ {
		name := fmt.Sprintf("%s-%d-%d%s", prefix, timeNowUTC().UnixNano(), attempt, extension)
		path := filepath.Join(dir, name)
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			if os.IsExist(err) {
				continue
			}
			return "", fmt.Errorf("create private file: %w", err)
		}
		writeErr := func() error {
			if _, err := file.Write(data); err != nil {
				return err
			}
			if err := file.Sync(); err != nil {
				return err
			}
			return nil
		}()
		closeErr := file.Close()
		if writeErr != nil {
			_ = os.Remove(path)
			return "", fmt.Errorf("write private file: %w", writeErr)
		}
		if closeErr != nil {
			_ = os.Remove(path)
			return "", fmt.Errorf("close private file: %w", closeErr)
		}
		if err := os.Chmod(path, 0o600); err != nil {
			_ = os.Remove(path)
			return "", fmt.Errorf("secure private file: %w", err)
		}
		return path, nil
	}
	return "", errors.New("could not allocate a unique private file name")
}

func safeFilePart(value string, secrets ...string) string {
	value = redactText(value, secrets...)
	var builder strings.Builder
	for _, char := range value {
		safe := (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' || char == '.'
		if safe {
			builder.WriteRune(char)
		} else {
			builder.WriteByte('_')
		}
	}
	return strings.Trim(builder.String(), "._-")
}
