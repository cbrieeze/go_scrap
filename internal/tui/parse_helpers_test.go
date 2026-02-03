package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParsePositiveInt(t *testing.T) {
	v, err := parsePositiveInt("5", "err")
	if err != nil || v != 5 {
		t.Fatalf("parsePositiveInt unexpected: %v %v", v, err)
	}
	if _, err := parsePositiveInt("0", "err"); err == nil {
		t.Fatalf("expected error for non-positive value")
	}
}

func TestParseNonNegativeInt(t *testing.T) {
	v, err := parseNonNegativeInt("0", "err")
	if err != nil || v != 0 {
		t.Fatalf("parseNonNegativeInt unexpected: %v %v", v, err)
	}
	if _, err := parseNonNegativeInt("-1", "err"); err == nil {
		t.Fatalf("expected error for negative value")
	}
}

func TestParseNonNegativeFloat(t *testing.T) {
	v, err := parseNonNegativeFloat("2.5", "err")
	if err != nil || v != 2.5 {
		t.Fatalf("parseNonNegativeFloat unexpected: %v %v", v, err)
	}
	if _, err := parseNonNegativeFloat("-0.1", "err"); err == nil {
		t.Fatalf("expected error for negative value")
	}
}

func TestWriteConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.json")
	res, err := buildResult(&formState{
		urlStr:          "https://example.com",
		mode:            "auto",
		timeoutSecStr:   "10",
		rateLimitStr:    "0",
		userAgent:       "ua",
		waitFor:         "body",
		headless:        true,
		runNow:          true,
		yes:             true,
		configPath:      path,
		saveConfig:      true,
		maxSectionsStr:  "0",
		maxMenuItemsStr: "0",
	})
	if err != nil {
		t.Fatalf("buildResult error: %v", err)
	}

	// buildResult writes config when saveConfig is true
	if res.ConfigPath != path {
		t.Fatalf("unexpected config path: %s", res.ConfigPath)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected config file to be written: %v", err)
	}
}
