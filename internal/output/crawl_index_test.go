package output_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"go_scrap/internal/crawler"
	"go_scrap/internal/output"
)

func TestWriteCrawlIndex(t *testing.T) {
	dir := t.TempDir()
	index := crawler.CrawlIndex{
		StartedAt:     time.Now(),
		CompletedAt:   time.Now(),
		BaseURL:       "https://example.com",
		PagesCrawled:  2,
		PagesFailed:   0,
		TotalSections: 5,
	}

	if err := output.WriteCrawlIndex(dir, index, true); err != nil {
		t.Fatalf("WriteCrawlIndex error: %v", err)
	}

	path := filepath.Join(dir, "crawl-index.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("missing crawl-index.json: %v", err)
	}
}

func TestBuildCrawlIndex_UsesSectionCounts(t *testing.T) {
	results := map[string]*crawler.Result{
		"https://example.com/a": {URL: "https://example.com/a", HTML: "<p>a</p>", FetchedAt: time.Now()},
	}
	stats := crawler.Stats{PagesCrawled: 1}
	sections := []output.PageSectionCount{{URL: "https://example.com/a", Sections: 3}}

	index := output.BuildCrawlIndex(results, stats, "https://example.com", sections)
	if index.TotalSections != 3 {
		t.Fatalf("expected total sections 3, got %d", index.TotalSections)
	}
	if len(index.Pages) != 1 || index.Pages[0].SectionCount != 3 {
		t.Fatalf("expected page section count 3, got %#v", index.Pages)
	}
}

func TestReadCrawlIndex(t *testing.T) {
	dir := t.TempDir()
	index := crawler.CrawlIndex{
		StartedAt:     time.Now(),
		CompletedAt:   time.Now(),
		BaseURL:       "https://example.com",
		PagesCrawled:  1,
		PagesFailed:   0,
		TotalSections: 1,
		Pages: []crawler.PageEntry{
			{
				URL:          "https://example.com",
				Status:       "success",
				SectionCount: 1,
				FetchedAt:    time.Now(),
				ContentHash:  "hash",
			},
		},
	}

	if err := output.WriteCrawlIndex(dir, index, true); err != nil {
		t.Fatalf("WriteCrawlIndex error: %v", err)
	}

	readIndex, err := output.ReadCrawlIndex(dir)
	if err != nil {
		t.Fatalf("ReadCrawlIndex error: %v", err)
	}
	if len(readIndex.Pages) != 1 || readIndex.Pages[0].ContentHash != "hash" {
		t.Fatalf("expected content hash to round trip, got %#v", readIndex.Pages)
	}
}
