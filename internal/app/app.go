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
	NavSelector        string
	ContentSelector    string
	NavWalk            bool
	MaxSections        int
	MaxMenuItems       int
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

	result, err := fetchResult(ctx, normalized)
	if err != nil {
		return err
	}

	var doc *parse.Document
	if normalized.NavWalk && strings.TrimSpace(normalized.NavSelector) != "" {
		doc, err = runNavWalk(ctx, normalized, result.HTML)
		if err != nil {
			return err
		}
	} else {
		doc, err = parseDocuments(result.HTML, normalized.ContentSelector)
		if err != nil {
			return err
		}
	}

	rep := report.Analyze(doc)
	printSummary(result.SourceInfo, doc, rep)

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
	md, err := buildMarkdown(conv, doc.Sections)
	if err != nil {
		return err
	}

	if normalized.Strict && reportHasIssues(rep) {
		return errors.New("completeness checks failed (use --strict=false to allow)")
	}

	mdPath, jsonPath, err := output.WriteAll(doc, rep, md, output.WriteOptions{OutputDir: normalized.OutputDir})
	if err != nil {
		return err
	}

	fmt.Printf("\nWrote markdown: %s\n", mdPath)
	fmt.Printf("Wrote json: %s\n", jsonPath)

	if err := writeMenuOutputs(normalized, result.HTML, doc, conv); err != nil {
		return err
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
	return opts, nil
}

func fetchResult(ctx context.Context, opts Options) (fetch.Result, error) {
	mode := opts.Mode
	if opts.NavWalk {
		mode = fetch.ModeDynamic
	}
	return fetch.Fetch(ctx, fetch.Options{
		URL:                opts.URL,
		Mode:               mode,
		Timeout:            opts.Timeout,
		UserAgent:          opts.UserAgent,
		WaitForSelector:    opts.WaitFor,
		Headless:           opts.Headless,
		RateLimitPerSecond: opts.RateLimitPerSecond,
	})
}

func runNavWalk(ctx context.Context, opts Options, htmlText string) (*parse.Document, error) {
	nodes, err := menu.Extract(htmlText, opts.NavSelector)
	if err != nil {
		return nil, fmt.Errorf("menu extract failed (%s): %w", opts.NavSelector, err)
	}
	items := flattenMenu(nodes)
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
		return nil, err
	}

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
		contentHTML := htmlForAnchor
		if strings.TrimSpace(opts.ContentSelector) != "" {
			extracted, err := parse.ExtractBySelector(htmlForAnchor, opts.ContentSelector)
			if err == nil && strings.TrimSpace(extracted) != "" {
				contentHTML = extracted
			}
		}
		text := strings.TrimSpace(parse.StripTags(contentHTML))
		level := 2 + item.Depth
		if level > 6 {
			level = 6
		}
		section := parse.Section{
			HeadingText:   strings.TrimSpace(item.Title),
			HeadingLevel:  level,
			HeadingID:     item.Anchor,
			ContentHTML:   contentHTML,
			ContentText:   text,
			AnchorTargets: anchors,
		}
		sections = append(sections, section)
		headings = append(headings, item.Anchor)
	}

	return &parse.Document{
		HTML:               htmlText,
		Sections:           sections,
		HeadingIDs:         headings,
		AnchorTargets:      anchors,
		AllElementIDs:      headings,
		AnchorTargetsByRaw: anchors,
	}, nil
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

func parseDocuments(htmlText, contentSelector string) (*parse.Document, error) {
	fullDoc, err := parse.Parse(htmlText)
	if err != nil {
		return nil, err
	}

	contentHTML := htmlText
	if strings.TrimSpace(contentSelector) != "" {
		extracted, err := parse.ExtractBySelector(htmlText, contentSelector)
		if err != nil {
			return nil, fmt.Errorf("content selector failed (%s): %w", contentSelector, err)
		}
		if strings.TrimSpace(extracted) != "" {
			contentHTML = extracted
		}
	}

	contentDoc, err := parse.Parse(contentHTML)
	if err != nil {
		return nil, err
	}

	if len(contentDoc.Sections) == 0 {
		return fullDoc, nil
	}

	// Keep IDs/anchors from the full page for menu stitching and completeness checks.
	contentDoc.HeadingIDs = fullDoc.HeadingIDs
	contentDoc.AnchorTargets = fullDoc.AnchorTargets
	contentDoc.AllElementIDs = fullDoc.AllElementIDs
	contentDoc.AnchorTargetsByRaw = fullDoc.AnchorTargetsByRaw
	return contentDoc, nil
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

func buildMarkdown(conv *markdown.Converter, sections []parse.Section) (string, error) {
	var mdBuilder strings.Builder
	for _, section := range sections {
		md, err := conv.SectionToMarkdown(section.HeadingText, section.HeadingLevel, section.ContentHTML)
		if err != nil {
			return "", err
		}
		mdBuilder.WriteString(md)
		mdBuilder.WriteString("\n")
	}
	return mdBuilder.String(), nil
}

func writeMenuOutputs(opts Options, htmlText string, doc *parse.Document, conv *markdown.Converter) error {
	if strings.TrimSpace(opts.NavSelector) == "" {
		return nil
	}
	nodes, err := menu.Extract(htmlText, opts.NavSelector)
	if err != nil {
		return fmt.Errorf("menu extract failed (%s): %w", opts.NavSelector, err)
	}
	if err := output.WriteMenu(opts.OutputDir, nodes); err != nil {
		return fmt.Errorf("menu write failed: %w", err)
	}

	mdByID := map[string]string{}
	for _, section := range doc.Sections {
		if section.HeadingID == "" {
			continue
		}
		md, err := conv.SectionToMarkdown(section.HeadingText, section.HeadingLevel, section.ContentHTML)
		if err != nil {
			continue
		}
		mdByID[section.HeadingID] = md
	}

	if err := output.WriteSectionFiles(opts.OutputDir, nodes, mdByID, opts.MaxMenuItems); err != nil {
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
