package main

import (
	"bufio"
	"context"

	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go_scrap/internal/app"
	"go_scrap/internal/config"
	"go_scrap/internal/fetch"
	"go_scrap/internal/markdown"
	"go_scrap/internal/menu"
	"go_scrap/internal/output"
	"go_scrap/internal/parse"
	"go_scrap/internal/report"
	"go_scrap/internal/tui"

	"github.com/PuerkitoBio/goquery"
)

type stringFlag struct {
	Value  string
	WasSet bool
}

func (s *stringFlag) String() string { return s.Value }
func (s *stringFlag) Set(v string) error {
	s.Value = v
	s.WasSet = true
	return nil
}

type intFlag struct {
	Value  int
	WasSet bool
}

func (i *intFlag) String() string { return fmt.Sprintf("%d", i.Value) }
func (i *intFlag) Set(v string) error {
	var parsed int
	_, err := fmt.Sscanf(v, "%d", &parsed)
	if err != nil {
		return err
	}
	i.Value = parsed
	i.WasSet = true
	return nil
}

type floatFlag struct {
	Value  float64
	WasSet bool
}

func (f *floatFlag) String() string { return fmt.Sprintf("%g", f.Value) }
func (f *floatFlag) Set(v string) error {
	var parsed float64
	_, err := fmt.Sscanf(v, "%f", &parsed)
	if err != nil {
		return err
	}
	f.Value = parsed
	f.WasSet = true
	return nil
}

type boolFlag struct {
	Value  bool
	WasSet bool
}

func (b *boolFlag) String() string { return fmt.Sprintf("%t", b.Value) }
func (b *boolFlag) Set(v string) error {
	v = strings.ToLower(strings.TrimSpace(v))
	b.Value = v == "true" || v == "1" || v == "yes" || v == "y"
	b.WasSet = true
	return nil
}

func (b *boolFlag) IsBoolFlag() bool { return true }

