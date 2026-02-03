package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go_scrap/internal/config"
)

func TestRunConfigWizard_WritesSchema(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "cfg.json")

	in := strings.Join([]string{
		cfgPath,               // Config file path
		"https://example.com", // URL
		"auto",                // Mode
		filepath.Join(dir, "out"),
		"60",    // Timeout seconds
		"body",  // Wait for selector
		"true",  // Headless
		".nav",  // Nav selector
		".main", // Content selector
		"",      // final newline
	}, "\n")

	withStdin(t, in, func() {
		if err := RunConfigWizard(); err != nil {
			t.Fatalf("RunConfigWizard error: %v", err)
		}
	})

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var cfg config.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}
	if cfg.URL != "https://example.com" {
		t.Fatalf("expected url to be set, got %q", cfg.URL)
	}
	if cfg.OutputDir == "" {
		t.Fatal("expected output_dir to be set")
	}
	if cfg.Headless == nil || *cfg.Headless != true {
		t.Fatalf("expected headless true, got %#v", cfg.Headless)
	}
}

func withStdin(t *testing.T, input string, fn func()) {
	t.Helper()

	orig := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	_, _ = w.WriteString(input)
	_ = w.Close()

	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = orig
		_ = r.Close()
	})

	fn()
}
