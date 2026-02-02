package config_test

import (
	"os"
	"path/filepath"
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
  "rate_limit_per_second": 2.5
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

	if cfg.URL != "https://example.com" {
		t.Fatalf("url mismatch: %s", cfg.URL)
	}
	if cfg.Mode != "dynamic" {
		t.Fatalf("mode mismatch: %s", cfg.Mode)
	}
	if cfg.OutputDir != "output/test" {
		t.Fatalf("output_dir mismatch: %s", cfg.OutputDir)
	}
	if cfg.TimeoutSeconds != 42 {
		t.Fatalf("timeout mismatch: %d", cfg.TimeoutSeconds)
	}
	if cfg.UserAgent != "test-agent" {
		t.Fatalf("user_agent mismatch: %s", cfg.UserAgent)
	}
	if cfg.WaitForSelector != "main" {
		t.Fatalf("wait_for mismatch: %s", cfg.WaitForSelector)
	}
	if cfg.Headless == nil || *cfg.Headless != true {
		t.Fatalf("headless mismatch")
	}
	if cfg.NavSelector != ".nav" {
		t.Fatalf("nav_selector mismatch: %s", cfg.NavSelector)
	}
	if cfg.ContentSelector != "main" {
		t.Fatalf("content_selector mismatch: %s", cfg.ContentSelector)
	}
	if cfg.ExcludeSelector != ".ads" {
		t.Fatalf("exclude_selector mismatch: %s", cfg.ExcludeSelector)
	}
	if !cfg.NavWalk {
		t.Fatalf("nav_walk mismatch")
	}
	if cfg.RateLimitPerSecond != 2.5 {
		t.Fatalf("rate_limit_per_second mismatch: %v", cfg.RateLimitPerSecond)
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
	}

	data, err := config.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("marshal produced empty output")
	}
}