func main() {
	// Subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "inspect":
			runInspect(os.Args[2:])
			return
		case "test-configs":
			runTestConfigs(os.Args[2:])
			return
		}
	}

	// If no CLI args are provided, launch the TUI and run using its settings.
	if len(os.Args) == 1 {
		res, err := tui.Run()
		if err != nil {
			fatal(err)
		}
		if !res.RunNow {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), res.Options.Timeout)
		defer cancel()
		if err := app.Run(ctx, res.Options); err != nil {
			fatal(err)
		}
		return
	}

	var (
		urlStr             string
		configStr          string
		initConfig         bool
		dryRun             bool
		modeStr            stringFlag
		outputDir          stringFlag
		timeout            intFlag
		userAgent          stringFlag
		waitFor            stringFlag
		headless           boolFlag
		rateLimit          floatFlag
		yes                bool
		strict             bool
		navSel             stringFlag
		contentSel         stringFlag
		navWalk            bool
		stdout             boolFlag
		excludeSel         stringFlag
		maxSections        int
		maxMenuItems       int
		useCache           bool
		downloadAssetsFlag bool
	)

	flag.StringVar(&urlStr, "url", "", "Target URL to scrape")
	flag.StringVar(&configStr, "config", "", "Path to JSON config file")
	flag.BoolVar(&initConfig, "init-config", false, "Interactive config wizard")
	flag.BoolVar(&dryRun, "dry-run", false, "Fetch and analyze only; do not write outputs")
	modeStr.Value = "auto"
	flag.Var(&modeStr, "mode", "Fetch mode: auto|static|dynamic")
	flag.Var(&outputDir, "output-dir", "Output directory (default: output/<host>)")
	timeout.Value = 45
	flag.Var(&timeout, "timeout", "Timeout seconds")
	flag.Var(&userAgent, "user-agent", "User-Agent header")
	flag.Var(&waitFor, "wait-for", "CSS selector to wait for (dynamic mode)")
	headless.Value = true
	flag.Var(&headless, "headless", "Run browser headless (dynamic mode)")
	rateLimit.Value = 0
	flag.Var(&rateLimit, "rate-limit", "Requests per second (0 = off)")
	flag.BoolVar(&yes, "yes", false, "Skip confirmation prompt")
	flag.BoolVar(&strict, "strict", false, "Fail if completeness checks report issues")
	flag.Var(&navSel, "nav-selector", "CSS selector for left menu/navigation")
	flag.Var(&contentSel, "content-selector", "CSS selector for main content container")
	flag.BoolVar(&navWalk, "nav-walk", false, "Click each menu anchor and capture content")
	flag.Var(&stdout, "stdout", "Print Markdown to stdout (implies --yes, suppresses logs)")
	flag.Var(&excludeSel, "exclude-selector", "CSS selector to remove from HTML before processing")
	flag.IntVar(&maxSections, "max-sections", 0, "Limit number of sections written (0 = all)")
	flag.IntVar(&maxMenuItems, "max-menu-items", 0, "Limit number of menu-based section files written (0 = all)")
	flag.BoolVar(&useCache, "cache", false, "Use disk cache for HTML content")
	flag.BoolVar(&downloadAssetsFlag, "download-assets", false, "Download referenced images to local assets directory")
	flag.Parse()

	if initConfig {
		if err := runConfigWizard(); err != nil {
			fatal(err)
		}
		return
	}

	if stdout.Value {
		yes = true
	}

	cfg := config.Config{}
	if configStr != "" {
		loaded, err := config.Load(configStr)
		if err != nil {
			fatal(err)
		}
		cfg = loaded
	}

	if urlStr == "" && cfg.URL != "" {
		urlStr = cfg.URL
	}
	if urlStr == "" {
		fmt.Fprintln(os.Stderr, "--url is required")
		os.Exit(2)
	}

	if !modeStr.WasSet && cfg.Mode != "" {
		modeStr.Value = cfg.Mode
	}
	if !outputDir.WasSet && cfg.OutputDir != "" {
		outputDir.Value = cfg.OutputDir
	}
	if !timeout.WasSet && cfg.TimeoutSeconds > 0 {
		timeout.Value = cfg.TimeoutSeconds
	}
	if !userAgent.WasSet && cfg.UserAgent != "" {
		userAgent.Value = cfg.UserAgent
	}
	if !waitFor.WasSet && cfg.WaitForSelector != "" {
		waitFor.Value = cfg.WaitForSelector
	}
	if !headless.WasSet && cfg.Headless != nil {
		headless.Value = *cfg.Headless
	}
	if !navSel.WasSet && cfg.NavSelector != "" {
		navSel.Value = cfg.NavSelector
	}
	if !contentSel.WasSet && cfg.ContentSelector != "" {
		contentSel.Value = cfg.ContentSelector
	}
	if !navWalk && cfg.NavWalk {
		navWalk = true
	}
	if !rateLimit.WasSet && cfg.RateLimitPerSecond > 0 {
		rateLimit.Value = cfg.RateLimitPerSecond
	}
	if !excludeSel.WasSet && cfg.ExcludeSelector != "" {
		excludeSel.Value = cfg.ExcludeSelector
	}

	mode := fetch.Mode(strings.ToLower(strings.TrimSpace(modeStr.Value)))
	if mode == "" {
		mode = fetch.ModeAuto
	}

	if outputDir.Value == "" {
		if u, err := url.Parse(urlStr); err == nil {
			outputDir.Value = filepath.Join("output", u.Hostname())
		} else {
			outputDir.Value = "output/unknown"
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout.Value)*time.Second)
	defer cancel()

	var result fetch.Result
	var haveResult bool
	var err error
	var cachePath string

	if useCache {
		cachePath = fetch.GetCachePath(urlStr)
		if content, err := os.ReadFile(cachePath); err == nil {
			fmt.Printf("Loaded from cache: %s\n", cachePath)
			result = fetch.Result{
				HTML:       string(content),
				SourceInfo: "cache",
			}
			haveResult = true
		}
	}

	if !haveResult {
		opts := fetch.Options{
			URL:             urlStr,
			Mode:            mode,
			Timeout:         time.Duration(timeout.Value) * time.Second,
			UserAgent:       userAgent.Value,
			WaitForSelector: waitFor.Value,
			Headless:        headless.Value,
		}

		backoffs := []time.Duration{0, time.Second, 2 * time.Second}
		for attempt := 0; attempt < 3; attempt++ {
			if attempt > 0 {
				time.Sleep(backoffs[attempt])
				if !stdout.Value {
					fmt.Fprintf(os.Stderr, "Fetch attempt %d failed. Retrying...\n", attempt)
				}
			}
			result, err = fetch.Fetch(ctx, opts)
			if err == nil || ctx.Err() != nil {
				break
			}
		}

		if err == nil && useCache {
			_ = fetch.SaveToCache(cachePath, result.HTML)
		}
	}
	if err != nil {
		fatal(err)
	}

	if excludeSel.Value != "" {
		cleaned, err := parse.RemoveSelectors(result.HTML, excludeSel.Value)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to exclude selectors: %v\n", err)
		} else {
			result.HTML = cleaned
		}
	}

	if downloadAssetsFlag && !dryRun {
		withAssets, err := output.Download(result.HTML, urlStr, outputDir.Value, userAgent.Value)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: asset processing failed: %v\n", err)
		} else {
			result.HTML = withAssets
		}
	}

	fullDoc, err := parse.Parse(result.HTML)
	if err != nil {
		fatal(err)
	}

	contentHTML := result.HTML
	if contentSel.Value != "" {
		extracted, err := parse.ExtractBySelector(result.HTML, contentSel.Value)
		if err == nil && strings.TrimSpace(extracted) != "" {
			contentHTML = extracted
		}
	}

	contentDoc, err := parse.Parse(contentHTML)
	if err != nil {
		fatal(err)
	}

	doc := contentDoc
	if len(contentDoc.Sections) == 0 {
		doc = fullDoc
	} else {
		doc.HeadingIDs = fullDoc.HeadingIDs
		doc.AnchorTargets = fullDoc.AnchorTargets
		doc.AllElementIDs = fullDoc.AllElementIDs
		doc.AnchorTargetsByRaw = fullDoc.AnchorTargetsByRaw
	}

	if navWalk && strings.TrimSpace(navSel.Value) != "" {
		navDoc, err := buildNavWalkDocument(ctx, result.HTML, urlStr, time.Duration(timeout.Value)*time.Second, waitFor.Value, headless.Value, rateLimit.Value, navSel.Value, contentSel.Value, excludeSel.Value, downloadAssetsFlag && !dryRun, outputDir.Value, userAgent.Value)
		if err != nil {
			fatal(err)
		}
		doc = navDoc
	}

	headingIDs := unique(doc.HeadingIDs)
	anchorTargets := unique(doc.AnchorTargets)

	fmt.Printf("Fetch mode: %s\n", result.SourceInfo)
	fmt.Printf("Sections found: %d\n", len(doc.Sections))
	fmt.Println("Heading IDs:")
	if len(headingIDs) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, id := range headingIDs {
			fmt.Printf("  - %s\n", id)
		}
	}

	fmt.Println("Anchor targets (from href=\"#...\"):")
	if len(anchorTargets) == 0 {
		fmt.Println("  (none)")
	} else {
		for _, a := range anchorTargets {
			fmt.Printf("  - %s\n", a)
		}
	}

	rep := report.Analyze(doc)
	if reportHasIssues(rep) {
		fmt.Println("\nCompleteness report:")
		fmt.Printf("  missing heading ids: %d\n", len(rep.MissingHeadingIDs))
		fmt.Printf("  duplicate ids: %d\n", len(rep.DuplicateIDs))
		fmt.Printf("  broken anchors: %d\n", len(rep.BrokenAnchors))
		fmt.Printf("  empty sections: %d\n", len(rep.EmptySections))
		fmt.Printf("  heading gaps: %d\n", len(rep.HeadingGaps))
	}

	if dryRun {
		fmt.Println("\nDry run complete (no files written).")
		return
	}

	if !yes {
		if !confirm("Continue and generate outputs? [y/N]: ") {
			fmt.Println("Aborted.")
			return
		}
	}

	if maxSections > 0 && maxSections < len(doc.Sections) {
		doc.Sections = doc.Sections[:maxSections]
	}

	conv := markdown.NewConverter()
	var mdBuilder strings.Builder
	for _, section := range doc.Sections {
		md, err := conv.SectionToMarkdown(section.HeadingText, section.HeadingLevel, section.ContentHTML)
		if err != nil {
			fatal(err)
		}
		mdBuilder.WriteString(md)
		mdBuilder.WriteString("\n")
	}

	if strict && reportHasIssues(rep) {
		fatal(errors.New("completeness checks failed (use --strict=false to allow)"))
	}

	mdPath, jsonPath, err := output.WriteAll(doc, rep, mdBuilder.String(), output.WriteOptions{OutputDir: outputDir.Value})
	if err != nil {
		fatal(err)
	}

	if stdout.Value {
		fmt.Println(mdBuilder.String())
	} else {
		fmt.Printf("\nWrote markdown: %s\n", mdPath)
		fmt.Printf("Wrote json: %s\n", jsonPath)
	}

	indexPath, err := output.WriteIndex(outputDir.Value, urlStr, doc.Sections)
	if err == nil && !stdout.Value {
		fmt.Printf("Wrote index: %s\n", indexPath)
	}

	if navSel.Value != "" {
		nodes, err := menu.Extract(result.HTML, navSel.Value)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Menu extract failed: %v\n", err)
			return
		}
		if err := output.WriteMenu(outputDir.Value, nodes); err != nil {
			fmt.Fprintf(os.Stderr, "Menu write failed: %v\n", err)
			return
		}

		mdByID := make(map[string]string)
		for _, section := range doc.Sections {
			md, err := conv.SectionToMarkdown(section.HeadingText, section.HeadingLevel, section.ContentHTML)
			if err != nil {
				continue
			}

			if section.HeadingID != "" {
				mdByID[section.HeadingID] = md
			}

			if downloadAssetsFlag {
				md = strings.ReplaceAll(md, "(assets/", "(../assets/")
				md = strings.ReplaceAll(md, "\"assets/", "\"../assets/")
			}

			// Map any internal IDs to this section so anchor-only menu links work
			secDoc, err := goquery.NewDocumentFromReader(strings.NewReader(section.ContentHTML))
			if err == nil {
				secDoc.Find("[id]").Each(func(_ int, s *goquery.Selection) {
					if id, exists := s.Attr("id"); exists && id != "" {
						if _, ok := mdByID[id]; !ok {
							mdByID[id] = md
						}
					}
				})
			}
		}
		if err := output.WriteSectionFiles(outputDir.Value, nodes, mdByID, maxMenuItems); err != nil {
			fmt.Fprintf(os.Stderr, "Section write failed: %v\n", err)
			return
		}
	}
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
	return out
}

