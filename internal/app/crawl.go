package app

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go_scrap/internal/crawler"
	"go_scrap/internal/output"
)

func initCrawler(ctx context.Context, opts Options) (*crawler.Crawler, string, error) {
	urlFilter, err := buildURLFilter(opts.CrawlFilter)
	if err != nil {
		return nil, "", err
	}

	baseURL, err := determineBaseURL(opts)
	if err != nil {
		return nil, "", err
	}

	crawlerOpts := buildCrawlerOptions(opts, baseURL, urlFilter)

	c, err := crawler.New(crawlerOpts)
	if err != nil {
		return nil, "", fmt.Errorf("create crawler: %w", err)
	}

	if err := addSitemapURLs(ctx, c, opts); err != nil {
		return nil, "", err
	}

	return c, baseURL, nil
}

func buildURLFilter(filter string) (*regexp.Regexp, error) {
	if filter == "" {
		return nil, nil
	}
	urlFilter, err := regexp.Compile(filter)
	if err != nil {
		return nil, fmt.Errorf("invalid crawl filter regex: %w", err)
	}
	return urlFilter, nil
}

func determineBaseURL(opts Options) (string, error) {
	if opts.URL != "" {
		return opts.URL, nil
	}
	if opts.SitemapURL != "" {
		u, err := url.Parse(opts.SitemapURL)
		if err != nil {
			return "", fmt.Errorf("invalid sitemap URL: %w", err)
		}
		return u.Scheme + "://" + u.Host, nil
	}
	return "", fmt.Errorf("no URL or sitemap URL provided")
}

func buildCrawlerOptions(opts Options, baseURL string, urlFilter *regexp.Regexp) crawler.Options {
	crawlerOpts := crawler.Options{
		BaseURL:     baseURL,
		RateLimit:   opts.RateLimitPerSecond,
		Parallelism: 2,
		UserAgent:   opts.UserAgent,
		MaxDepth:    opts.CrawlDepth,
		MaxPages:    opts.MaxPages,
		URLFilter:   urlFilter,
		Timeout:     opts.Timeout,
		ProxyURL:    opts.ProxyURL,
		Headers:     opts.AuthHeaders,
		Cookies:     opts.AuthCookies,
	}
	if crawlerOpts.RateLimit <= 0 {
		crawlerOpts.RateLimit = 1.0
	}
	return crawlerOpts
}

func addSitemapURLs(ctx context.Context, c *crawler.Crawler, opts Options) error {
	if opts.SitemapURL == "" {
		return nil
	}
	sitemapURLs, err := crawler.ParseSitemap(ctx, opts.SitemapURL, crawler.SitemapOptions{
		UserAgent: opts.UserAgent,
		Timeout:   opts.Timeout,
	})
	if err != nil {
		return fmt.Errorf("parse sitemap: %w", err)
	}
	if !opts.Stdout {
		fmt.Printf("Found %d URLs in sitemap\n", len(sitemapURLs))
	}
	if err := c.AddURLs(sitemapURLs); err != nil {
		return fmt.Errorf("add sitemap URLs: %w", err)
	}
	return nil
}

func processCrawlResults(ctx context.Context, pipeline *pipeline, opts Options, results map[string]*crawler.Result, stats crawler.Stats) error {
	pagesDir := filepath.Join(opts.OutputDir, "pages")
	pageSections := []output.PageSectionCount{}
	resumeEntries, err := loadResumeEntries(opts)
	if err != nil {
		return err
	}

	for pageURL, result := range results {
		if resumeEntry, ok := resumeEntries[pageURL]; ok && shouldResumeSkip(opts, result, resumeEntry) {
			pageDir, dirErr := urlToOutputDir(pageURL, pagesDir)
			if dirErr == nil {
				if _, err := os.Stat(pageDir); err == nil {
					if resumeEntry.Status == "success" {
						pageSections = append(pageSections, output.PageSectionCount{
							URL:      pageURL,
							Sections: resumeEntry.SectionCount,
						})
					}
					if !opts.Stdout {
						fmt.Printf("Skipped (unchanged): %s\n", pageDir)
					}
					continue
				}
			}
		}

		summary := pipeline.processCrawlPage(ctx, opts, pageURL, result, pagesDir)
		if summary.Processed {
			pageSections = append(pageSections, output.PageSectionCount{
				URL:      pageURL,
				Sections: summary.Sections,
			})
			if !opts.Stdout {
				fmt.Printf("Wrote: %s (%d sections)\n", summary.OutputDir, summary.Sections)
			}
			continue
		}
		if summary.Skipped {
			fmt.Fprintf(os.Stderr, "Warning: skipping %s: %s\n", pageURL, summary.SkipReason)
			continue
		}
		if summary.ProcessError != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to process %s: %v\n", pageURL, summary.ProcessError)
		}
	}

	baseURL, _ := determineBaseURL(opts)
	if err := output.WriteCrawlIndexFromPages(opts.OutputDir, results, stats, baseURL, pageSections, opts.Stdout); err != nil {
		return fmt.Errorf("write crawl index: %w", err)
	}

	return nil
}

func loadResumeEntries(opts Options) (map[string]crawler.PageEntry, error) {
	if !opts.Resume {
		return nil, nil
	}
	index, err := output.ReadCrawlIndex(opts.OutputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read crawl index: %w", err)
	}
	entries := make(map[string]crawler.PageEntry, len(index.Pages))
	for _, page := range index.Pages {
		entries[page.URL] = page
	}
	return entries, nil
}

func shouldResumeSkip(opts Options, result *crawler.Result, entry crawler.PageEntry) bool {
	if !opts.Resume {
		return false
	}
	if result == nil || result.Error != nil || result.ContentHash == "" {
		return false
	}
	return entry.Status == "success" && entry.ContentHash != "" && entry.ContentHash == result.ContentHash
}

func urlToOutputDir(pageURL, baseDir string) (string, error) {
	u, err := url.Parse(pageURL)
	if err != nil {
		return "", err
	}

	path := strings.TrimPrefix(u.Path, "/")
	if path == "" {
		path = "index"
	}

	path = strings.ReplaceAll(path, "\\", "/")
	parts := strings.Split(path, "/")
	for i, part := range parts {
		parts[i] = sanitizePathComponent(part)
	}

	return filepath.Join(baseDir, filepath.Join(parts...)), nil
}

func sanitizePathComponent(s string) string {
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, "?", "_")
	s = strings.ReplaceAll(s, "*", "_")
	s = strings.ReplaceAll(s, "\"", "_")
	s = strings.ReplaceAll(s, "<", "_")
	s = strings.ReplaceAll(s, ">", "_")
	s = strings.ReplaceAll(s, "|", "_")
	if s == "" {
		s = "_"
	}
	return s
}
