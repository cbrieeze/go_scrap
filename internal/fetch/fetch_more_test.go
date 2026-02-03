package fetch

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestLooksDynamic(t *testing.T) {
	// Very short HTML is treated as dynamic/placeholder-like.
	if !looksDynamic("<html></html>") {
		t.Fatal("expected short html to look dynamic")
	}

	// Long-ish HTML with headings should not look dynamic.
	longWithHeading := "<html><body>" + strings.Repeat("x", 2100) + "<h1>Title</h1></body></html>"
	if looksDynamic(longWithHeading) {
		t.Fatal("expected html with headings to not look dynamic")
	}

	// Long HTML with a React root and no headings should look dynamic.
	longReact := "<html><body><div id=\"root\"></div>" + strings.Repeat("x", 2100) + "</body></html>"
	if !looksDynamic(longReact) {
		t.Fatal("expected react root without headings to look dynamic")
	}
}

func TestWaitForRateLimit(t *testing.T) {
	t.Run("Disabled", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := waitForRateLimit(ctx, 0); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("CanceledContext", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if err := waitForRateLimit(ctx, 10); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("HighRate", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := waitForRateLimit(ctx, 1e12); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
