package fetch

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
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
}

type Result struct {
	HTML       string
	FinalMode  Mode
	SourceInfo string
}

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
		html, err := fetchStatic(ctx, opts)
		if err != nil {
			return Result{}, err
		}
		return Result{HTML: html, FinalMode: ModeStatic, SourceInfo: "static"}, nil
	case ModeDynamic:
		html, err := fetchDynamic(ctx, opts)
		if err != nil {
			return Result{}, err
		}
		return Result{HTML: html, FinalMode: ModeDynamic, SourceInfo: "dynamic"}, nil
	case ModeAuto:
		html, err := fetchStatic(ctx, opts)
		if err == nil && !looksDynamic(html) {
			return Result{HTML: html, FinalMode: ModeStatic, SourceInfo: "auto:static"}, nil
		}
		html, derr := fetchDynamic(ctx, opts)
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

	client := &http.Client{Timeout: opts.Timeout}
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

func fetchDynamic(ctx context.Context, opts Options) (string, error) {
	if err := waitForRateLimit(ctx, opts.RateLimitPerSecond); err != nil {
		return "", err
	}

	if err := playwright.Install(&playwright.RunOptions{}); err != nil {
		return "", fmt.Errorf("install playwright: %w", err)
	}
	pw, err := playwright.Run()
	if err != nil {
		return "", err
	}
	defer func() {
		_ = pw.Stop()
	}()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(opts.Headless),
	})
	if err != nil {
		return "", err
	}
	defer browser.Close()

	page, err := browser.NewPage(playwright.BrowserNewPageOptions{
		UserAgent: playwright.String(opts.UserAgent),
	})
	if err != nil {
		return "", err
	}
	defer page.Close()

	_, err = page.Goto(opts.URL, playwright.PageGotoOptions{
		Timeout:   playwright.Float(float64(opts.Timeout.Milliseconds())),
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	})
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("dynamic fetch timed out after %s (try --timeout or --wait-for)", opts.Timeout)
		}
		return "", err
	}
	if opts.WaitForSelector != "" {
		loc := page.Locator(opts.WaitForSelector)
		if err := loc.WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(float64(opts.Timeout.Milliseconds())),
		}); err != nil {
			return "", fmt.Errorf("wait-for selector timed out: %s", opts.WaitForSelector)
		}
	}

	html, err := page.Content()
	if err != nil {
		return "", err
	}
	return html, nil
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
