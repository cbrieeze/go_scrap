package crawler

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
)

type Options struct {
	BaseURL         string
	RateLimit       float64 // requests per second per domain
	Parallelism     int     // concurrent requests (default: 2)
	UserAgent       string
	MaxDepth        int            // max link depth from start URL
	MaxPages        int            // max pages to crawl
	URLFilter       *regexp.Regexp // filter URLs to crawl
	Timeout         time.Duration
	AllowAllDomains bool // disable domain restriction (for testing)
	ProxyURL        string
	Headers         map[string]string
	Cookies         map[string]string
}

type Result struct {
	URL         string
	HTML        string
	Error       error
	FetchedAt   time.Time
	ContentHash string
}

type Stats struct {
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
	PagesCrawled int       `json:"pages_crawled"`
	PagesFailed  int       `json:"pages_failed"`
	Errors       []string  `json:"errors,omitempty"`
}

// PageEntry represents a single crawled page in the index.
type PageEntry struct {
	URL           string    `json:"url"`
	Status        string    `json:"status"` // "success", "error"
	SectionCount  int       `json:"section_count,omitempty"`
	FetchedAt     time.Time `json:"fetched_at"`
	Error         string    `json:"error,omitempty"`
	ContentLength int       `json:"content_length,omitempty"`
	ContentHash   string    `json:"content_hash,omitempty"`
}

// CrawlIndex is a comprehensive summary of a crawl operation.
type CrawlIndex struct {
	StartedAt     time.Time   `json:"started_at"`
	CompletedAt   time.Time   `json:"completed_at"`
	BaseURL       string      `json:"base_url"`
	PagesCrawled  int         `json:"pages_crawled"`
	PagesFailed   int         `json:"pages_failed"`
	TotalSections int         `json:"total_sections"`
	Pages         []PageEntry `json:"pages"`
	Errors        []string    `json:"errors,omitempty"`
}

type Crawler struct {
	collector *colly.Collector
	opts      Options
	results   map[string]*Result
	mu        sync.Mutex
	stats     Stats
	urlCount  int
}

func New(opts Options) (*Crawler, error) {
	baseURL, err := validateAndNormalizeOptions(&opts)
	if err != nil {
		return nil, err
	}

	var c *colly.Collector
	if opts.AllowAllDomains {
		c = colly.NewCollector(
			colly.MaxDepth(opts.MaxDepth),
			colly.Async(true),
			colly.UserAgent(opts.UserAgent),
		)
	} else {
		c = colly.NewCollector(
			colly.AllowedDomains(baseURL.Host),
			colly.MaxDepth(opts.MaxDepth),
			colly.Async(true),
			colly.UserAgent(opts.UserAgent),
		)
	}

	configureRateLimiting(c, opts)
	if err := configureProxy(c, opts); err != nil {
		return nil, err
	}

	crawler := &Crawler{
		collector: c,
		opts:      opts,
		results:   make(map[string]*Result),
		stats:     Stats{StartedAt: time.Now()},
	}

	crawler.setupCallbacks(c)
	return crawler, nil
}

func validateAndNormalizeOptions(opts *Options) (*url.URL, error) {
	if opts.BaseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	baseURL, err := url.Parse(opts.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	if opts.Parallelism <= 0 {
		opts.Parallelism = 2
	}
	if opts.MaxDepth <= 0 {
		opts.MaxDepth = 2
	}
	if opts.MaxPages <= 0 {
		opts.MaxPages = 100
	}
	if opts.UserAgent == "" {
		opts.UserAgent = "go_scrap/1.0"
	}
	if opts.RateLimit <= 0 {
		opts.RateLimit = 1.0
	}

	return baseURL, nil
}

func configureRateLimiting(c *colly.Collector, opts Options) {
	delay := time.Duration(float64(time.Second) / opts.RateLimit)
	_ = c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: opts.Parallelism,
		Delay:       delay,
	})

	if opts.Timeout > 0 {
		c.SetRequestTimeout(opts.Timeout)
	}
}

func configureProxy(c *colly.Collector, opts Options) error {
	if opts.ProxyURL == "" {
		return nil
	}
	if err := c.SetProxy(opts.ProxyURL); err != nil {
		return fmt.Errorf("set proxy: %w", err)
	}
	return nil
}

func (cr *Crawler) setupCallbacks(c *colly.Collector) {
	c.OnHTML("html", cr.handleHTMLResponse)
	c.OnHTML("a[href]", cr.handleLink)
	c.OnError(cr.handleError)
	c.OnRequest(func(r *colly.Request) {
		applyRequestHeaders(r, cr.opts.Headers, cr.opts.Cookies)
	})
}

func (cr *Crawler) handleHTMLResponse(e *colly.HTMLElement) {
	cr.mu.Lock()
	defer cr.mu.Unlock()

	html, err := e.DOM.Html()
	if err != nil {
		cr.recordError(e.Request.URL.String(), err)
		return
	}

	cr.results[e.Request.URL.String()] = &Result{
		URL:         e.Request.URL.String(),
		HTML:        html,
		FetchedAt:   time.Now(),
		ContentHash: hashHTML(html),
	}
	cr.stats.PagesCrawled++
}

