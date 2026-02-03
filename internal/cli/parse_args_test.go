package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseArgs_UsesConfigDefaults(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cfg.json")
	if err := os.WriteFile(cfgPath, []byte(`{
  "url": "https://example.com",
  "mode": "static",
  "output_dir": "output/example",
  "timeout_seconds": 9,
  "user_agent": "ua",
  "wait_for": "body",
  "headless": false,
  "nav_selector": ".nav",
  "content_selector": ".content",
  "exclude_selector": ".ads",
  "nav_walk": true,
  "rate_limit_per_second": 1.5
}`), 0600); err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	opts, initCfg, err := ParseArgs([]string{"--config", cfgPath, "--yes"})
	if err != nil {
		t.Fatalf("ParseArgs error: %v", err)
	}
	if initCfg {
		t.Fatalf("expected initConfig=false")
	}
	if opts.URL != "https://example.com" || opts.Mode != "static" || opts.OutputDir != "output/example" {
		t.Fatalf("config merge failed: %+v", opts)
	}
	if opts.Timeout.Seconds() != 9 {
		t.Fatalf("timeout not applied: %v", opts.Timeout)
	}
	if opts.Headless {
		t.Fatalf("headless should be false from config")
	}
	if !opts.NavWalk || opts.RateLimitPerSecond != 1.5 {
		t.Fatalf("nav/rate not applied: %+v", opts)
	}
}

func TestParseArgs_InitConfigShortCircuit(t *testing.T) {
	opts, initCfg, err := ParseArgs([]string{"--init-config"})
	if err != nil {
		t.Fatalf("ParseArgs error: %v", err)
	}
	if !initCfg {
		t.Fatalf("expected initConfig=true")
	}
	if opts.URL != "" || opts.OutputDir != "" {
		t.Fatalf("expected zero opts when init-config set")
	}
}

func TestParseArgs_ErrorOnMissingURL(t *testing.T) {
	_, _, err := ParseArgs([]string{"--mode", "static"})
	if err == nil {
		t.Fatalf("expected error for missing url")
	}
	if exitErr, ok := err.(ExitError); !ok || exitErr.Code != 2 {
		t.Fatalf("expected ExitError code 2, got %#v", err)
	}
}
