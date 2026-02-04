package output

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"go_scrap/internal/crawler"
)

type PageSectionCount struct {
	URL      string
	Sections int
}

func BuildCrawlIndex(results map[string]*crawler.Result, stats crawler.Stats, baseURL string, sections []PageSectionCount) crawler.CrawlIndex {
	counts := map[string]int{}
	for _, s := range sections {
		if s.URL == "" {
			continue
		}
		counts[s.URL] = s.Sections
	}
	return crawler.BuildIndex(results, stats, baseURL, counts)
}

func WriteCrawlIndexFromPages(outputDir string, results map[string]*crawler.Result, stats crawler.Stats, baseURL string, sections []PageSectionCount, silent bool) error {
	index := BuildCrawlIndex(results, stats, baseURL, sections)
	return WriteCrawlIndex(outputDir, index, silent)
}

func WriteCrawlIndex(outputDir string, index crawler.CrawlIndex, silent bool) error {
	if outputDir == "" {
		outputDir = "output"
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	indexPath := filepath.Join(outputDir, "crawl-index.json")
	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(indexPath, data, 0600); err != nil {
		return err
	}

	if !silent {
		fmt.Printf("Wrote crawl index: %s (%d pages, %d total sections)\n",
			indexPath, index.PagesCrawled, index.TotalSections)
	}

	return nil
}

func ReadCrawlIndex(outputDir string) (crawler.CrawlIndex, error) {
	if outputDir == "" {
		outputDir = "output"
	}
	indexPath := filepath.Join(outputDir, "crawl-index.json")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return crawler.CrawlIndex{}, err
	}
	var index crawler.CrawlIndex
	if err := json.Unmarshal(data, &index); err != nil {
		return crawler.CrawlIndex{}, err
	}
	return index, nil
}
