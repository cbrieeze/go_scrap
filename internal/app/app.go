package app

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"go_scrap/internal/crawler"
	"go_scrap/internal/fetch"
	"go_scrap/internal/markdown"
	"go_scrap/internal/menu"
	"go_scrap/internal/output"
	"go_scrap/internal/parse"
	"go_scrap/internal/report"

	"github.com/PuerkitoBio/goquery"
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
	// Crawl mode options
	Crawl       bool
	SitemapURL  string
	MaxPages    int
	CrawlDepth  int
	CrawlFilter string
}

type menuItem struct {
	Title  string
	Anchor string
	Depth  int
}

func Run(ctx context.Context, opts Options) error {
	normalized, err := normalizeOptions(opts)
	if err != nil {
		return err
	}

	// Branch based on crawl mode
	if normalized.Crawl {
		return runCrawl(ctx, normalized)
	}

	return runSingle(ctx, normalized)
}

func runSingle(ctx context.Context, opts Options) error {
	pipeline := newPipeline()
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

	return pipeline.writeOutputs(opts, baseDoc, analysis)
}

func runCrawl(ctx context.Context, opts Options) error {
	pipeline := newPipeline()
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

	return processCrawlResults(ctx, opts, results, stats)
}

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

func processCrawlResults(ctx context.Context, opts Options, results map[string]*crawler.Result, stats crawler.Stats) error {
	pipeline := newPipeline()
	pagesDir := filepath.Join(opts.OutputDir, "pages")
	pageSections := []output.PageSectionCount{}

	for pageURL, result := range results {
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

	// Build and write crawl index
	baseURL, _ := determineBaseURL(opts)
	if err := output.WriteCrawlIndexFromPages(opts.OutputDir, results, stats, baseURL, pageSections, opts.Stdout); err != nil {
		return fmt.Errorf("write crawl index: %w", err)
	}

	return nil
}

func urlToOutputDir(pageURL, baseDir string) (string, error) {
	u, err := url.Parse(pageURL)
	if err != nil {
		return "", err
	}

	// Build path from URL path
	path := strings.TrimPrefix(u.Path, "/")
	if path == "" {
		path = "index"
	}

	// Sanitize path components
	path = strings.ReplaceAll(path, "\\", "/")
	parts := strings.Split(path, "/")
	for i, part := range parts {
		parts[i] = sanitizePathComponent(part)
	}

	return filepath.Join(baseDir, filepath.Join(parts...)), nil
}

func sanitizePathComponent(s string) string {
	// Remove or replace invalid filename characters
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

func buildDocument(ctx context.Context, opts Options, baseDoc *goquery.Document) (*parse.Document, error) {
	if opts.NavWalk && strings.TrimSpace(opts.NavSelector) != "" {
		return runNavWalk(ctx, opts, baseDoc)
	}
	return parseDocuments(baseDoc, opts.ContentSelector)
}

func printSummaryIfNeeded(opts Options, sourceInfo string, doc *parse.Document, rep report.Report) {
	if opts.Stdout {
		return
	}
	printSummary(sourceInfo, doc, rep)
}

func writeOutputsWithMarkdown(opts Options, baseDoc *goquery.Document, result analysisResult, md string, sectionMarkdowns []sectionMarkdown) error {
	if opts.Strict && reportHasIssues(result.Rep) {
		return errors.New("completeness checks failed (use --strict=false to allow)")
	}

	jsonPath, err := output.WriteJSON(result.Doc, result.Rep, output.WriteOptions{OutputDir: opts.OutputDir})
	if err != nil {
		return err
	}

	var mdPath string
	limits := chunkLimits(opts)
	contentParts := make([]string, 0, len(sectionMarkdowns))
	for _, sm := range sectionMarkdowns {
		contentParts = append(contentParts, sm.Markdown)
	}
	if limits.Enabled() {
		mdPath, err = output.WriteMarkdownParts(opts.OutputDir, "content.md", contentParts, limits)
	} else {
		mdPath, err = output.WriteMarkdown(opts.OutputDir, "content.md", md)
	}
	if err != nil {
		return err
	}

	if opts.Stdout {
		fmt.Println(md)
	} else {
		fmt.Printf("\nWrote markdown: %s\n", mdPath)
		fmt.Printf("Wrote json: %s\n", jsonPath)
	}

	if err := writeMenuOutputs(opts, baseDoc, result.Doc, sectionMarkdowns); err != nil {
		return err
	}

	if !opts.Stdout {
		if indexPath, err := output.WriteIndex(opts.OutputDir, opts.URL, result.Doc.Sections); err == nil {
			fmt.Printf("Wrote index: %s\n", indexPath)
		}
	}

	return nil
}

func normalizeOptions(opts Options) (Options, error) {
	// In crawl mode, URL can be empty if sitemap is provided
	if strings.TrimSpace(opts.URL) == "" && !opts.Crawl {
		return opts, errors.New("url is required")
	}
	if opts.Crawl && strings.TrimSpace(opts.URL) == "" && strings.TrimSpace(opts.SitemapURL) == "" {
		return opts, errors.New("url or sitemap is required for crawl mode")
	}
	if opts.Mode == "" {
		opts.Mode = fetch.ModeAuto
	}
	if opts.Timeout == 0 {
		opts.Timeout = 45 * time.Second
	}
	if opts.UserAgent == "" {
		opts.UserAgent = "go_scrap/1.0"
	}
	if opts.OutputDir == "" {
		urlForHost := opts.URL
		if urlForHost == "" {
			urlForHost = opts.SitemapURL
		}
		host := hostFromURL(urlForHost)
		if host == "" {
			host = "output"
		}
		opts.OutputDir = filepath.Join("output", host)
	}
	if opts.Stdout {
		opts.Yes = true
	}
	return opts, nil
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
		result, err = fetch.Fetch(ctx, fetch.Options{
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
		})
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

func runNavWalk(ctx context.Context, opts Options, baseDoc *goquery.Document) (*parse.Document, error) {
	nodes, err := menu.Extract(baseDoc, opts.NavSelector)
	if err != nil {
		return nil, fmt.Errorf("menu extract failed (%s): %w", opts.NavSelector, err)
	}
	items := flattenMenu(nodes)
	anchors := collectAnchors(items)

	htmlByAnchor, err := fetch.AnchorHTML(ctx, fetch.Options{
		URL:                opts.URL,
		Mode:               fetch.ModeDynamic,
		Timeout:            opts.Timeout,
		UserAgent:          opts.UserAgent,
		WaitForSelector:    opts.WaitFor,
		Headless:           opts.Headless,
		RateLimitPerSecond: opts.RateLimitPerSecond,
		ProxyURL:           opts.ProxyURL,
		Headers:            opts.AuthHeaders,
		Cookies:            opts.AuthCookies,
	}, anchors)
	if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("navwalk timed out processing %d anchors (try increasing --timeout or reducing menu depth): %w", len(anchors), err)
		}
		return nil, err
	}

	sections, headings := buildNavSections(items, anchors, htmlByAnchor, opts)

	return &parse.Document{
		HTML:               documentOuterHTML(baseDoc),
		Sections:           sections,
		HeadingIDs:         headings,
		AnchorTargets:      anchors,
		AllElementIDs:      headings,
		AnchorTargetsByRaw: anchors,
	}, nil
}

func collectAnchors(items []menuItem) []string {
	anchors := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		if item.Anchor == "" {
			continue
		}
		if _, ok := seen[item.Anchor]; ok {
			continue
		}
		seen[item.Anchor] = struct{}{}
		anchors = append(anchors, item.Anchor)
	}
	return anchors
}

