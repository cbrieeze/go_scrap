package crawler

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
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
}

type Result struct {
	URL       string
	HTML      string
	Error     error
	FetchedAt time.Time
}

type Stats struct {
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
	PagesCrawled int       `json:"pages_crawled"`
	PagesFailed  int       `json:"pages_failed"`
	Errors       []string  `json:"errors,omitempty"`
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

func (cr *Crawler) setupCallbacks(c *colly.Collector) {
	c.OnHTML("html", cr.handleHTMLResponse)
	c.OnHTML("a[href]", cr.handleLink)
	c.OnError(cr.handleError)
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
		URL:       e.Request.URL.String(),
		HTML:      html,
		FetchedAt: time.Now(),
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

func (c *Crawler) Crawl(ctx context.Context) (map[string]*Result, Stats, error) {
	c.mu.Lock()
	c.urlCount = 1 // Start URL counts as 1
	c.mu.Unlock()

	if err := c.collector.Visit(c.opts.BaseURL); err != nil {
		return nil, c.stats, fmt.Errorf("failed to start crawl: %w", err)
	}

	// Wait for all requests to complete
	done := make(chan struct{})
	go func() {
		c.collector.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return c.results, c.stats, ctx.Err()
	case <-done:
		// Crawl completed normally
	}

	c.stats.CompletedAt = time.Now()
	return c.results, c.stats, nil
}

func (c *Crawler) AddURL(url string) error {
	c.mu.Lock()
	if c.urlCount >= c.opts.MaxPages {
		c.mu.Unlock()
		return fmt.Errorf("max pages limit reached")
	}
	c.urlCount++
	c.mu.Unlock()

	return c.collector.Visit(url)
}

func (c *Crawler) AddURLs(urls []string) error {
	for _, u := range urls {
		if err := c.AddURL(u); err != nil {
			return err
		}
	}
	return nil
}
