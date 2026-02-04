package config

import (
	"path/filepath"
	"strings"
)

const (
	DefaultConfigDir  = "configs"
	DefaultConfigFile = "config.json"
	LegacyConfigDir   = ".codex/CONFIGS"
)

func DefaultConfigPath() string {
	return filepath.Join(DefaultConfigDir, DefaultConfigFile)
}

func SearchDirs() []string {
	return uniqueDirs([]string{
		".",
		DefaultConfigDir,
		LegacyConfigDir,
		"CONFIGS",
		".codex",
	})
}

func uniqueDirs(dirs []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		trimmed := strings.TrimSpace(dir)
		if trimmed == "" {
			continue
		}
		normalized := strings.ToLower(filepath.Clean(trimmed))
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
