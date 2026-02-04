package cli

import (
	"os"
	"path/filepath"
	"testing"

	"go_scrap/internal/app"
)

func TestParseArgs_UsesConfigDefaults(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cfg.json")
	if err := os.WriteFile(cfgPath, []byte(`{
  "url": "https://example.com",
  "mode": "static",
  "output_dir": "artifacts/example",
  "timeout_seconds": 9,
  "user_agent": "ua",
  "wait_for": "body",
  "headless": false,
  "nav_selector": ".nav",
  "content_selector": ".content",
  "exclude_selector": ".ads",
  "nav_walk": true,
  "rate_limit_per_second": 1.5,
  "max_markdown_bytes": 4096,
  "max_chars": 12000,
  "max_tokens": 3000,
  "pipeline_hooks": ["strict-report", "exec"],
  "post_commands": ["echo hello"]
}`), 0600); err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	opts, initCfg, err := ParseArgs([]string{"--config", cfgPath, "--yes"})
	if err != nil {
		t.Fatalf("ParseArgs error: %v", err)
	}
	assertConfigDefaults(t, opts, initCfg)
}

func assertConfigDefaults(t *testing.T, opts app.Options, initCfg bool) {
	t.Helper()
	assertCoreDefaults(t, opts, initCfg)
	assertLimitDefaults(t, opts)
	assertPipelineDefaults(t, opts)
}

func assertCoreDefaults(t *testing.T, opts app.Options, initCfg bool) {
	t.Helper()
	if initCfg {
		t.Fatalf("expected initConfig=false")
	}
	if opts.URL != "https://example.com" || opts.Mode != "static" || opts.OutputDir != "artifacts/example" {
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

func assertLimitDefaults(t *testing.T, opts app.Options) {
	t.Helper()
	if opts.MaxMarkdownBytes != 4096 {
		t.Fatalf("max markdown bytes not applied: %+v", opts)
	}
	if opts.MaxChars != 12000 || opts.MaxTokens != 3000 {
		t.Fatalf("max chars/tokens not applied: %+v", opts)
	}
}

func assertPipelineDefaults(t *testing.T, opts app.Options) {
	t.Helper()
	if len(opts.PipelineHooks) != 2 || opts.PipelineHooks[0] != "strict-report" || opts.PipelineHooks[1] != "exec" {
		t.Fatalf("pipeline hooks not applied: %+v", opts.PipelineHooks)
	}
	if len(opts.PostCommands) != 1 || opts.PostCommands[0] != "echo hello" {
		t.Fatalf("post commands not applied: %+v", opts.PostCommands)
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
