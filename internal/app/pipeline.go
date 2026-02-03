package app

import (
	"context"
	"fmt"

	"go_scrap/internal/crawler"
	"go_scrap/internal/markdown"
	"go_scrap/internal/output"
	"go_scrap/internal/parse"
	"go_scrap/internal/report"

	"github.com/PuerkitoBio/goquery"
)

type pipeline struct {
	conv *markdown.Converter
}

type analysisResult struct {
	Doc *parse.Document
	Rep report.Report
}

func (r analysisResult) Trim(maxSections int) {
	if r.Doc == nil {
		return
	}
	trimSections(r.Doc, maxSections)
}

func (r analysisResult) SectionsCount() int {
	if r.Doc == nil {
		return 0
	}
	return len(r.Doc.Sections)
}

func newPipeline() *pipeline {
	return &pipeline{conv: markdown.NewConverter()}
}

func (p *pipeline) analyze(ctx context.Context, opts Options, baseDoc *goquery.Document, allowNavWalk bool) (analysisResult, error) {
	var (
		doc *parse.Document
		err error
	)
	if allowNavWalk {
		doc, err = buildDocument(ctx, opts, baseDoc)
	} else {
		doc, err = parseDocuments(baseDoc, opts.ContentSelector)
	}
	if err != nil {
		return analysisResult{}, err
	}
	return analysisResult{Doc: doc, Rep: report.Analyze(doc)}, nil
}

func (p *pipeline) prepareDocument(_ context.Context, opts Options, html string) (*goquery.Document, error) {
	doc, err := parse.NewDocument(html)
	if err != nil {
		return nil, err
	}
	applyExclusions(doc, opts.ExcludeSelector)
	if opts.DownloadAssets && !opts.DryRun {
		if err := output.Download(doc, opts.URL, opts.OutputDir, opts.UserAgent); err != nil && !opts.Stdout {
			fmt.Printf("Warning: asset processing failed: %v\n", err)
		}
	}
	return doc, nil
}

func (p *pipeline) renderSections(sections []parse.Section) (string, []sectionMarkdown, error) {
	return buildMarkdown(p.conv, sections)
}

func (p *pipeline) writeOutputs(opts Options, baseDoc *goquery.Document, result analysisResult) error {
	md, sectionMarkdowns, err := p.renderSections(result.Doc.Sections)
	if err != nil {
		return err
	}
	return writeOutputsWithMarkdown(opts, baseDoc, result, md, sectionMarkdowns)
}

type crawlPageSummary struct {
	URL          string
	Sections     int
	OutputDir    string
	Skipped      bool
	SkipReason   string
	Processed    bool
	ProcessError error
}

func (p *pipeline) processCrawlPage(ctx context.Context, opts Options, pageURL string, result *crawler.Result, pagesDir string) crawlPageSummary {
	summary := crawlPageSummary{URL: pageURL}
	if result == nil || result.Error != nil || result.HTML == "" {
		summary.Skipped = true
		summary.SkipReason = "empty or errored result"
		return summary
	}

	pageDir, err := urlToOutputDir(pageURL, pagesDir)
	if err != nil {
		summary.Skipped = true
		summary.SkipReason = err.Error()
		return summary
	}
	summary.OutputDir = pageDir

	pageOpts := opts
	pageOpts.URL = pageURL
	pageOpts.OutputDir = pageDir

	baseDoc, err := p.prepareDocument(ctx, pageOpts, result.HTML)
	if err != nil {
		summary.Skipped = true
		summary.SkipReason = err.Error()
		return summary
	}

	analysis, err := p.analyze(ctx, pageOpts, baseDoc, false)
	if err != nil {
		summary.ProcessError = err
		return summary
	}
	analysis.Trim(opts.MaxSections)
	summary.Sections = analysis.SectionsCount()

	if err := p.writeOutputs(pageOpts, baseDoc, analysis); err != nil {
		summary.ProcessError = err
		return summary
	}

	summary.Processed = true
	return summary
}

func (p *pipeline) summarize(opts Options, sourceInfo string, result analysisResult) {
	printSummaryIfNeeded(opts, sourceInfo, result.Doc, result.Rep)
}

func (p *pipeline) shouldWrite(opts Options) bool {
	if opts.DryRun {
		fmt.Println("\nDry run complete (no files written).")
		return false
	}
	if opts.Yes {
		return true
	}
	if confirm("Continue and generate outputs? [y/N]: ") {
		return true
	}
	fmt.Println("Aborted.")
	return false
}
