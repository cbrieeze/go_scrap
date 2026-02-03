package crawler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go_scrap/internal/crawler"
)

func TestParseSitemap_BasicURLSet(t *testing.T) {
	sitemap := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/page1</loc></url>
  <url><loc>https://example.com/page2</loc></url>
  <url><loc>https://example.com/page3</loc></url>
</urlset>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(sitemap))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	urls, err := crawler.ParseSitemap(ctx, srv.URL, crawler.SitemapOptions{})
	if err != nil {
		t.Fatalf("parse sitemap failed: %v", err)
	}

	if len(urls) != 3 {
		t.Fatalf("expected 3 URLs, got %d", len(urls))
	}

	expected := []string{
		"https://example.com/page1",
		"https://example.com/page2",
		"https://example.com/page3",
	}
	for i, u := range urls {
		if u != expected[i] {
			t.Errorf("url %d: expected %s, got %s", i, expected[i], u)
		}
	}
}

func TestParseSitemap_SitemapIndex(t *testing.T) {
	mux := http.NewServeMux()

	// Sitemap index
	mux.HandleFunc("/sitemap-index.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <sitemap><loc>` + "http://" + r.Host + `/sitemap1.xml</loc></sitemap>
  <sitemap><loc>` + "http://" + r.Host + `/sitemap2.xml</loc></sitemap>
</sitemapindex>`))
	})

	// Child sitemaps
	mux.HandleFunc("/sitemap1.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/page1</loc></url>
</urlset>`))
	})

	mux.HandleFunc("/sitemap2.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/page2</loc></url>
</urlset>`))
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	urls, err := crawler.ParseSitemap(ctx, srv.URL+"/sitemap-index.xml", crawler.SitemapOptions{})
	if err != nil {
		t.Fatalf("parse sitemap failed: %v", err)
	}

	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs from sitemap index, got %d", len(urls))
	}
}

func TestParseSitemap_EmptyURLs(t *testing.T) {
	sitemap := `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
  <url><loc>https://example.com/valid</loc></url>
  <url><loc>  </loc></url>
  <url><loc></loc></url>
</urlset>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(sitemap))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	urls, err := crawler.ParseSitemap(ctx, srv.URL, crawler.SitemapOptions{})
	if err != nil {
		t.Fatalf("parse sitemap failed: %v", err)
	}

	if len(urls) != 1 {
		t.Fatalf("expected 1 valid URL, got %d", len(urls))
	}

	if urls[0] != "https://example.com/valid" {
		t.Errorf("expected valid URL, got %s", urls[0])
	}
}

func TestParseSitemap_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := crawler.ParseSitemap(ctx, srv.URL, crawler.SitemapOptions{})
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}
