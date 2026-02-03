package fetch

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
)

type fakeNavPage struct {
	locators   map[string]*fakeNavLocator
	gotoURL    string
	gotoLog    []string
	evals      []string
	content    string
	gotoErr    error
	contentErr error
}

func (f *fakeNavPage) Locator(sel string) navLocator {
	if loc, ok := f.locators[sel]; ok {
		return loc
	}
	return &fakeNavLocator{}
}

func (f *fakeNavPage) Goto(url string, _ playwright.PageGotoOptions) (playwright.Response, error) {
	f.gotoURL = url
	f.gotoLog = append(f.gotoLog, url)
	return nil, f.gotoErr
}

func (f *fakeNavPage) Evaluate(_ string, _ interface{}) (interface{}, error) {
	if len(f.evals) == 0 {
		return "", nil
	}
	val := f.evals[0]
	f.evals = f.evals[1:]
	return val, nil
}

func (f *fakeNavPage) Content() (string, error) {
	if f.contentErr != nil {
		return "", f.contentErr
	}
	return f.content, nil
}

type fakeNavLocator struct {
	count     int
	clickErr  error
	waitErr   error
	clicked   bool
	waited    bool
	scrollErr error
}

func (f *fakeNavLocator) Count() (int, error) {
	return f.count, nil
}

func (f *fakeNavLocator) First() navLocator {
	return f
}

func (f *fakeNavLocator) ScrollIntoViewIfNeeded() error {
	return f.scrollErr
}

func (f *fakeNavLocator) Click(playwright.LocatorClickOptions) error {
	f.clicked = true
	return f.clickErr
}

func (f *fakeNavLocator) WaitFor(playwright.LocatorWaitForOptions) error {
	f.waited = true
	return f.waitErr
}

func TestNavigateToAnchor_ClicksLink(t *testing.T) {
	page := &fakeNavPage{
		locators: map[string]*fakeNavLocator{
			`a[href="#anchor"]`: {count: 1},
		},
	}
	opts := Options{Timeout: 1 * time.Second}
	if err := navigateToAnchor(page, "https://example.com", "anchor", opts); err != nil {
		t.Fatalf("navigate failed: %v", err)
	}
	if page.gotoURL != "" {
		t.Fatalf("expected no goto, got %s", page.gotoURL)
	}
	if !page.locators[`a[href="#anchor"]`].clicked {
		t.Fatal("expected click")
	}
}

func TestNavigateToAnchor_FallbackGoto(t *testing.T) {
	page := &fakeNavPage{}
	opts := Options{Timeout: 1 * time.Second}
	if err := navigateToAnchor(page, "https://example.com", "anchor", opts); err != nil {
		t.Fatalf("navigate failed: %v", err)
	}
	if page.gotoURL != "https://example.com#anchor" {
		t.Fatalf("unexpected goto: %s", page.gotoURL)
	}
}

func TestNavigateToAnchor_ClickErrorFallsBack(t *testing.T) {
	page := &fakeNavPage{
		locators: map[string]*fakeNavLocator{
			`a[href="#anchor"]`: {count: 1, clickErr: errors.New("click")},
		},
	}
	opts := Options{Timeout: 1 * time.Second}
	if err := navigateToAnchor(page, "https://example.com", "anchor", opts); err != nil {
		t.Fatalf("navigate failed: %v", err)
	}
	if page.gotoURL != "https://example.com#anchor" {
		t.Fatalf("expected fallback goto, got %s", page.gotoURL)
	}
}

func TestWaitForAnchorContent_EvaluatesUntilText(t *testing.T) {
	loc := &fakeNavLocator{count: 1}
	page := &fakeNavPage{
		locators: map[string]*fakeNavLocator{"#anchor": loc},
		evals:    []string{"", "ready"},
	}
	waitForAnchorContent(page, "anchor", 10*time.Millisecond)
	if !loc.waited {
		t.Fatal("expected WaitFor to run")
	}
}

func TestWaitForAnchorContent_EmptyAnchor(t *testing.T) {
	loc := &fakeNavLocator{count: 1}
	page := &fakeNavPage{
		locators: map[string]*fakeNavLocator{"#anchor": loc},
		evals:    []string{"ready"},
	}
	waitForAnchorContent(page, "", 10*time.Millisecond)
	if loc.waited {
		t.Fatal("expected no wait for empty anchor")
	}
}

func TestNormalizeAnchorBase(t *testing.T) {
	base, err := normalizeAnchorBase("https://example.com/docs#section")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if base != "https://example.com/docs" {
		t.Fatalf("unexpected base: %s", base)
	}
}

