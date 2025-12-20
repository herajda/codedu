package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func normalizeTestFilePayload(fileName, fileBase64 *string) (*string, *string, error) {
	name := strings.TrimSpace(stringOrEmpty(fileName))
	raw := strings.TrimSpace(stringOrEmpty(fileBase64))
	if name == "" && raw == "" {
		return nil, nil, nil
	}
	if name == "" {
		return nil, nil, fmt.Errorf("file_name is required when file_base64 is provided")
	}
	if raw == "" && fileBase64 == nil {
		return nil, nil, fmt.Errorf("file_base64 is required when file_name is provided")
	}
	clean := filepath.Base(name)
	if clean == "." || clean == "" {
		return nil, nil, fmt.Errorf("invalid file_name")
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("file_base64 must be valid base64")
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	return &clean, &encoded, nil
}

func stageTestFile(dir, mainFile string, tc TestCase) error {
	name := strings.TrimSpace(stringOrEmpty(tc.FileName))
	raw := strings.TrimSpace(stringOrEmpty(tc.FileBase64))
	if name == "" && raw == "" {
		return nil
	}
	if name == "" {
		return fmt.Errorf("test file missing file_name")
	}
	if raw == "" {
		return fmt.Errorf("test file missing file_base64")
	}
	clean := filepath.Base(name)
	if clean == "." || clean == "" {
		return fmt.Errorf("invalid test file name")
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return fmt.Errorf("invalid test file content")
	}
	target := filepath.Join(dir, clean)
	if err := os.WriteFile(target, data, 0644); err != nil {
		return fmt.Errorf("write test file: %w", err)
	}
	mainDir := strings.TrimSpace(filepath.Dir(mainFile))
	if mainDir != "" && mainDir != "." {
		altTarget := filepath.Join(dir, mainDir, clean)
		if err := os.MkdirAll(filepath.Dir(altTarget), 0755); err != nil {
			return fmt.Errorf("prepare test file directory: %w", err)
		}
		if err := os.WriteFile(altTarget, data, 0644); err != nil {
			return fmt.Errorf("write test file in module dir: %w", err)
		}
	}
	return nil
}
