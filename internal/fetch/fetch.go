package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Mode string

const (
	ModeAuto    Mode = "auto"
	ModeStatic  Mode = "static"
	ModeDynamic Mode = "dynamic"
)

type Options struct {
	URL                string
	Mode               Mode
	Timeout            time.Duration
	UserAgent          string
	WaitForSelector    string
	Headless           bool
	RateLimitPerSecond float64
	ProxyURL           string
	Headers            map[string]string
	Cookies            map[string]string
}

type Result struct {
	HTML       string
	FinalMode  Mode
	SourceInfo string
}

var staticFetch = fetchStatic
var dynamicFetch = fetchDynamic

func Fetch(ctx context.Context, opts Options) (Result, error) {
	if opts.URL == "" {
		return Result{}, errors.New("url is required")
	}
	if opts.Timeout == 0 {
		opts.Timeout = 45 * time.Second
	}
	if opts.UserAgent == "" {
		opts.UserAgent = "go_scrap/1.0"
	}

	switch opts.Mode {
	case ModeStatic:
		html, err := staticFetch(ctx, opts)
		if err != nil {
			return Result{}, err
		}
		return Result{HTML: html, FinalMode: ModeStatic, SourceInfo: "static"}, nil
	case ModeDynamic:
		html, err := dynamicFetch(ctx, opts)
		if err != nil {
			return Result{}, err
		}
		return Result{HTML: html, FinalMode: ModeDynamic, SourceInfo: "dynamic"}, nil
	case ModeAuto:
		html, err := staticFetch(ctx, opts)
		if err == nil && !looksDynamic(html) {
			return Result{HTML: html, FinalMode: ModeStatic, SourceInfo: "auto:static"}, nil
		}
		html, derr := dynamicFetch(ctx, opts)
		if derr != nil {
			if err != nil {
				return Result{}, fmt.Errorf("static failed: %v; dynamic failed: %w", err, derr)
			}
			return Result{}, derr
		}
		return Result{HTML: html, FinalMode: ModeDynamic, SourceInfo: "auto:dynamic"}, nil
	default:
		return Result{}, fmt.Errorf("unknown mode: %s", opts.Mode)
	}
}

func fetchStatic(ctx context.Context, opts Options) (string, error) {
	if err := waitForRateLimit(ctx, opts.RateLimitPerSecond); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, opts.URL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("User-Agent", opts.UserAgent)
	applyHeaders(req.Header, opts.Headers, opts.Cookies)

	client := &http.Client{Timeout: opts.Timeout}
	if opts.ProxyURL != "" {
		proxyURL, err := url.Parse(opts.ProxyURL)
		if err != nil {
			return "", fmt.Errorf("invalid proxy URL: %w", err)
		}
		client.Transport = &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("static fetch timed out after %s", opts.Timeout)
		}
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("http status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func applyHeaders(headers http.Header, extra map[string]string, cookies map[string]string) {
	for key, value := range extra {
		headers.Set(key, value)
	}
	cookieHeader := buildCookieHeader(cookies)
	if cookieHeader == "" {
		return
	}
	if existing := headers.Get("Cookie"); existing != "" {
		headers.Set("Cookie", existing+"; "+cookieHeader)
		return
	}
	headers.Set("Cookie", cookieHeader)
}

func buildCookieHeader(cookies map[string]string) string {
	if len(cookies) == 0 {
		return ""
	}
	keys := make([]string, 0, len(cookies))
	for key := range cookies {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, cookies[key]))
	}
	return strings.Join(parts, "; ")
}

func waitForRateLimit(ctx context.Context, ratePerSecond float64) error {
	if ratePerSecond <= 0 {
		return nil
	}
	interval := time.Duration(float64(time.Second) / ratePerSecond)
	if interval <= 0 {
		return nil
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func looksDynamic(html string) bool {
	trimmed := strings.TrimSpace(html)
	if len(trimmed) < 2000 {
		return true
	}
	lower := strings.ToLower(trimmed)
	if !strings.Contains(lower, "<h1") && !strings.Contains(lower, "<h2") && !strings.Contains(lower, "<h3") {
		if strings.Contains(lower, "id=\"root\"") || strings.Contains(lower, "id=\"app\"") || strings.Contains(lower, "data-reactroot") {
			return true
		}
	}
	return false
}
