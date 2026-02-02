package fetch

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

func FetchAnchorHTML(ctx context.Context, opts Options, anchors []string) (map[string]string, error) {
	if opts.URL == "" {
		return nil, errors.New("url is required")
	}
	if opts.Timeout == 0 {
		opts.Timeout = 45 * time.Second
	}
	if opts.UserAgent == "" {
		opts.UserAgent = "go_scrap/1.0"
	}

	baseURL, err := normalizeAnchorBase(opts.URL)
	if err != nil {
		return nil, err
	}

	if err := waitForRateLimit(ctx, opts.RateLimitPerSecond); err != nil {
		return nil, err
	}

	if err := playwright.Install(&playwright.RunOptions{}); err != nil {
		return nil, fmt.Errorf("install playwright: %w", err)
	}
	pw, err := playwright.Run()
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = pw.Stop()
	}()

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(opts.Headless),
	})
	if err != nil {
		return nil, err
	}
	defer browser.Close()

	page, err := browser.NewPage(playwright.BrowserNewPageOptions{
		UserAgent: playwright.String(opts.UserAgent),
	})
	if err != nil {
		return nil, err
	}
	defer page.Close()

	if _, err := page.Goto(baseURL, playwright.PageGotoOptions{
		Timeout:   playwright.Float(float64(opts.Timeout.Milliseconds())),
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	}); err != nil {
		return nil, err
	}
	if opts.WaitForSelector != "" {
		loc := page.Locator(opts.WaitForSelector)
		if err := loc.WaitFor(playwright.LocatorWaitForOptions{
			Timeout: playwright.Float(float64(opts.Timeout.Milliseconds())),
		}); err != nil {
			return nil, fmt.Errorf("wait-for selector timed out: %s", opts.WaitForSelector)
		}
	}

	results := make(map[string]string, len(anchors))
	for _, anchor := range anchors {
		if strings.TrimSpace(anchor) == "" {
			continue
		}
		targetURL := baseURL + "#" + anchor
		if _, err := page.Goto(targetURL, playwright.PageGotoOptions{
			Timeout:   playwright.Float(float64(opts.Timeout.Milliseconds())),
			WaitUntil: playwright.WaitUntilStateNetworkidle,
		}); err != nil {
			return nil, err
		}
		if opts.WaitForSelector != "" {
			loc := page.Locator(opts.WaitForSelector)
			if err := loc.WaitFor(playwright.LocatorWaitForOptions{
				Timeout: playwright.Float(float64(opts.Timeout.Milliseconds())),
			}); err != nil {
				return nil, fmt.Errorf("wait-for selector timed out: %s", opts.WaitForSelector)
			}
		}
		html, err := page.Content()
		if err != nil {
			return nil, err
		}
		results[anchor] = html
	}

	return results, nil
}

func normalizeAnchorBase(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	u.Fragment = ""
	return u.String(), nil
}
