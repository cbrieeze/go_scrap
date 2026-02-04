package app

import (
	"context"
	"fmt"
	"strings"

	"go_scrap/internal/fetch"
	"go_scrap/internal/menu"
	"go_scrap/internal/output"
	"go_scrap/internal/parse"

	"github.com/PuerkitoBio/goquery"
)

type menuItem struct {
	Title  string
	Anchor string
	Depth  int
}

func buildDocument(ctx context.Context, opts Options, baseDoc *goquery.Document) (*parse.Document, error) {
	if opts.NavWalk && strings.TrimSpace(opts.NavSelector) != "" {
		return runNavWalk(ctx, opts, baseDoc)
	}
	return parseDocuments(baseDoc, opts.ContentSelector)
}

func runNavWalk(ctx context.Context, opts Options, baseDoc *goquery.Document) (*parse.Document, error) {
	nodes, err := menu.Extract(baseDoc, opts.NavSelector)
	if err != nil {
		return nil, fmt.Errorf("menu extract failed (%s): %w", opts.NavSelector, err)
	}
	items := flattenMenu(nodes)
	anchors := collectAnchors(items)

	htmlByAnchor, err := fetch.AnchorHTML(ctx, buildFetchOptions(opts, fetch.ModeDynamic), anchors)
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

	contentParsed.HeadingIDs = fullDoc.HeadingIDs
	contentParsed.AnchorTargets = fullDoc.AnchorTargets
	contentParsed.AllElementIDs = fullDoc.AllElementIDs
	contentParsed.AnchorTargetsByRaw = fullDoc.AnchorTargetsByRaw
	return contentParsed, nil
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
