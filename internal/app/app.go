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

	baseDoc, result, err := prepareBaseDocument(ctx, normalized)
	if err != nil {
		return err
	}

	doc, err := buildDocument(ctx, normalized, baseDoc)
	if err != nil {
		return err
	}

	rep := report.Analyze(doc)
	printSummaryIfNeeded(normalized, result.SourceInfo, doc, rep)

	if normalized.DryRun {
		fmt.Println("\nDry run complete (no files written).")
		return nil
	}

	if !normalized.Yes && !confirm("Continue and generate outputs? [y/N]: ") {
		fmt.Println("Aborted.")
		return nil
	}

	trimSections(doc, normalized.MaxSections)

	conv := markdown.NewConverter()
	return writeOutputs(normalized, baseDoc, doc, rep, conv)
}

func prepareBaseDocument(ctx context.Context, opts Options) (*goquery.Document, fetch.Result, error) {
	result, err := fetchResult(ctx, opts)
	if err != nil {
		return nil, fetch.Result{}, err
	}

	baseDoc, err := parse.NewDocument(result.HTML)
	if err != nil {
		return nil, fetch.Result{}, err
	}

	if strings.TrimSpace(opts.ExcludeSelector) != "" {
		_ = parse.RemoveSelectors(baseDoc, opts.ExcludeSelector)
	}

	if opts.DownloadAssets && !opts.DryRun {
		if err := output.Download(baseDoc, opts.URL, opts.OutputDir, opts.UserAgent); err != nil && !opts.Stdout {
			fmt.Fprintf(os.Stderr, "Warning: asset processing failed: %v\n", err)
		}
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

func writeOutputs(opts Options, baseDoc *goquery.Document, doc *parse.Document, rep report.Report, conv *markdown.Converter) error {
	md, sectionMarkdowns, err := buildMarkdown(conv, doc.Sections)
	if err != nil {
		return err
	}

	if opts.Strict && reportHasIssues(rep) {
		return errors.New("completeness checks failed (use --strict=false to allow)")
	}

	jsonPath, err := output.WriteJSON(doc, rep, output.WriteOptions{OutputDir: opts.OutputDir})
	if err != nil {
		return err
	}

	var mdPath string
	limits := output.ChunkLimits{
		MaxBytes:  opts.MaxMarkdownBytes,
		MaxChars:  opts.MaxChars,
		MaxTokens: opts.MaxTokens,
	}
	if limits.Enabled() {
		mdPath, err = output.WriteMarkdownParts(opts.OutputDir, "content.md", sectionMarkdowns, limits)
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

	if err := writeMenuOutputs(opts, baseDoc, doc, conv); err != nil {
		return err
	}

	if !opts.Stdout {
		if indexPath, err := output.WriteIndex(opts.OutputDir, opts.URL, doc.Sections); err == nil {
			fmt.Printf("Wrote index: %s\n", indexPath)
		}
	}

	return nil
}

func normalizeOptions(opts Options) (Options, error) {
	if strings.TrimSpace(opts.URL) == "" {
		return opts, errors.New("url is required")
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
		host := hostFromURL(opts.URL)
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

	htmlByAnchor, err := fetch.FetchAnchorHTML(ctx, fetch.Options{
		URL:                opts.URL,
		Mode:               fetch.ModeDynamic,
		Timeout:            opts.Timeout,
		UserAgent:          opts.UserAgent,
		WaitForSelector:    opts.WaitFor,
		Headless:           opts.Headless,
		RateLimitPerSecond: opts.RateLimitPerSecond,
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
	if strings.TrimSpace(opts.ExcludeSelector) != "" {
		_ = parse.RemoveSelectors(anchorDoc, opts.ExcludeSelector)
	}
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

func trimSections(doc *parse.Document, max int) {
	if max > 0 && max < len(doc.Sections) {
		doc.Sections = doc.Sections[:max]
	}
}

func buildMarkdown(conv *markdown.Converter, sections []parse.Section) (string, []string, error) {
	var mdBuilder strings.Builder
	parts := make([]string, 0, len(sections))
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
		parts = append(parts, md)
	}
	return mdBuilder.String(), parts, nil
}

func writeMenuOutputs(opts Options, baseDoc *goquery.Document, doc *parse.Document, conv *markdown.Converter) error {
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
	for _, section := range doc.Sections {
		md, err := conv.SectionToMarkdown(section.HeadingText, section.HeadingLevel, section.ContentHTML)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to convert section %q: %v\n", section.HeadingText, err)
			continue
		}

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

	limits := output.ChunkLimits{
		MaxBytes:  opts.MaxMarkdownBytes,
		MaxChars:  opts.MaxChars,
		MaxTokens: opts.MaxTokens,
	}
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