func reportHasIssues(rep report.Report) bool {
	return len(rep.MissingHeadingIDs) > 0 ||
		len(rep.DuplicateIDs) > 0 ||
		len(rep.BrokenAnchors) > 0 ||
		len(rep.EmptySections) > 0 ||
		len(rep.HeadingGaps) > 0
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "Error:", err)
	os.Exit(1)
}

func runConfigWizard() error {
	reader := bufio.NewReader(os.Stdin)
	fmt.Println("Config wizard (press Enter to accept defaults)")

	path := promptString(reader, "Config file path", "config.json")
	mode := promptString(reader, "Mode (auto|static|dynamic)", "dynamic")
	timeout := promptInt(reader, "Timeout seconds", 60)
	waitFor := promptString(reader, "Wait for selector", "body")
	headless := promptBool(reader, "Headless (true/false)", true)
	navSel := promptString(reader, "Nav selector (optional)", "")
	contentSel := promptString(reader, "Content selector (optional)", "")

	cfg := config.Config{
		Mode:            mode,
		TimeoutSeconds:  timeout,
		WaitForSelector: waitFor,
		Headless:        &headless,
		NavSelector:     navSel,
		ContentSelector: contentSel,
	}

	data, err := config.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}

	fmt.Printf("Wrote %s\n", path)
	return nil
}

