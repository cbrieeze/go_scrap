package fetch

import (
	"testing"
	"time"

	"github.com/playwright-community/playwright-go"
)

type fakeNavPage struct {
	locators map[string]*fakeNavLocator
	gotoURL  string
	evals    []string
	content  string
}

func (f *fakeNavPage) Locator(sel string) navLocator {
	if loc, ok := f.locators[sel]; ok {
		return loc
	}
	return &fakeNavLocator{}
}

func (f *fakeNavPage) Goto(url string, _ playwright.PageGotoOptions) (playwright.Response, error) {
	f.gotoURL = url
	return nil, nil
}

func (f *fakeNavPage) Evaluate(expr string, arg interface{}) (interface{}, error) {
	if len(f.evals) == 0 {
		return "", nil
	}
	val := f.evals[0]
	f.evals = f.evals[1:]
	return val, nil
}

func (f *fakeNavPage) Content() (string, error) {
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