func buildNavSections(items []menuItem, anchors []string, htmlByAnchor map[string]string, opts Options) ([]parse.Section, []string) {
	sections := []parse.Section{}
	headings := []string{}
	for _, item := range items {
		if item.Anchor == "" {
			continue
		}
		htmlForAnchor, ok := htmlByAnchor[item.Anchor]
		if !ok {
			continue
		}
		section, ok := buildSectionFromAnchor(item, htmlForAnchor, anchors, opts)
		if !ok {
			continue
		}
		sections = append(sections, section)
		headings = append(headings, item.Anchor)
	}
	return sections, headings
}

func buildSectionFromAnchor(item menuItem, htmlForAnchor string, anchors []string, opts Options) (parse.Section, bool) {
	anchorDoc, err := parse.NewDocument(htmlForAnchor)
	if err != nil {
		return parse.Section{}, false
	}
	contentDoc := prepareContentDoc(anchorDoc, opts, item.Anchor)

	contentHTML := documentOuterHTML(contentDoc)
	contentText := strings.TrimSpace(contentDoc.Text())
	contentIDs := documentIDs(contentDoc)
	level := 2 + item.Depth
	if level > 6 {
		level = 6
	}
	section := parse.Section{
		HeadingText:   strings.TrimSpace(item.Title),
		HeadingLevel:  level,
		HeadingID:     item.Anchor,
		ContentHTML:   contentHTML,
		ContentText:   contentText,
		AnchorTargets: anchors,
		ContentIDs:    contentIDs,
	}
	return section, true
}

