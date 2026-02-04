package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	URL                string            `json:"url"`
	Mode               string            `json:"mode"`
	OutputDir          string            `json:"output_dir"`
	TimeoutSeconds     int               `json:"timeout_seconds"`
	UserAgent          string            `json:"user_agent"`
	WaitForSelector    string            `json:"wait_for"`
	Headless           *bool             `json:"headless"`
	NavSelector        string            `json:"nav_selector"`
	ContentSelector    string            `json:"content_selector"`
	ExcludeSelector    string            `json:"exclude_selector"`
	NavWalk            bool              `json:"nav_walk"`
	RateLimitPerSecond float64           `json:"rate_limit_per_second"`
	MaxMarkdownBytes   int               `json:"max_markdown_bytes"`
	MaxChars           int               `json:"max_chars"`
	MaxTokens          int               `json:"max_tokens"`
	ProxyURL           string            `json:"proxy_url"`
	AuthHeaders        map[string]string `json:"auth_headers"`
	AuthCookies        map[string]string `json:"auth_cookies"`
	// Post-processing pipeline hooks
	PipelineHooks []string `json:"pipeline_hooks"`
	PostCommands  []string `json:"post_commands"`
	// Crawl mode settings
	Crawl       bool   `json:"crawl"`
	Resume      bool   `json:"resume"`
	SitemapURL  string `json:"sitemap_url"`
	MaxPages    int    `json:"max_pages"`
	CrawlDepth  int    `json:"crawl_depth"`
	CrawlFilter string `json:"crawl_filter"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Marshal(cfg Config) ([]byte, error) {
	return json.MarshalIndent(cfg, "", "  ")
}
