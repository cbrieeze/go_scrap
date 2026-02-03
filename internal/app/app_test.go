package app_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

func TestRun_WithContentSelector(t *testing.T) {
	html := `<html><body>
		<nav><a href="#sec1">Link</a></nav>
		<main class="content">
			<h1 id="sec1">Section 1</h1>
			<p>Content paragraph</p>
			<h2 id="sec2">Section 2</h2>
			<p>More content</p>
		</main>
		<footer>Footer</footer>
	</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := app.Options{
		URL:             srv.URL,
		Mode:            fetch.ModeStatic,
		Timeout:         5 * time.Second,
		Yes:             true,
		DryRun:          true,
		Headless:        true,
		UserAgent:       "test",
		ContentSelector: ".content",
	}

	if err := app.Run(ctx, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_WithNavSelector(t *testing.T) {
	html := `<html><body>
		<nav class="menu">
			<ul>
				<li><a href="#intro">Introduction</a></li>
				<li><a href="#guide">Guide</a></li>
			</ul>
		</nav>
		<main>
			<h1 id="intro">Introduction</h1>
			<p>Intro content</p>
			<h2 id="guide">Guide</h2>
			<p>Guide content</p>
		</main>
	</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	tmpDir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := app.Options{
		URL:         srv.URL,
		Mode:        fetch.ModeStatic,
		Timeout:     5 * time.Second,
		Yes:         true,
		Headless:    true,
		UserAgent:   "test",
		NavSelector: ".menu",
		OutputDir:   tmpDir,
	}

	if err := app.Run(ctx, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify menu.json was created
	menuPath := filepath.Join(tmpDir, "menu.json")
	if _, err := os.Stat(menuPath); os.IsNotExist(err) {
		t.Errorf("expected menu.json to be created at %s", menuPath)
	}
}

func TestRun_DryRunNoFilesWritten(t *testing.T) {
	html := `<html><body><h1 id="title">Title</h1><p>Body</p></body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(html))
	}))
	defer srv.Close()

	tmpDir := t.TempDir()

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
		OutputDir: tmpDir,
	}

	if err := app.Run(ctx, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify no files were written in dry run mode
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read tmpDir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected no files in dry run, got %d entries", len(entries))
	}
}

func TestRun_EmptyURL(t *testing.T) {
	ctx := context.Background()
	opts := app.Options{
		URL: "",
	}

	err := app.Run(ctx, opts)
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
	if !strings.Contains(err.Error(), "url is required") {
		t.Errorf("expected 'url is required' error, got: %v", err)
	}
}