func promptString(reader *bufio.Reader, label, def string) string {
	if def != "" {
		fmt.Printf("%s [%s]: ", label, def)
	} else {
		fmt.Printf("%s: ", label)
	}
	line, err := reader.ReadString('\n')
	if err != nil {
		return def
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func promptInt(reader *bufio.Reader, label string, def int) int {
	fmt.Printf("%s [%d]: ", label, def)
	line, err := reader.ReadString('\n')
	if err != nil {
		return def
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	var val int
	_, err = fmt.Sscanf(line, "%d", &val)
	if err != nil {
		return def
	}
	return val
}

func promptBool(reader *bufio.Reader, label string, def bool) bool {
	defStr := "false"
	if def {
		defStr = "true"
	}
	fmt.Printf("%s [%s]: ", label, defStr)
	line, err := reader.ReadString('\n')
	if err != nil {
		return def
	}
	line = strings.TrimSpace(strings.ToLower(line))
	if line == "" {
		return def
	}
	return line == "true" || line == "1" || line == "yes" || line == "y"
}

type navMenuItem struct {
	Title  string
	Anchor string
	Depth  int
}

func buildNavWalkDocument(ctx context.Context, baseHTML, urlStr string, timeout time.Duration, waitFor string, headless bool, rateLimit float64, navSelector, contentSelector, excludeSelector string, downloadAssets bool, outputDir, userAgent string) (*parse.Document, error) {
	nodes, err := menu.Extract(baseHTML, navSelector)
	if err != nil {
		return nil, err
	}
	items := flattenMenuNodes(nodes)
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
		URL:                urlStr,
		Mode:               fetch.ModeDynamic,
		Timeout:            timeout,
		UserAgent:          userAgent,
		WaitForSelector:    waitFor,
		Headless:           headless,
		RateLimitPerSecond: rateLimit,
	}, anchors)
	if err != nil {
		return nil, err
	}

	sections := []parse.Section{}
	ids := []string{}
	for _, item := range items {
		if item.Anchor == "" {
			continue
		}
		htmlForAnchor, ok := htmlByAnchor[item.Anchor]
		if !ok {
			continue
		}
		contentHTML := htmlForAnchor
		if strings.TrimSpace(contentSelector) != "" {
			extracted, err := parse.ExtractBySelector(htmlForAnchor, contentSelector)
			if err == nil && strings.TrimSpace(extracted) != "" {
				contentHTML = extracted
			}
		}
		if strings.TrimSpace(excludeSelector) != "" {
			cleaned, err := parse.RemoveSelectors(contentHTML, excludeSelector)
			if err == nil {
				contentHTML = cleaned
			}
		}
		if downloadAssets {
			if withAssets, err := output.Download(contentHTML, urlStr, outputDir, userAgent); err == nil {
				contentHTML = withAssets
			}
		}
		heading := strings.TrimSpace(item.Title)
		if heading == "" {
			heading = item.Anchor
		}
		level := 2 + item.Depth
		if level > 6 {
			level = 6
		}
		sections = append(sections, parse.Section{
			HeadingText:   heading,
			HeadingLevel:  level,
			HeadingID:     item.Anchor,
			ContentHTML:   contentHTML,
			ContentText:   strings.TrimSpace(parse.StripTags(contentHTML)),
			AnchorTargets: anchors,
		})
		ids = append(ids, item.Anchor)
	}

	return &parse.Document{
		HTML:               baseHTML,
		Sections:           sections,
		HeadingIDs:         ids,
		AnchorTargets:      anchors,
		AllElementIDs:      ids,
		AnchorTargetsByRaw: anchors,
	}, nil
}

func flattenMenuNodes(nodes []menu.Node) []navMenuItem {
	items := []navMenuItem{}
	var walk func([]menu.Node, int)
	walk = func(list []menu.Node, depth int) {
		for _, n := range list {
			items = append(items, navMenuItem{Title: n.Title, Anchor: n.Anchor, Depth: depth})
			if len(n.Children) > 0 {
				walk(n.Children, depth+1)
			}
		}
	}
	walk(nodes, 0)
	return items
}
