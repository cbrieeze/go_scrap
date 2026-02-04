package app

import (
	"context"
	"fmt"
	"os"
	"time"

	"go_scrap/internal/fetch"

	"github.com/PuerkitoBio/goquery"
)

func prepareBaseDocument(ctx context.Context, pipeline *pipeline, opts Options) (*goquery.Document, fetch.Result, error) {
	result, err := fetchResult(ctx, opts)
	if err != nil {
		return nil, fetch.Result{}, err
	}

	baseDoc, err := pipeline.prepareDocument(ctx, opts, result.HTML)
	if err != nil {
		return nil, fetch.Result{}, err
	}

	return baseDoc, result, nil
}

func fetchResult(ctx context.Context, opts Options) (fetch.Result, error) {
	mode := opts.Mode
	if opts.NavWalk {
		mode = fetch.ModeDynamic
	}

	if opts.UseCache {
		cachePath := fetch.GetCachePath(opts.URL)
		if content, err := os.ReadFile(cachePath); err == nil {
			return fetch.Result{HTML: string(content), SourceInfo: "cache"}, nil
		}
	}

	var result fetch.Result
	var err error
	backoffs := []time.Duration{0, time.Second, 2 * time.Second}
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(backoffs[attempt])
			if !opts.Stdout {
				fmt.Fprintf(os.Stderr, "Fetch attempt %d failed. Retrying...\n", attempt)
			}
		}
		result, err = fetch.Fetch(ctx, buildFetchOptions(opts, mode))
		if err == nil || ctx.Err() != nil {
			break
		}
	}
	if err != nil {
		return fetch.Result{}, err
	}

	if opts.UseCache {
		cachePath := fetch.GetCachePath(opts.URL)
		_ = fetch.SaveToCache(cachePath, result.HTML)
	}

	return result, nil
}

func buildFetchOptions(opts Options, mode fetch.Mode) fetch.Options {
	return fetch.Options{
		URL:                opts.URL,
		Mode:               mode,
		Timeout:            opts.Timeout,
		UserAgent:          opts.UserAgent,
		WaitForSelector:    opts.WaitFor,
		Headless:           opts.Headless,
		RateLimitPerSecond: opts.RateLimitPerSecond,
		ProxyURL:           opts.ProxyURL,
		Headers:            opts.AuthHeaders,
		Cookies:            opts.AuthCookies,
	}
}
