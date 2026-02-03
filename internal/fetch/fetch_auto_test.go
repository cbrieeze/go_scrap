package fetch

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func withFetchers(staticFn func(context.Context, Options) (string, error), dynamicFn func(context.Context, Options) (string, error), fn func()) {
	prevStatic := staticFetch
	prevDynamic := dynamicFetch
	staticFetch = staticFn
	dynamicFetch = dynamicFn
	defer func() {
		staticFetch = prevStatic
		dynamicFetch = prevDynamic
	}()
	fn()
}

func TestFetch_AutoUsesDynamic(t *testing.T) {
	longReact := "<html><body><div id=\"root\"></div>" + strings.Repeat("x", 2100) + "</body></html>"
	withFetchers(
		func(_ context.Context, _ Options) (string, error) { return longReact, nil },
		func(_ context.Context, _ Options) (string, error) { return "<html>dynamic</html>", nil },
		func() {
			res, err := Fetch(context.Background(), Options{URL: "https://example.com", Mode: ModeAuto})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if res.FinalMode != ModeDynamic || res.SourceInfo != "auto:dynamic" {
				t.Fatalf("expected auto:dynamic, got %+v", res)
			}
			if res.HTML != "<html>dynamic</html>" {
				t.Fatalf("unexpected html: %s", res.HTML)
			}
		},
	)
}

func TestFetch_AutoBothFail(t *testing.T) {
	withFetchers(
		func(_ context.Context, _ Options) (string, error) { return "", errors.New("static down") },
		func(_ context.Context, _ Options) (string, error) { return "", errors.New("dynamic down") },
		func() {
			_, err := Fetch(context.Background(), Options{URL: "https://example.com", Mode: ModeAuto})
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "static failed") || !strings.Contains(err.Error(), "dynamic failed") {
				t.Fatalf("expected combined error, got %v", err)
			}
		},
	)
}
