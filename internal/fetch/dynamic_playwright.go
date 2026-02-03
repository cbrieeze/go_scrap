package fetch

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/playwright-community/playwright-go"
)

type dynamicProvider interface {
	Install() error
	Run() (dynamicRunner, error)
}

type dynamicRunner interface {
	ChromiumLaunch(headless bool) (dynamicBrowser, error)
	Stop() error
}

type dynamicBrowser interface {
	NewPage(userAgent string) (dynamicPage, error)
	Close() error
}

type dynamicPage interface {
	Goto(url string, timeout time.Duration) error
	WaitFor(selector string, timeout time.Duration) error
	Content() (string, error)
	Close() error
}

type playwrightProvider struct{}

func (playwrightProvider) Install() error {
	return playwright.Install(&playwright.RunOptions{})
}

func (playwrightProvider) Run() (dynamicRunner, error) {
	pw, err := playwright.Run()
	if err != nil {
		return nil, err
	}
	return &playwrightRunner{pw: pw}, nil
}

type playwrightRunner struct {
	pw *playwright.Playwright
}

func (r *playwrightRunner) ChromiumLaunch(headless bool) (dynamicBrowser, error) {
	browser, err := r.pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(headless),
	})
	if err != nil {
		return nil, err
	}
	return &playwrightBrowser{browser: browser}, nil
}

func (r *playwrightRunner) Stop() error {
	return r.pw.Stop()
}

type playwrightBrowser struct {
	browser playwright.Browser
}

func (b *playwrightBrowser) NewPage(userAgent string) (dynamicPage, error) {
	page, err := b.browser.NewPage(playwright.BrowserNewPageOptions{
		UserAgent: playwright.String(userAgent),
	})
	if err != nil {
		return nil, err
	}
	return &playwrightPage{page: page}, nil
}

func (b *playwrightBrowser) Close() error {
	return b.browser.Close()
}

type playwrightPage struct {
	page playwright.Page
}

func (p *playwrightPage) Goto(url string, timeout time.Duration) error {
	_, err := p.page.Goto(url, playwright.PageGotoOptions{
		Timeout:   playwright.Float(float64(timeout.Milliseconds())),
		WaitUntil: playwright.WaitUntilStateNetworkidle,
	})
	return err
}

func (p *playwrightPage) WaitFor(selector string, timeout time.Duration) error {
	loc := p.page.Locator(selector)
	return loc.WaitFor(playwright.LocatorWaitForOptions{
		Timeout: playwright.Float(float64(timeout.Milliseconds())),
	})
}

func (p *playwrightPage) Content() (string, error) {
	return p.page.Content()
}

func (p *playwrightPage) Close() error {
	return p.page.Close()
}

func fetchDynamic(ctx context.Context, opts Options) (string, error) {
	return fetchDynamicWith(ctx, opts, playwrightProvider{})
}

func fetchDynamicWith(ctx context.Context, opts Options, provider dynamicProvider) (string, error) {
	if err := waitForRateLimit(ctx, opts.RateLimitPerSecond); err != nil {
		return "", err
	}

	if err := provider.Install(); err != nil {
		return "", fmt.Errorf("install playwright: %w", err)
	}
	runner, err := provider.Run()
	if err != nil {
		return "", err
	}
	defer func() {
		_ = runner.Stop()
	}()

	browser, err := runner.ChromiumLaunch(opts.Headless)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = browser.Close()
	}()

	page, err := browser.NewPage(opts.UserAgent)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = page.Close()
	}()

	if err := page.Goto(opts.URL, opts.Timeout); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return "", fmt.Errorf("dynamic fetch timed out after %s (try --timeout or --wait-for)", opts.Timeout)
		}
		return "", err
	}
	if opts.WaitForSelector != "" {
		if err := page.WaitFor(opts.WaitForSelector, opts.Timeout); err != nil {
			return "", fmt.Errorf("wait-for selector timed out: %s", opts.WaitForSelector)
		}
	}

	html, err := page.Content()
	if err != nil {
		return "", err
	}
	return html, nil
}
