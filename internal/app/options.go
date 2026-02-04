package app

import (
	"errors"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go_scrap/internal/fetch"
)

func normalizeOptions(opts Options) (Options, error) {
	if strings.TrimSpace(opts.URL) == "" && !opts.Crawl {
		return opts, errors.New("url is required")
	}
	if opts.Crawl && strings.TrimSpace(opts.URL) == "" && strings.TrimSpace(opts.SitemapURL) == "" {
		return opts, errors.New("url or sitemap is required for crawl mode")
	}
	if opts.Mode == "" {
		opts.Mode = fetch.ModeAuto
	}
	if opts.Timeout == 0 {
		opts.Timeout = time.Duration(DefaultTimeoutSeconds) * time.Second
	}
	if opts.UserAgent == "" {
		opts.UserAgent = DefaultUserAgent
	}
	if opts.OutputDir == "" {
		urlForHost := opts.URL
		if urlForHost == "" {
			urlForHost = opts.SitemapURL
		}
		host := hostFromURL(urlForHost)
		if host == "" {
			host = "default"
		}
		opts.OutputDir = filepath.Join(DefaultOutputRoot, host)
	}
	if opts.Stdout {
		opts.Yes = true
	}
	return opts, nil
}

func hostFromURL(urlStr string) string {
	if !strings.Contains(urlStr, "://") {
		urlStr = "https://" + urlStr
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	host := u.Hostname()
	host = strings.ReplaceAll(host, ".", "_")
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	host = re.ReplaceAllString(host, "")
	return host
}
