package app

import (
	"context"
	"fmt"
	"time"

	"go_scrap/internal/fetch"
)

type Options struct {
	URL                string
	Mode               fetch.Mode
	OutputDir          string
	Timeout            time.Duration
	UserAgent          string
	WaitFor            string
	Headless           bool
	RateLimitPerSecond float64
	Yes                bool
	Strict             bool
	DryRun             bool
	Stdout             bool
	UseCache           bool
	DownloadAssets     bool
	NavSelector        string
	ContentSelector    string
	ExcludeSelector    string
	NavWalk            bool
	MaxSections        int
	MaxMenuItems       int
	MaxMarkdownBytes   int
	MaxChars           int
	MaxTokens          int
	ProxyURL           string
	AuthHeaders        map[string]string
	AuthCookies        map[string]string
	PipelineHooks      []string
	PostCommands       []string
	Crawl              bool
	Resume             bool
	SitemapURL         string
	MaxPages           int
	CrawlDepth         int
	CrawlFilter        string
}

func Run(ctx context.Context, opts Options) error {
	normalized, err := normalizeOptions(opts)
	if err != nil {
		return err
	}

	if normalized.Crawl {
		return runCrawl(ctx, normalized)
	}
	return runSingle(ctx, normalized)
}

func runSingle(ctx context.Context, opts Options) error {
	pipeline, err := newPipeline(opts)
	if err != nil {
		return err
	}
	baseDoc, fetchResult, err := prepareBaseDocument(ctx, pipeline, opts)
	if err != nil {
		return err
	}

	analysis, err := pipeline.analyze(ctx, opts, baseDoc, true)
	if err != nil {
		return err
	}
	pipeline.summarize(opts, fetchResult.SourceInfo, analysis)

	if !pipeline.shouldWrite(opts) {
		return nil
	}

	analysis.Trim(opts.MaxSections)
	return pipeline.writeOutputs(ctx, opts, baseDoc, analysis)
}

func runCrawl(ctx context.Context, opts Options) error {
	pipeline, err := newPipeline(opts)
	if err != nil {
		return err
	}
	c, baseURL, err := initCrawler(ctx, opts)
	if err != nil {
		return err
	}

	if !opts.Stdout {
		fmt.Printf("Starting crawl from %s (max %d pages, depth %d)\n", baseURL, opts.MaxPages, opts.CrawlDepth)
	}

	results, stats, err := c.Crawl(ctx)
	if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
		return fmt.Errorf("crawl failed: %w", err)
	}

	if !opts.Stdout {
		fmt.Printf("Crawl complete: %d pages crawled, %d failed\n", stats.PagesCrawled, stats.PagesFailed)
	}

	if !pipeline.shouldWrite(opts) {
		return nil
	}

	return processCrawlResults(ctx, pipeline, opts, results, stats)
}