func prepareContentDoc(anchorDoc *goquery.Document, opts Options, anchor string) *goquery.Document {
	applyExclusions(anchorDoc, opts.ExcludeSelector)
	if opts.DownloadAssets && !opts.DryRun {
		_ = output.Download(anchorDoc, opts.URL, opts.OutputDir, opts.UserAgent)
	}
	baseDoc := anchorDoc
	if strings.TrimSpace(opts.ContentSelector) != "" {
		extracted, err := parse.ExtractBySelector(anchorDoc, opts.ContentSelector)
		if err == nil && extracted != nil {
			baseDoc = extracted
		}
	}
	if strings.TrimSpace(anchor) != "" {
		if sliced, ok := sliceByAnchor(baseDoc, anchor); ok {
			return sliced
		}
		if baseDoc != anchorDoc {
			if sliced, ok := sliceByAnchor(anchorDoc, anchor); ok {
				return sliced
			}
		}
	}
	return baseDoc
}

func sliceByAnchor(doc *goquery.Document, anchor string) (*goquery.Document, bool) {
	if doc == nil || doc.Selection == nil {
		return nil, false
	}
	anchor = strings.TrimSpace(anchor)
	if anchor == "" {
		return nil, false
	}
	selector := fmt.Sprintf(`[id="%s"]`, escapeCSSAttrValue(anchor))
	sel := doc.Find(selector).First()
	if sel.Length() == 0 {
		return nil, false
	}

	tag := strings.ToLower(goquery.NodeName(sel))
	if isHeadingTag(tag) {
		siblings := sel.NextUntil("h1, h2, h3, h4, h5, h6")
		html := selectionOuterHTML(siblings)
		if strings.TrimSpace(html) == "" {
			return nil, false
		}
		wrapped := "<div>" + html + "</div>"
		sliced, err := parse.NewDocument(wrapped)
		if err != nil {
			return nil, false
		}
		return sliced, true
	}

	clone := sel.Clone()
	clone.Find("h1, h2, h3, h4, h5, h6").First().Remove()
	node := clone.Get(0)
	if node == nil {
		return nil, false
	}
	return goquery.NewDocumentFromNode(node), true
}

func selectionOuterHTML(sel *goquery.Selection) string {
	if sel == nil {
		return ""
	}
	var htmlBuf strings.Builder
	sel.Each(func(_ int, s *goquery.Selection) {
		if h, err := goquery.OuterHtml(s); err == nil {
			htmlBuf.WriteString(h)
		}
	})
	return htmlBuf.String()
}

func escapeCSSAttrValue(value string) string {
	return strings.ReplaceAll(value, `"`, `\"`)
}

func isHeadingTag(tag string) bool {
	switch strings.ToLower(tag) {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		return true
	default:
		return false
	}
}

func flattenMenu(nodes []menu.Node) []menuItem {
	items := []menuItem{}
	var walk func([]menu.Node, int)
	walk = func(list []menu.Node, depth int) {
		for _, n := range list {
			items = append(items, menuItem{Title: n.Title, Anchor: n.Anchor, Depth: depth})
			if len(n.Children) > 0 {
				walk(n.Children, depth+1)
			}
		}
	}
	walk(nodes, 0)
	return items
}

func parseDocuments(doc *goquery.Document, contentSelector string) (*parse.Document, error) {
	fullDoc, err := parse.Parse(doc)
	if err != nil {
		return nil, err
	}

	contentDoc := doc
	if strings.TrimSpace(contentSelector) != "" {
		extracted, err := parse.ExtractBySelector(doc, contentSelector)
		if err == nil && extracted != nil {
			contentDoc = extracted
		}
	}

	contentParsed, err := parse.Parse(contentDoc)
	if err != nil {
		return nil, err
	}

	if len(contentParsed.Sections) == 0 {
		return fullDoc, nil
	}

	// Keep IDs/anchors from the full page for menu stitching and completeness checks.
	contentParsed.HeadingIDs = fullDoc.HeadingIDs
	contentParsed.AnchorTargets = fullDoc.AnchorTargets
	contentParsed.AllElementIDs = fullDoc.AllElementIDs
	contentParsed.AnchorTargetsByRaw = fullDoc.AnchorTargetsByRaw
	return contentParsed, nil
}

func printSummary(sourceInfo string, doc *parse.Document, rep report.Report) {
	headingIDs := unique(doc.HeadingIDs)
	anchorTargets := unique(doc.AnchorTargets)

	fmt.Printf("Fetch mode: %s\n", sourceInfo)
	fmt.Printf("Sections found: %d\n", len(doc.Sections))

	fmt.Println("Heading IDs:")
	printList(headingIDs)

	fmt.Println("Anchor targets (from href=\"#...\"):")
	printList(anchorTargets)

	if reportHasIssues(rep) {
		fmt.Println("\nCompleteness report:")
		fmt.Printf("  missing heading ids: %d\n", len(rep.MissingHeadingIDs))
		fmt.Printf("  duplicate ids: %d\n", len(rep.DuplicateIDs))
		fmt.Printf("  broken anchors: %d\n", len(rep.BrokenAnchors))
		fmt.Printf("  empty sections: %d\n", len(rep.EmptySections))
		fmt.Printf("  heading gaps: %d\n", len(rep.HeadingGaps))
	}
}

