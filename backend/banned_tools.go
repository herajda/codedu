package main

import (
	"encoding/json"
	"sort"
	"strings"
)

type StructuredToolRule struct {
	Library  string `json:"library"`
	Function string `json:"function"`
	Note     string `json:"note,omitempty"`
}

type BannedToolsConfig struct {
	Mode       string               `json:"mode,omitempty"`
	Structured []StructuredToolRule `json:"structured,omitempty"`
	Advanced   []string             `json:"advanced,omitempty"`
}

func parseBannedToolsConfig(raw *string) (*BannedToolsConfig, error) {
	if raw == nil {
		return nil, nil
	}
	trimmed := strings.TrimSpace(*raw)
	if trimmed == "" {
		return nil, nil
	}
	var cfg BannedToolsConfig
	if err := json.Unmarshal([]byte(trimmed), &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func serializeBannedToolsConfig(cfg *BannedToolsConfig) (*string, error) {
	if cfg == nil {
		return nil, nil
	}
	if len(cfg.Structured) == 0 && len(cfg.Advanced) == 0 {
		return nil, nil
	}
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	out := string(data)
	return &out, nil
}

func (cfg *BannedToolsConfig) normalize() ([]string, []string, map[string]string) {
	if cfg == nil {
		return nil, nil, map[string]string{}
	}

	cfg.Mode = strings.ToLower(strings.TrimSpace(cfg.Mode))
	if cfg.Mode != "advanced" {
		cfg.Mode = "structured"
	}

	notes := make(map[string]string)
	funcSeen := make(map[string]struct{})
	modSeen := make(map[string]struct{})
	functions := make([]string, 0)
	modules := make([]string, 0)

	addFunc := func(pattern, note string) {
		pattern = strings.TrimSpace(strings.ToLower(pattern))
		if pattern == "" {
			return
		}
		key := pattern
		if _, exists := funcSeen[key]; !exists {
			functions = append(functions, pattern)
			funcSeen[key] = struct{}{}
		}
		if note = strings.TrimSpace(note); note != "" {
			notes[key] = note
		}
	}

	addModule := func(name, note string) {
		name = strings.TrimSpace(strings.ToLower(name))
		if name == "" {
			return
		}
		key := name
		if _, exists := modSeen[key]; !exists {
			modules = append(modules, name)
			modSeen[key] = struct{}{}
		}
		if note = strings.TrimSpace(note); note != "" {
			notes[key] = note
		}
	}

	for i := range cfg.Structured {
		lib := strings.TrimSpace(strings.ToLower(cfg.Structured[i].Library))
		fn := strings.TrimSpace(strings.ToLower(cfg.Structured[i].Function))
		note := strings.TrimSpace(cfg.Structured[i].Note)
		if fn == "" {
			fn = "*"
		}
		cfg.Structured[i].Library = lib
		cfg.Structured[i].Function = fn
		cfg.Structured[i].Note = note
		if lib == "" {
			continue
		}
		if fn == "*" {
			addModule(lib, note)
			addFunc(lib+".*", note)
		} else {
			addFunc(lib+"."+fn, note)
		}
	}

	normalizedAdvanced := make([]string, 0, len(cfg.Advanced))
	for _, entry := range cfg.Advanced {
		pieces := strings.Split(entry, "\n")
		for _, piece := range pieces {
			line := strings.TrimSpace(piece)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if idx := strings.Index(line, "#"); idx >= 0 {
				line = strings.TrimSpace(line[:idx])
				if line == "" {
					continue
				}
			}
			lower := strings.ToLower(line)
			normalizedAdvanced = append(normalizedAdvanced, lower)
			if strings.HasSuffix(lower, ".*") {
				base := strings.TrimSuffix(lower, ".*")
				addModule(base, "")
				addFunc(lower, "")
			} else if strings.Contains(lower, ".") {
				addFunc(lower, "")
			} else {
				addFunc(lower, "")
			}
		}
	}
	cfg.Advanced = normalizedAdvanced

	if len(functions) > 1 {
		sort.Strings(functions)
	}
	if len(modules) > 1 {
		sort.Strings(modules)
	}

	return functions, modules, notes
}

func buildNotesMapFromConfig(cfg *BannedToolsConfig) map[string]string {
	if cfg == nil {
		return map[string]string{}
	}
	_, _, notes := cfg.normalize()
	return notes
}

func cloneConfig(config *BannedToolsConfig) *BannedToolsConfig {
	if config == nil {
		return nil
	}
	out := &BannedToolsConfig{Mode: config.Mode}
	if len(config.Structured) > 0 {
		out.Structured = make([]StructuredToolRule, len(config.Structured))
		copy(out.Structured, config.Structured)
	}
	if len(config.Advanced) > 0 {
		out.Advanced = make([]string, len(config.Advanced))
		copy(out.Advanced, config.Advanced)
	}
	return out
}

func notesFromAssignment(a *Assignment) map[string]string {
	if a == nil || a.BannedToolRules == nil {
		return map[string]string{}
	}
	cfg, err := parseBannedToolsConfig(a.BannedToolRules)
	if err != nil {
		return map[string]string{}
	}
	return buildNotesMapFromConfig(cfg)
}