func (cr *Crawler) handleLink(e *colly.HTMLElement) {
	link := e.Attr("href")
	if !isValidLink(link) {
		return
	}

	absURL := e.Request.AbsoluteURL(link)
	if absURL == "" {
		return
	}

	if cr.opts.URLFilter != nil && !cr.opts.URLFilter.MatchString(absURL) {
		return
	}

	if !cr.incrementURLCount() {
		return
	}

	_ = e.Request.Visit(absURL)
}

func (cr *Crawler) handleError(r *colly.Response, err error) {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	cr.recordError(r.Request.URL.String(), err)
}

func (cr *Crawler) recordError(urlStr string, err error) {
	cr.results[urlStr] = &Result{
		URL:       urlStr,
		Error:     err,
		FetchedAt: time.Now(),
	}
	cr.stats.PagesFailed++
	cr.stats.Errors = append(cr.stats.Errors, fmt.Sprintf("%s: %v", urlStr, err))
}

func (cr *Crawler) incrementURLCount() bool {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	if cr.urlCount >= cr.opts.MaxPages {
		return false
	}
	cr.urlCount++
	return true
}

func isValidLink(link string) bool {
	if link == "" {
		return false
	}
	return !strings.HasPrefix(link, "#") &&
		!strings.HasPrefix(link, "javascript:") &&
		!strings.HasPrefix(link, "mailto:")
}

func applyRequestHeaders(r *colly.Request, headers map[string]string, cookies map[string]string) {
	for key, value := range headers {
		r.Headers.Set(key, value)
	}
	cookieHeader := buildCookieHeader(cookies)
	if cookieHeader == "" {
		return
	}
	if existing := r.Headers.Get("Cookie"); existing != "" {
		r.Headers.Set("Cookie", existing+"; "+cookieHeader)
		return
	}
	r.Headers.Set("Cookie", cookieHeader)
}

func buildCookieHeader(cookies map[string]string) string {
	if len(cookies) == 0 {
		return ""
	}
	keys := make([]string, 0, len(cookies))
	for key := range cookies {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", key, cookies[key]))
	}
	return strings.Join(parts, "; ")
}

func (cr *Crawler) Crawl(ctx context.Context) (map[string]*Result, Stats, error) {
	cr.mu.Lock()
	cr.urlCount = 1 // Start URL counts as 1
	cr.mu.Unlock()

	if err := cr.collector.Visit(cr.opts.BaseURL); err != nil {
		return nil, cr.stats, fmt.Errorf("failed to start crawl: %w", err)
	}

	// Wait for all requests to complete
	done := make(chan struct{})
	go func() {
		cr.collector.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return cr.results, cr.stats, ctx.Err()
	case <-done:
		// Crawl completed normally
	}

	cr.stats.CompletedAt = time.Now()
	return cr.results, cr.stats, nil
}

func (cr *Crawler) AddURL(url string) error {
	cr.mu.Lock()
	if cr.urlCount >= cr.opts.MaxPages {
		cr.mu.Unlock()
		return fmt.Errorf("max pages limit reached")
	}
	cr.urlCount++
	cr.mu.Unlock()

	return cr.collector.Visit(url)
}

func (cr *Crawler) AddURLs(urls []string) error {
	for _, u := range urls {
		if err := cr.AddURL(u); err != nil {
			return err
		}
	}
	return nil
}

// BuildIndex creates a CrawlIndex from the crawler results.
// sectionCounts is a map from URL to section count (provided by caller after parsing).
func BuildIndex(results map[string]*Result, stats Stats, baseURL string, sectionCounts map[string]int) CrawlIndex {
	index := CrawlIndex{
		StartedAt:    stats.StartedAt,
		CompletedAt:  stats.CompletedAt,
		BaseURL:      baseURL,
		PagesCrawled: stats.PagesCrawled,
		PagesFailed:  stats.PagesFailed,
		Pages:        make([]PageEntry, 0, len(results)),
		Errors:       stats.Errors,
	}

	for url, result := range results {
		entry := PageEntry{
			URL:       url,
			FetchedAt: result.FetchedAt,
		}

		if result.Error != nil {
			entry.Status = "error"
			entry.Error = result.Error.Error()
		} else {
			entry.Status = "success"
			entry.ContentLength = len(result.HTML)
			entry.ContentHash = result.ContentHash
			if count, ok := sectionCounts[url]; ok {
				entry.SectionCount = count
				index.TotalSections += count
			}
		}

		index.Pages = append(index.Pages, entry)
	}

	// Sort pages by URL for consistent output
	sortPageEntries(index.Pages)

	return index
}

func hashHTML(html string) string {
	sum := sha256.Sum256([]byte(html))
	return hex.EncodeToString(sum[:])
}

func sortPageEntries(pages []PageEntry) {
	for i := 0; i < len(pages)-1; i++ {
		for j := i + 1; j < len(pages); j++ {
			if pages[i].URL > pages[j].URL {
				pages[i], pages[j] = pages[j], pages[i]
			}
		}
	}
}