func printList(items []string) {
	if len(items) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, item := range items {
		fmt.Printf("  - %s\n", item)
	}
}

func trimSections(doc *parse.Document, maxSections int) {
	if maxSections > 0 && maxSections < len(doc.Sections) {
		doc.Sections = doc.Sections[:maxSections]
	}
}

func chunkLimits(opts Options) output.ChunkLimits {
	return output.ChunkLimits{
		MaxBytes:  opts.MaxMarkdownBytes,
		MaxChars:  opts.MaxChars,
		MaxTokens: opts.MaxTokens,
	}
}

func applyExclusions(doc *goquery.Document, selector string) {
	if strings.TrimSpace(selector) == "" {
		return
	}
	_ = parse.RemoveSelectors(doc, selector)
}

type sectionMarkdown struct {
	HeadingID  string
	ContentIDs []string
	Markdown   string
}

func buildMarkdown(conv *markdown.Converter, sections []parse.Section) (string, []sectionMarkdown, error) {
	var mdBuilder strings.Builder
	parts := make([]sectionMarkdown, 0, len(sections))
	for _, section := range sections {
		md, err := conv.SectionToMarkdown(section.HeadingText, section.HeadingLevel, section.ContentHTML)
		if err != nil {
			return "", nil, err
		}
		mdBuilder.WriteString(md)
		mdBuilder.WriteString("\n")
		if !strings.HasSuffix(md, "\n") {
			md += "\n"
		}
		parts = append(parts, sectionMarkdown{
			HeadingID:  section.HeadingID,
			ContentIDs: section.ContentIDs,
			Markdown:   md,
		})
	}
	return mdBuilder.String(), parts, nil
}

func writeMenuOutputs(opts Options, baseDoc *goquery.Document, _ *parse.Document, sections []sectionMarkdown) error {
	if strings.TrimSpace(opts.NavSelector) == "" {
		return nil
	}
	nodes, err := menu.Extract(baseDoc, opts.NavSelector)
	if err != nil {
		return fmt.Errorf("menu extract failed (%s): %w", opts.NavSelector, err)
	}
	if err := output.WriteMenu(opts.OutputDir, nodes); err != nil {
		return fmt.Errorf("menu write failed: %w", err)
	}

	mdByID := map[string]string{}
	for _, section := range sections {
		md := section.Markdown
		if opts.DownloadAssets {
			md = strings.ReplaceAll(md, "(assets/", "(../assets/")
			md = strings.ReplaceAll(md, "\"assets/", "\"../assets/")
		}

		if section.HeadingID != "" {
			mdByID[section.HeadingID] = md
		}
		for _, id := range section.ContentIDs {
			if _, ok := mdByID[id]; !ok {
				mdByID[id] = md
			}
		}
	}

	limits := chunkLimits(opts)
	if err := output.WriteSectionFiles(opts.OutputDir, nodes, mdByID, opts.MaxMenuItems, limits); err != nil {
		return fmt.Errorf("section write failed: %w", err)
	}
	return nil
}

func hostFromURL(urlStr string) string {
	if !strings.Contains(urlStr, "://") {
		urlStr = "https://" + urlStr
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	host := u.Hostname()
	host = strings.ReplaceAll(host, ".", "_")
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	host = re.ReplaceAllString(host, "")
	return host
}

func confirm(prompt string) bool {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}

func unique(list []string) []string {
	set := map[string]struct{}{}
	out := []string{}
	for _, v := range list {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := set[v]; ok {
			continue
		}
		set[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func reportHasIssues(rep report.Report) bool {
	return len(rep.MissingHeadingIDs) > 0 ||
		len(rep.DuplicateIDs) > 0 ||
		len(rep.BrokenAnchors) > 0 ||
		len(rep.EmptySections) > 0 ||
		len(rep.HeadingGaps) > 0
}

func documentOuterHTML(doc *goquery.Document) string {
	if doc == nil || doc.Selection == nil {
		return ""
	}
	if html, err := goquery.OuterHtml(doc.Selection); err == nil && strings.TrimSpace(html) != "" {
		return html
	}
	if html, err := doc.Html(); err == nil {
		return html
	}
	return ""
}

func documentIDs(doc *goquery.Document) []string {
	if doc == nil || doc.Selection == nil {
		return nil
	}
	ids := []string{}
	doc.Find("[id]").Each(func(_ int, s *goquery.Selection) {
		if id, exists := s.Attr("id"); exists && id != "" {
			ids = append(ids, id)
		}
	})
	return ids
}
