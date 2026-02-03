package config_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"go_scrap/internal/config"
)

func TestLoadConfig(t *testing.T) {
	data := []byte(`{
  "url": "https://example.com",
  "mode": "dynamic",
  "output_dir": "output/test",
  "timeout_seconds": 42,
  "user_agent": "test-agent",
  "wait_for": "main",
  "headless": true,
  "nav_selector": ".nav",
  "content_selector": "main",
  "exclude_selector": ".ads",
  "nav_walk": true,
  "rate_limit_per_second": 2.5,
  "max_markdown_bytes": 2048,
  "max_chars": 16000,
  "max_tokens": 4000
}`)

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}

	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	headless := true
	expected := config.Config{
		URL:                "https://example.com",
		Mode:               "dynamic",
		OutputDir:          "output/test",
		TimeoutSeconds:     42,
		UserAgent:          "test-agent",
		WaitForSelector:    "main",
		Headless:           &headless,
		NavSelector:        ".nav",
		ContentSelector:    "main",
		ExcludeSelector:    ".ads",
		NavWalk:            true,
		RateLimitPerSecond: 2.5,
		MaxMarkdownBytes:   2048,
		MaxChars:           16000,
		MaxTokens:          4000,
	}

	if !reflect.DeepEqual(cfg, expected) {
		t.Fatalf("config mismatch\nexpected: %#v\ngot:      %#v", expected, cfg)
	}
}

func TestMarshalConfig(t *testing.T) {
	headless := true
	cfg := config.Config{
		URL:                "https://example.com",
		Mode:               "auto",
		OutputDir:          "output/x",
		TimeoutSeconds:     10,
		UserAgent:          "ua",
		WaitForSelector:    "body",
		Headless:           &headless,
		NavSelector:        "nav",
		ContentSelector:    "main",
		ExcludeSelector:    ".ads",
		NavWalk:            true,
		RateLimitPerSecond: 1.2,
		MaxMarkdownBytes:   1024,
		MaxChars:           8000,
		MaxTokens:          2000,
	}

	data, err := config.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("marshal produced empty output")
	}
}
