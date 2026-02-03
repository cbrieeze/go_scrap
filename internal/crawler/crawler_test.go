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

func TestBuildIndex_Basic(t *testing.T) {
	now := time.Now()
	results := map[string]*crawler.Result{
		"https://example.com/":      {URL: "https://example.com/", HTML: "<html>page1</html>", FetchedAt: now},
		"https://example.com/page2": {URL: "https://example.com/page2", HTML: "<html>page2</html>", FetchedAt: now},
	}
	stats := crawler.Stats{
		StartedAt:    now.Add(-time.Minute),
		CompletedAt:  now,
		PagesCrawled: 2,
		PagesFailed:  0,
	}
	sectionCounts := map[string]int{
		"https://example.com/":      3,
		"https://example.com/page2": 5,
	}

	index := crawler.BuildIndex(results, stats, "https://example.com", sectionCounts)

	if index.PagesCrawled != 2 {
		t.Errorf("expected 2 pages crawled, got %d", index.PagesCrawled)
	}
	if index.TotalSections != 8 {
		t.Errorf("expected 8 total sections, got %d", index.TotalSections)
	}
	if len(index.Pages) != 2 {
		t.Errorf("expected 2 page entries, got %d", len(index.Pages))
	}
	if index.BaseURL != "https://example.com" {
		t.Errorf("expected base URL 'https://example.com', got %q", index.BaseURL)
	}
}

func TestBuildIndex_WithErrors(t *testing.T) {
	now := time.Now()
	results := map[string]*crawler.Result{
		"https://example.com/":       {URL: "https://example.com/", HTML: "<html>ok</html>", FetchedAt: now},
		"https://example.com/broken": {URL: "https://example.com/broken", Error: http.ErrServerClosed, FetchedAt: now},
	}
	stats := crawler.Stats{
		StartedAt:    now.Add(-time.Minute),
		CompletedAt:  now,
		PagesCrawled: 1,
		PagesFailed:  1,
		Errors:       []string{"https://example.com/broken: server closed"},
	}
	sectionCounts := map[string]int{
		"https://example.com/": 2,
	}

	index := crawler.BuildIndex(results, stats, "https://example.com", sectionCounts)

	if index.PagesFailed != 1 {
		t.Errorf("expected 1 page failed, got %d", index.PagesFailed)
	}
	if index.TotalSections != 2 {
		t.Errorf("expected 2 total sections (only from successful page), got %d", index.TotalSections)
	}

	// Check that error page has correct status
	var errorPage *crawler.PageEntry
	for i := range index.Pages {
		if index.Pages[i].URL == "https://example.com/broken" {
			errorPage = &index.Pages[i]
			break
		}
	}
	if errorPage == nil {
		t.Fatal("expected to find error page in index")
	}
	if errorPage.Status != "error" {
		t.Errorf("expected status 'error', got %q", errorPage.Status)
	}
	if errorPage.Error == "" {
		t.Error("expected error message to be set")
	}
}

func TestBuildIndex_SortedByURL(t *testing.T) {
	now := time.Now()
	results := map[string]*crawler.Result{
		"https://example.com/z": {URL: "https://example.com/z", HTML: "<html>z</html>", FetchedAt: now},
		"https://example.com/a": {URL: "https://example.com/a", HTML: "<html>a</html>", FetchedAt: now},
		"https://example.com/m": {URL: "https://example.com/m", HTML: "<html>m</html>", FetchedAt: now},
	}
	stats := crawler.Stats{PagesCrawled: 3}

	index := crawler.BuildIndex(results, stats, "https://example.com", nil)

	if len(index.Pages) != 3 {
		t.Fatalf("expected 3 pages, got %d", len(index.Pages))
	}

	// Verify sorted order
	if index.Pages[0].URL != "https://example.com/a" {
		t.Errorf("expected first page to be /a, got %s", index.Pages[0].URL)
	}
	if index.Pages[1].URL != "https://example.com/m" {
		t.Errorf("expected second page to be /m, got %s", index.Pages[1].URL)
	}
	if index.Pages[2].URL != "https://example.com/z" {
		t.Errorf("expected third page to be /z, got %s", index.Pages[2].URL)
	}
}
