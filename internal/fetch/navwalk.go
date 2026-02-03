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
	if err := normalizeAnchorOptions(&opts); err != nil {
		return nil, err
	}

	baseURL, err := normalizeAnchorBase(opts.URL)
	if err != nil {
		return nil, err
	}

	if err := waitForRateLimit(ctx, opts.RateLimitPerSecond); err != nil {
		return nil, err
	}

	page, closeAll, err := openPage(opts)
	if err != nil {
		return nil, err
	}
	defer closeAll()

	if err := gotoAndWait(page, baseURL, opts); err != nil {
		return nil, err
	}

	return fetchAnchorContent(page, baseURL, opts, anchors)
}

func normalizeAnchorBase(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	u.Fragment = ""
	return u.String(), nil
}

func normalizeAnchorOptions(opts *Options) error {
	if opts.URL == "" {
		return errors.New("url is required")
	}
	if opts.Timeout == 0 {
		opts.Timeout = 45 * time.Second
	}
	if opts.UserAgent == "" {
		opts.UserAgent = "go_scrap/1.0"
	}
	return nil
}

func openPage(opts Options) (playwright.Page, func(), error) {
	if err := playwright.Install(&playwright.RunOptions{}); err != nil {
		return nil, func() {}, fmt.Errorf("install playwright: %w", err)
	}
	pw, err := playwright.Run()
	if err != nil {
		return nil, func() {}, err
	}

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(opts.Headless),
	})
	if err != nil {
		_ = pw.Stop()
		return nil, func() {}, err
	}

	page, err := browser.NewPage(playwright.BrowserNewPageOptions{
		UserAgent: playwright.String(opts.UserAgent),
	})
	if err != nil {
		_ = browser.Close()
		_ = pw.Stop()
		return nil, func() {}, err
	}

	closeAll := func() {
		_ = page.Close()
		_ = browser.Close()
		_ = pw.Stop()
	}
	return page, closeAll, nil
}

func gotoAndWait(page playwright.Page, url string, opts Options) error {
	if _, err := page.Goto(url, playwright.PageGotoOptions{
		Timeout:   playwright.Float(float64(opts.Timeout.Milliseconds())),
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	}); err != nil {
		return err
	}
	if opts.WaitForSelector == "" {
		return nil
	}
	loc := page.Locator(opts.WaitForSelector)
	if err := loc.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(float64(opts.Timeout.Milliseconds())),
	}); err != nil {
		return fmt.Errorf("wait-for selector timed out: %s", opts.WaitForSelector)
	}
	return nil
}

func fetchAnchorContent(page playwright.Page, baseURL string, opts Options, anchors []string) (map[string]string, error) {
	results := make(map[string]string, len(anchors))
	for _, anchor := range anchors {
		if strings.TrimSpace(anchor) == "" {
			continue
		}
		targetURL := baseURL + "#" + anchor
		if err := gotoAndWait(page, targetURL, opts); err != nil {
			return nil, err
		}
		html, err := page.Content()
		if err != nil {
			return nil, err
		}
		results[anchor] = html
	}
	return results, nil
}
