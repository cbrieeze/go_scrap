package app_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go_scrap/internal/app"
	"go_scrap/internal/fetch"
)

func TestRun_StaticHTML_NoSelectors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><h1 id="h">Title</h1><p>Body</p></body></html>`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := app.Options{
		URL:       srv.URL,
		Mode:      fetch.ModeStatic,
		Timeout:   5 * time.Second,
		Yes:       true,
		DryRun:    true,
		Headless:  true,
		UserAgent: "test",
	}

	if err := app.Run(ctx, opts); err != nil {
		if !strings.Contains(err.Error(), "selector") {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}