func TestNormalizeAnchorBase_InvalidURL(t *testing.T) {
	if _, err := normalizeAnchorBase("://bad"); err == nil {
		t.Fatal("expected error")
	}
}

func TestNormalizeAnchorOptions(t *testing.T) {
	opts := Options{URL: "https://example.com"}
	if err := normalizeAnchorOptions(&opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts.Timeout == 0 {
		t.Fatal("expected default timeout")
	}
	if opts.UserAgent == "" {
		t.Fatal("expected default user agent")
	}
}

func TestNormalizeAnchorOptions_MissingURL(t *testing.T) {
	opts := Options{}
	if err := normalizeAnchorOptions(&opts); err == nil {
		t.Fatal("expected error")
	}
}

func TestGotoAndWait_NoSelector(t *testing.T) {
	page := &fakeNavPage{}
	opts := Options{Timeout: time.Second}
	if err := gotoAndWait(page, "https://example.com", opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.gotoURL != "https://example.com" {
		t.Fatalf("unexpected goto: %s", page.gotoURL)
	}
}

func TestGotoAndWait_GotoError(t *testing.T) {
	page := &fakeNavPage{gotoErr: errors.New("goto")}
	opts := Options{Timeout: time.Second}
	err := gotoAndWait(page, "https://example.com", opts)
	if err == nil || err.Error() != "goto" {
		t.Fatalf("expected goto error, got %v", err)
	}
}

func TestGotoAndWait_WaitError(t *testing.T) {
	loc := &fakeNavLocator{waitErr: errors.New("wait")}
	page := &fakeNavPage{
		locators: map[string]*fakeNavLocator{".content": loc},
	}
	opts := Options{Timeout: time.Second, WaitForSelector: ".content"}
	err := gotoAndWait(page, "https://example.com", opts)
	if err == nil || !strings.Contains(err.Error(), "wait-for selector timed out") {
		t.Fatalf("expected wait-for error, got %v", err)
	}
}

func TestFetchAnchorContentWithPage(t *testing.T) {
	page := &fakeNavPage{
		locators: map[string]*fakeNavLocator{
			`a[href="#a1"]`: {count: 1},
			`a[href="#a2"]`: {count: 1},
			`#a1`:           {count: 1},
			`#a2`:           {count: 1},
		},
		evals:   []string{"ready", "ready"},
		content: "<html>ok</html>",
	}
	opts := Options{Timeout: 10 * time.Millisecond}
	results, err := fetchAnchorContentWithPage(page, "https://example.com", opts, []string{"a1", " ", "a2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results["a1"] == "" || results["a2"] == "" {
		t.Fatal("expected html for anchors")
	}
}

func TestFetchAnchorContentWithPage_ContentError(t *testing.T) {
	page := &fakeNavPage{
		locators: map[string]*fakeNavLocator{
			`a[href="#a1"]`: {count: 1},
			`#a1`:           {count: 1},
		},
		evals:      []string{"ready"},
		contentErr: errors.New("content"),
	}
	opts := Options{Timeout: 10 * time.Millisecond}
	_, err := fetchAnchorContentWithPage(page, "https://example.com", opts, []string{"a1"})
	if err == nil || err.Error() != "content" {
		t.Fatalf("expected content error, got %v", err)
	}
}

func TestFetchAnchorContentWithPage_NavigateError(t *testing.T) {
	page := &fakeNavPage{gotoErr: errors.New("goto")}
	opts := Options{Timeout: 10 * time.Millisecond}
	_, err := fetchAnchorContentWithPage(page, "https://example.com", opts, []string{"a1"})
	if err == nil || err.Error() != "goto" {
		t.Fatalf("expected navigate error, got %v", err)
	}
}

func TestEscapeCSSAttr(t *testing.T) {
	got := escapeCSSAttr(`a"b`)
	if got != `a\"b` {
		t.Fatalf("unexpected escaped value: %s", got)
	}
}

func TestAnchorHTML_UsesBaseURL(t *testing.T) {
	page := &fakeNavPage{
		locators: map[string]*fakeNavLocator{
			`a[href="#a1"]`: {count: 0},
			`#a1`:           {count: 1},
		},
		evals:   []string{"ready"},
		content: "<html>ok</html>",
	}

	prev := openPageFn
	openPageFn = func(Options) (navPage, func(), error) {
		return page, func() {}, nil
	}
	defer func() { openPageFn = prev }()

	opts := Options{URL: "https://example.com/docs#fragment", Timeout: 10 * time.Millisecond}
	results, err := AnchorHTML(context.Background(), opts, []string{"a1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if page.gotoLog[0] != "https://example.com/docs" {
		t.Fatalf("unexpected base goto: %s", page.gotoLog[0])
	}
}
