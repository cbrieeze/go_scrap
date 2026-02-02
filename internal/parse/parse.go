package parse

import (
	"bytes"
	"errors"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

type Section struct {
	HeadingText   string   `json:"heading_text"`
	HeadingHTML   string   `json:"heading_html"`
	HeadingLevel  int      `json:"heading_level"`
	HeadingID     string   `json:"heading_id"`
	ContentHTML   string   `json:"content_html"`
	ContentText   string   `json:"content_text"`
	AnchorTargets []string `json:"anchor_targets"`
}

type Document struct {
	HTML               string
	Sections           []Section
	HeadingIDs         []string
	AnchorTargets      []string
	AllElementIDs      []string
	AnchorTargetsByRaw []string
}

type headingInfo struct {
	Node *html.Node
	ID   string
}

func ExtractBySelector(htmlText, selector string) (string, error) {
	if strings.TrimSpace(selector) == "" {
		return htmlText, nil
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		return "", err
	}
	sel := doc.Find(selector).First()
	if sel.Length() == 0 {
		return "", errors.New("selector not found: " + selector)
	}
	node := sel.Get(0)
	if node == nil {
		return "", errors.New("selector node missing: " + selector)
	}
	var buf bytes.Buffer
	if err := html.Render(&buf, node); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func StripTags(htmlText string) string {
	return stripTags(htmlText)
}

func Parse(htmlText string) (*Document, error) {
	if strings.TrimSpace(htmlText) == "" {
		return nil, errors.New("empty html")
	}
	node, err := html.Parse(strings.NewReader(htmlText))
	if err != nil {
		return nil, err
	}
	query := goquery.NewDocumentFromNode(node)

	headings := []headingInfo{}
	query.Find("h1, h2, h3, h4, h5, h6").Each(func(_ int, s *goquery.Selection) {
		n := s.Get(0)
		if n == nil {
			return
		}
		id := ""
		if val, exists := s.Attr("id"); exists && val != "" {
			id = val
		} else {
			child := s.Find("[id]").First()
			if child.Length() > 0 {
				if val, exists := child.Attr("id"); exists && val != "" {
					id = val
				}
			}
		}
		headings = append(headings, headingInfo{Node: n, ID: id})
	})

	allIDs := []string{}
	query.Find("[id]").Each(func(_ int, s *goquery.Selection) {
		if id, exists := s.Attr("id"); exists && id != "" {
			allIDs = append(allIDs, id)
		}
	})

	anchorsRaw := []string{}
	anchors := []string{}
	query.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		if strings.HasPrefix(href, "#") && len(href) > 1 {
			anchorsRaw = append(anchorsRaw, href)
			anchors = append(anchors, strings.TrimPrefix(href, "#"))
		}
	})

	sections := make([]Section, 0, len(headings))
	headingIDSet := map[string]struct{}{}
	for _, h := range headings {
		next := findNextHeading(h.Node)
		contentHTML := collectContentBetween(h.Node, next)
		headingText := strings.TrimSpace(extractText(h.Node))
		headingID := h.ID
		if headingID == "" {
			headingID = slugifyHeading(headingText)
		}
		if headingID != "" {
			if _, exists := headingIDSet[headingID]; !exists {
				headingIDSet[headingID] = struct{}{}
			}
		}

		section := Section{
			HeadingText:   headingText,
			HeadingHTML:   renderHTML(h.Node),
			HeadingLevel:  headingLevel(h.Node),
			HeadingID:     headingID,
			ContentHTML:   contentHTML,
			ContentText:   strings.TrimSpace(stripTags(contentHTML)),
			AnchorTargets: anchors,
		}
		sections = append(sections, section)
	}

	headingIDs := make([]string, 0, len(headingIDSet))
	for id := range headingIDSet {
		headingIDs = append(headingIDs, id)
	}

	return &Document{
		HTML:               htmlText,
		Sections:           sections,
		HeadingIDs:         headingIDs,
		AnchorTargets:      anchors,
		AllElementIDs:      allIDs,
		AnchorTargetsByRaw: anchorsRaw,
	}, nil
}

func slugifyHeading(text string) string {
	text = strings.TrimSpace(strings.ToLower(text))
	if text == "" {
		return ""
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteRune('_')
			lastUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_")
	return out
}

func isHeadingNode(n *html.Node) bool {
	if n.Type != html.ElementNode {
		return false
	}
	switch n.Data {
	case "h1", "h2", "h3", "h4", "h5", "h6":
		return true
	default:
		return false
	}
}

func headingLevel(n *html.Node) int {
	if n.Type != html.ElementNode {
		return 0
	}
	switch n.Data {
	case "h1":
		return 1
	case "h2":
		return 2
	case "h3":
		return 3
	case "h4":
		return 4
	case "h5":
		return 5
	case "h6":
		return 6
	default:
		return 0
	}
}

func findNextHeading(start *html.Node) *html.Node {
	for n := nextNode(start); n != nil; n = nextNode(n) {
		if isHeadingNode(n) {
			return n
		}
	}
	return nil
}

func collectContentBetween(start, end *html.Node) string {
	var buf strings.Builder
	for n := nextNodeAfterSubtree(start); n != nil && n != end; n = nextNodeAfterSubtree(n) {
		if isHeadingNode(n) {
			break
		}
		if shouldSkipNode(n) {
			continue
		}
		buf.WriteString(renderHTML(n))
	}
	return buf.String()
}

func shouldSkipNode(n *html.Node) bool {
	if n.Type == html.ElementNode {
		switch n.Data {
		case "script", "style", "noscript":
			return true
		}
	}
	return false
}

func nextNode(n *html.Node) *html.Node {
	if n.FirstChild != nil {
		return n.FirstChild
	}
	for n != nil {
		if n.NextSibling != nil {
			return n.NextSibling
		}
		n = n.Parent
	}
	return nil
}

func nextNodeAfterSubtree(n *html.Node) *html.Node {
	if n == nil {
		return nil
	}
	if n.NextSibling != nil {
		return n.NextSibling
	}
	for n.Parent != nil {
		n = n.Parent
		if n.NextSibling != nil {
			return n.NextSibling
		}
	}
	return nil
}

func renderHTML(n *html.Node) string {
	var buf bytes.Buffer
	_ = html.Render(&buf, n)
	return buf.String()
}

func extractText(n *html.Node) string {
	if n == nil {
		return ""
	}
	var buf strings.Builder
	var walkText func(*html.Node)
	walkText = func(node *html.Node) {
		if node.Type == html.TextNode {
			buf.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walkText(c)
		}
	}
	walkText(n)
	return buf.String()
}

func stripTags(htmlText string) string {
	node, err := html.Parse(strings.NewReader(htmlText))
	if err != nil {
		return htmlText
	}
	return strings.TrimSpace(extractText(node))
}

func getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}
