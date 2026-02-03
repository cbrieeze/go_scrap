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

type navPage interface {
	Locator(string) navLocator
	Goto(string, playwright.PageGotoOptions) (playwright.Response, error)
	Evaluate(string, interface{}) (interface{}, error)
	Content() (string, error)
}

type navLocator interface {
	Count() (int, error)
	First() navLocator
	ScrollIntoViewIfNeeded() error
	Click(playwright.LocatorClickOptions) error
	WaitFor(playwright.LocatorWaitForOptions) error
}

type playwrightPageAdapter struct {
	page playwright.Page
}

func (p *playwrightPageAdapter) Locator(selector string) navLocator {
	return &playwrightLocatorAdapter{locator: p.page.Locator(selector)}
}

func (p *playwrightPageAdapter) Goto(url string, opts playwright.PageGotoOptions) (playwright.Response, error) {
	return p.page.Goto(url, opts)
}

func (p *playwrightPageAdapter) Evaluate(expr string, arg interface{}) (interface{}, error) {
	return p.page.Evaluate(expr, arg)
}

func (p *playwrightPageAdapter) Content() (string, error) {
	return p.page.Content()
}

type playwrightLocatorAdapter struct {
	locator playwright.Locator
}

func (l *playwrightLocatorAdapter) Count() (int, error) {
	return l.locator.Count()
}

func (l *playwrightLocatorAdapter) First() navLocator {
	return &playwrightLocatorAdapter{locator: l.locator.First()}
}

func (l *playwrightLocatorAdapter) ScrollIntoViewIfNeeded() error {
	return l.locator.ScrollIntoViewIfNeeded()
}

func (l *playwrightLocatorAdapter) Click(opts playwright.LocatorClickOptions) error {
	return l.locator.Click(opts)
}

func (l *playwrightLocatorAdapter) WaitFor(opts playwright.LocatorWaitForOptions) error {
	return l.locator.WaitFor(opts)
}

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

	adapter := &playwrightPageAdapter{page: page}
	if err := gotoAndWait(adapter, baseURL, opts); err != nil {
		return nil, err
	}

	return fetchAnchorContentWithPage(adapter, baseURL, opts, anchors)
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

func gotoAndWait(page navPage, url string, opts Options) error {
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

func fetchAnchorContentWithPage(page navPage, baseURL string, opts Options, anchors []string) (map[string]string, error) {
	results := make(map[string]string, len(anchors))
	for _, anchor := range anchors {
		if strings.TrimSpace(anchor) == "" {
			continue
		}
		if err := navigateToAnchor(page, baseURL, anchor, opts); err != nil {
			return nil, err
		}
		waitForAnchorContent(page, anchor, opts.Timeout)
		html, err := page.Content()
		if err != nil {
			return nil, err
		}
		results[anchor] = html
	}
	return results, nil
}

func navigateToAnchor(page navPage, baseURL string, anchor string, opts Options) error {
	if strings.TrimSpace(anchor) == "" {
		return nil
	}
	linkSelector := fmt.Sprintf(`a[href="#%s"]`, escapeCSSAttr(anchor))
	loc := page.Locator(linkSelector)
	if count, err := loc.Count(); err == nil && count > 0 {
		_ = loc.First().ScrollIntoViewIfNeeded()
		if err := loc.First().Click(playwright.LocatorClickOptions{
			Timeout: playwright.Float(float64(opts.Timeout.Milliseconds())),
			Force:   playwright.Bool(true),
		}); err == nil {
			return nil
		}
	}
	targetURL := baseURL + "#" + anchor
	return gotoAndWait(page, targetURL, opts)
}

func waitForAnchorContent(page navPage, anchor string, timeout time.Duration) {
	anchor = strings.TrimSpace(anchor)
	if anchor == "" {
		return
	}
	selector := "#" + anchor
	loc := page.Locator(selector)
	_ = loc.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(float64(timeout.Milliseconds())),
	})
	_ = loc.ScrollIntoViewIfNeeded()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		val, err := page.Evaluate(`(sel) => {
			const el = document.querySelector(sel);
			if (!el) return "";
			return (el.innerText || el.textContent || "").trim();
		}`, selector)
		if err == nil {
			if text, ok := val.(string); ok && strings.TrimSpace(text) != "" {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func escapeCSSAttr(value string) string {
	return strings.ReplaceAll(value, `"`, `\"`)
}
