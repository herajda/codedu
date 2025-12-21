package main

import (
	"encoding/base64"
	"encoding/json"
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

type TestFilePayload struct {
	Name    string `json:"name"`
	Content string `json:"content"` // base64 encoded content
}

func normalizeTestFilesPayload(files []TestFilePayload) (*string, error) {
	if len(files) == 0 {
		return nil, nil
	}
	validFiles := []TestFilePayload{}
	for _, f := range files {
		name := strings.TrimSpace(f.Name)
		raw := strings.TrimSpace(f.Content)
		if name == "" {
			return nil, fmt.Errorf("file name is required")
		}
		clean := filepath.Base(name)
		if clean == "." || clean == "" {
			return nil, fmt.Errorf("invalid file name: %s", name)
		}
		if raw == "" {
			return nil, fmt.Errorf("file content is empty for %s", name)
		}
		// Validate base64
		if _, err := base64.StdEncoding.DecodeString(raw); err != nil {
			return nil, fmt.Errorf("invalid base64 content for %s", name)
		}
		validFiles = append(validFiles, TestFilePayload{Name: clean, Content: raw})
	}
	if len(validFiles) == 0 {
		return nil, nil
	}
	bytes, err := json.Marshal(validFiles)
	if err != nil {
		return nil, err
	}
	s := string(bytes)
	return &s, nil
}

func stageTestFile(dir, mainFile string, tc TestCase) error {
	if tc.FilesJSON != nil && *tc.FilesJSON != "" {
		var files []TestFilePayload
		if err := json.Unmarshal([]byte(*tc.FilesJSON), &files); err != nil {
			return fmt.Errorf("invalid files_json: %w", err)
		}
		for _, f := range files {
			if err := writeTestFile(dir, mainFile, f.Name, f.Content); err != nil {
				return err
			}
		}
		return nil
	}

	name := strings.TrimSpace(stringOrEmpty(tc.FileName))
	raw := strings.TrimSpace(stringOrEmpty(tc.FileBase64))
	if name == "" && raw == "" {
		// No legacy file either
		return nil
	}
	return writeTestFile(dir, mainFile, name, raw)
}

func writeTestFile(dir, mainFile, name, raw string) error {
	name = strings.TrimSpace(name)
	raw = strings.TrimSpace(raw)
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
	fmt.Printf("[worker] stageTestFile: wrote %s (%d bytes) to %s\n", clean, len(data), target)

	mainDir := strings.TrimSpace(filepath.Dir(mainFile))
	if mainDir != "" && mainDir != "." {
		altTarget := filepath.Join(dir, mainDir, clean)
		if err := os.MkdirAll(filepath.Dir(altTarget), 0755); err != nil {
			return fmt.Errorf("prepare test file directory: %w", err)
		}
		if err := os.WriteFile(altTarget, data, 0644); err != nil {
			return fmt.Errorf("write test file in module dir: %w", err)
		}
		fmt.Printf("[worker] stageTestFile: wrote copy to %s\n", altTarget)
	}
	return nil
}
