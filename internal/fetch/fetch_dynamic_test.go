package fetch

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type fakeProvider struct {
	installErr error
	runErr     error
	runner     *fakeRunner
}

func (p *fakeProvider) Install() error {
	return p.installErr
}

func (p *fakeProvider) Run() (dynamicRunner, error) {
	if p.runErr != nil {
		return nil, p.runErr
	}
	if p.runner == nil {
		p.runner = &fakeRunner{}
	}
	return p.runner, nil
}

type fakeRunner struct {
	launchErr error
	browser   *fakeBrowser
	stopped   bool
}

func (r *fakeRunner) ChromiumLaunch(_ bool) (dynamicBrowser, error) {
	if r.launchErr != nil {
		return nil, r.launchErr
	}
	if r.browser == nil {
		r.browser = &fakeBrowser{}
	}
	return r.browser, nil
}

func (r *fakeRunner) Stop() error {
	r.stopped = true
	return nil
}

type fakeBrowser struct {
	newPageErr error
	page       *fakePage
	closed     bool
	userAgent  string
}

func (b *fakeBrowser) NewPage(userAgent string) (dynamicPage, error) {
	if b.newPageErr != nil {
		return nil, b.newPageErr
	}
	if b.page == nil {
		b.page = &fakePage{}
	}
	b.userAgent = userAgent
	return b.page, nil
}

func (b *fakeBrowser) Close() error {
	b.closed = true
	return nil
}

type fakePage struct {
	gotoErr     error
	waitErr     error
	contentErr  error
	content     string
	closed      bool
	gotoURL     string
	gotoTimeout time.Duration
	waitSel     string
	waitTimeout time.Duration
}

func (p *fakePage) Goto(url string, timeout time.Duration) error {
	p.gotoURL = url
	p.gotoTimeout = timeout
	return p.gotoErr
}

func (p *fakePage) WaitFor(selector string, timeout time.Duration) error {
	p.waitSel = selector
	p.waitTimeout = timeout
	return p.waitErr
}

func (p *fakePage) Content() (string, error) {
	return p.content, p.contentErr
}

func (p *fakePage) Close() error {
	p.closed = true
	return nil
}

func TestFetchDynamicWith_InstallError(t *testing.T) {
	_, err := fetchDynamicWith(context.Background(), Options{}, &fakeProvider{installErr: errors.New("nope")})
	if err == nil || !strings.Contains(err.Error(), "install playwright") {
		t.Fatalf("expected install error, got %v", err)
	}
}

func TestFetchDynamicWith_RunError(t *testing.T) {
	_, err := fetchDynamicWith(context.Background(), Options{}, &fakeProvider{runErr: errors.New("boom")})
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected run error, got %v", err)
	}
}

func TestFetchDynamicWith_LaunchError(t *testing.T) {
	provider := &fakeProvider{runner: &fakeRunner{launchErr: errors.New("launch")}}
	_, err := fetchDynamicWith(context.Background(), Options{}, provider)
	if err == nil || err.Error() != "launch" {
		t.Fatalf("expected launch error, got %v", err)
	}
}

func TestFetchDynamicWith_NewPageError(t *testing.T) {
	provider := &fakeProvider{runner: &fakeRunner{browser: &fakeBrowser{newPageErr: errors.New("page")}}}
	_, err := fetchDynamicWith(context.Background(), Options{}, provider)
	if err == nil || err.Error() != "page" {
		t.Fatalf("expected page error, got %v", err)
	}
}

func TestFetchDynamicWith_GotoTimeout(t *testing.T) {
	page := &fakePage{gotoErr: context.DeadlineExceeded}
	provider := &fakeProvider{runner: &fakeRunner{browser: &fakeBrowser{page: page}}}
	opts := Options{URL: "https://example.com", Timeout: 2 * time.Second}
	_, err := fetchDynamicWith(context.Background(), opts, provider)
	if err == nil || !strings.Contains(err.Error(), "dynamic fetch timed out") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

func TestFetchDynamicWith_GotoError(t *testing.T) {
	page := &fakePage{gotoErr: errors.New("goto")}
	provider := &fakeProvider{runner: &fakeRunner{browser: &fakeBrowser{page: page}}}
	_, err := fetchDynamicWith(context.Background(), Options{}, provider)
	if err == nil || err.Error() != "goto" {
		t.Fatalf("expected goto error, got %v", err)
	}
}

func TestFetchDynamicWith_WaitForError(t *testing.T) {
	page := &fakePage{waitErr: errors.New("wait")}
	provider := &fakeProvider{runner: &fakeRunner{browser: &fakeBrowser{page: page}}}
	opts := Options{URL: "https://example.com", Timeout: time.Second, WaitForSelector: ".content"}
	_, err := fetchDynamicWith(context.Background(), opts, provider)
	if err == nil || !strings.Contains(err.Error(), "wait-for selector timed out") {
		t.Fatalf("expected wait-for error, got %v", err)
	}
}

func TestFetchDynamicWith_ContentError(t *testing.T) {
	page := &fakePage{contentErr: errors.New("content")}
	provider := &fakeProvider{runner: &fakeRunner{browser: &fakeBrowser{page: page}}}
	_, err := fetchDynamicWith(context.Background(), Options{}, provider)
	if err == nil || err.Error() != "content" {
		t.Fatalf("expected content error, got %v", err)
	}
}

func TestFetchDynamicWith_Success(t *testing.T) {
	page := &fakePage{content: "<html>ok</html>"}
	browser := &fakeBrowser{page: page}
	runner := &fakeRunner{browser: browser}
	provider := &fakeProvider{runner: runner}

	opts := Options{URL: "https://example.com", Timeout: time.Second, UserAgent: "ua"}
	html, err := fetchDynamicWith(context.Background(), opts, provider)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if html != "<html>ok</html>" {
		t.Fatalf("unexpected html: %s", html)
	}
	if browser.userAgent != "ua" {
		t.Fatalf("expected user agent to be set, got %q", browser.userAgent)
	}
}

func TestFetchDynamicWith_RateLimitCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := fetchDynamicWith(ctx, Options{RateLimitPerSecond: 1}, &fakeProvider{})
	if err == nil {
		t.Fatal("expected rate limit error")
	}
}
