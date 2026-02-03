package crawler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go_scrap/internal/crawler"
)

func TestNew_ValidOptions(t *testing.T) {
	c, err := crawler.New(crawler.Options{
		BaseURL:   "https://example.com",
		RateLimit: 1.0,
		MaxPages:  10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("expected crawler, got nil")
	}
}

func TestNew_MissingBaseURL(t *testing.T) {
	_, err := crawler.New(crawler.Options{})
	if err == nil {
		t.Fatal("expected error for missing base URL")
	}
}

func TestNew_InvalidBaseURL(t *testing.T) {
	_, err := crawler.New(crawler.Options{
		BaseURL: "://invalid",
	})
	if err == nil {
		t.Fatal("expected error for invalid base URL")
	}
}

func TestCrawl_SinglePage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><h1>Test Page</h1></body></html>`))
	}))
	defer srv.Close()

	c, err := crawler.New(crawler.Options{
		BaseURL:         srv.URL,
		RateLimit:       10.0,
		MaxPages:        1,
		Timeout:         5 * time.Second,
		AllowAllDomains: true,
	})
	if err != nil {
		t.Fatalf("create crawler: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, stats, err := c.Crawl(ctx)
	if err != nil {
		t.Fatalf("crawl failed: %v", err)
	}

	if stats.PagesCrawled != 1 {
		t.Errorf("expected 1 page crawled, got %d", stats.PagesCrawled)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	// Find the result (URL might have trailing slash)
	var result *crawler.Result
	for _, r := range results {
		result = r
		break
	}
	if result == nil {
		t.Fatal("expected result for server URL")
	}
	if result.HTML == "" {
		t.Error("expected HTML content")
	}
	if !strings.Contains(result.HTML, "Test Page") {
		t.Error("expected HTML to contain 'Test Page'")
	}
}

func TestCrawl_FollowsLinks(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><h1>Home</h1><a href="/page2">Page 2</a></body></html>`))
	})
	mux.HandleFunc("/page2", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><h1>Page 2</h1></body></html>`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	c, err := crawler.New(crawler.Options{
		BaseURL:         srv.URL,
		RateLimit:       10.0,
		MaxPages:        10,
		MaxDepth:        2,
		Timeout:         5 * time.Second,
		AllowAllDomains: true,
	})
	if err != nil {
		t.Fatalf("create crawler: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	results, stats, err := c.Crawl(ctx)
	if err != nil {
		t.Fatalf("crawl failed: %v", err)
	}

	if stats.PagesCrawled < 2 {
		t.Errorf("expected at least 2 pages crawled, got %d", stats.PagesCrawled)
	}

	if len(results) < 2 {
		t.Errorf("expected at least 2 results, got %d", len(results))
	}
}

func TestCrawl_RespectsMaxPages(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "text/html")
		// Return links to many pages
		_, _ = w.Write([]byte(`<html><body>
			<a href="/page1">1</a>
			<a href="/page2">2</a>
			<a href="/page3">3</a>
			<a href="/page4">4</a>
			<a href="/page5">5</a>
		</body></html>`))
	}))
	defer srv.Close()

	c, err := crawler.New(crawler.Options{
		BaseURL:         srv.URL,
		RateLimit:       10.0,
		MaxPages:        3,
		MaxDepth:        2,
		Timeout:         5 * time.Second,
		AllowAllDomains: true,
	})
	if err != nil {
		t.Fatalf("create crawler: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, stats, err := c.Crawl(ctx)
	if err != nil {
		t.Fatalf("crawl failed: %v", err)
	}

	if stats.PagesCrawled > 3 {
		t.Errorf("expected at most 3 pages crawled, got %d", stats.PagesCrawled)
	}
}
