package crawler

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type urlset struct {
	XMLName xml.Name     `xml:"urlset"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

type sitemapIndex struct {
	XMLName  xml.Name          `xml:"sitemapindex"`
	Sitemaps []sitemapLocation `xml:"sitemap"`
}

type sitemapLocation struct {
	Loc string `xml:"loc"`
}

type SitemapOptions struct {
	UserAgent string
	Timeout   time.Duration
}

func ParseSitemap(ctx context.Context, sitemapURL string, opts SitemapOptions) ([]string, error) {
	if opts.UserAgent == "" {
		opts.UserAgent = "go_scrap/1.0"
	}
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}

	body, err := fetchSitemapContent(ctx, sitemapURL, opts)
	if err != nil {
		return nil, err
	}

	// Try parsing as sitemap index first
	urls, err := parseSitemapIndex(ctx, body, opts)
	if err == nil && len(urls) > 0 {
		return urls, nil
	}

	// Parse as regular urlset
	return parseURLSet(body)
}

func fetchSitemapContent(ctx context.Context, url string, opts SitemapOptions) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", opts.UserAgent)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch sitemap: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sitemap returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read sitemap body: %w", err)
	}

	return body, nil
}

func parseSitemapIndex(ctx context.Context, body []byte, opts SitemapOptions) ([]string, error) {
	var index sitemapIndex
	if err := xml.Unmarshal(body, &index); err != nil {
		return nil, err
	}

	if len(index.Sitemaps) == 0 {
		return nil, fmt.Errorf("no sitemaps in index")
	}

	var allURLs []string
	for _, sitemap := range index.Sitemaps {
		if strings.TrimSpace(sitemap.Loc) == "" {
			continue
		}

		urls, err := ParseSitemap(ctx, sitemap.Loc, opts)
		if err != nil {
			// Continue with other sitemaps even if one fails
			continue
		}
		allURLs = append(allURLs, urls...)
	}

	return allURLs, nil
}

func parseURLSet(body []byte) ([]string, error) {
	var set urlset
	if err := xml.Unmarshal(body, &set); err != nil {
		return nil, fmt.Errorf("parse sitemap XML: %w", err)
	}

	urls := make([]string, 0, len(set.URLs))
	for _, u := range set.URLs {
		loc := strings.TrimSpace(u.Loc)
		if loc != "" {
			urls = append(urls, loc)
		}
	}

	return urls, nil
}
