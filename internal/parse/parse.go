package parse

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Section struct {
	HeadingText   string   `json:"heading_text"`
	HeadingHTML   string   `json:"heading_html"`
	HeadingLevel  int      `json:"heading_level"`
	HeadingID     string   `json:"heading_id"`
	ContentHTML   string   `json:"content_html"`
	ContentText   string   `json:"content_text"`
	AnchorTargets []string `json:"anchor_targets"`
	ContentIDs    []string `json:"-"`
}

type Document struct {
	HTML               string
	Sections           []Section
	HeadingIDs         []string
	AnchorTargets      []string
	AllElementIDs      []string
	AnchorTargetsByRaw []string
}

func NewDocument(htmlText string) (*goquery.Document, error) {
	if strings.TrimSpace(htmlText) == "" {
		return nil, errors.New("empty html")
	}
	return goquery.NewDocumentFromReader(strings.NewReader(htmlText))
}

func ExtractBySelector(doc *goquery.Document, selector string) (*goquery.Document, error) {
	if doc == nil {
		return nil, errors.New("nil document")
	}
	if strings.TrimSpace(selector) == "" {
		return doc, nil
	}
	sel := doc.Find(selector).First()
	if sel.Length() == 0 {
		return nil, errors.New("selector not found: " + selector)
	}
	node := sel.Get(0)
	if node == nil {
		return nil, errors.New("selector node missing: " + selector)
	}
	return goquery.NewDocumentFromNode(node), nil
}

func Parse(doc *goquery.Document) (*Document, error) {
	if doc == nil {
		return nil, errors.New("nil document")
	}

	allIDs := []string{}
	doc.Find("[id]").Each(func(_ int, s *goquery.Selection) {
		if id, exists := s.Attr("id"); exists && id != "" {
			allIDs = append(allIDs, id)
		}
	})

	anchorsRaw := []string{}
	anchors := []string{}
	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		if strings.HasPrefix(href, "#") && len(href) > 1 {
			anchorsRaw = append(anchorsRaw, href)
			anchors = append(anchors, strings.TrimPrefix(href, "#"))
		}
	})

	sections := []Section{}
	headingIDSet := map[string]struct{}{}

	doc.Find("h1, h2, h3, h4, h5, h6").Each(func(_ int, s *goquery.Selection) {
		// 1. Resolve Heading ID
		headingID := s.AttrOr("id", "")
		if headingID == "" {
			if childID := s.Find("[id]").First().AttrOr("id", ""); childID != "" {
				headingID = childID
			}
		}

		// 2. Extract Content (siblings until next heading)
		contentSel := s.NextUntil("h1, h2, h3, h4, h5, h6")

		// Handle nested headings (e.g. <div><h2>...</h2></div> <p>Content</p>)
		// If the heading is the last element in its parent, the content might be after the parent.
		if contentSel.Length() == 0 && s.Next().Length() == 0 {
			parent := s.Parent()
			if !parent.Is("body, html") {
				contentSel = parent.NextUntil("h1, h2, h3, h4, h5, h6, :has(h1, h2, h3, h4, h5, h6)")
			}
		}

		contentHTML, contentText, contentIDs := renderSelection(contentSel)

		// 3. Extract Heading Text
		headingText := strings.TrimSpace(s.Text())
		// 4. Generate Slug if needed
		if headingID == "" {
			headingID = slugifyHeading(headingText)
		}
		// 5. Handle ID collisions by appending counter suffix
		headingID = deduplicateID(headingID, headingIDSet)

		headingHTML, _ := goquery.OuterHtml(s)

		section := Section{
			HeadingText:   headingText,
			HeadingHTML:   headingHTML,
			HeadingLevel:  headingLevelFromTag(goquery.NodeName(s)),
			HeadingID:     headingID,
			ContentHTML:   contentHTML,
			ContentText:   strings.TrimSpace(contentText),
			AnchorTargets: anchors,
			ContentIDs:    contentIDs,
		}
		sections = append(sections, section)
	})

	headingIDs := make([]string, 0, len(headingIDSet))
	for id := range headingIDSet {
		headingIDs = append(headingIDs, id)
	}

	htmlText, _ := doc.Html()
	return &Document{
		HTML:               htmlText,
		Sections:           sections,
		HeadingIDs:         headingIDs,
		AnchorTargets:      anchors,
		AllElementIDs:      allIDs,
		AnchorTargetsByRaw: anchorsRaw,
	}, nil
}

var slugRegexp = regexp.MustCompile(`[^a-z0-9]+`)

func slugifyHeading(text string) string {
	text = strings.TrimSpace(strings.ToLower(text))
	return strings.Trim(slugRegexp.ReplaceAllString(text, "_"), "_")
}

func deduplicateID(id string, seen map[string]struct{}) string {
	if id == "" {
		return ""
	}
	if _, exists := seen[id]; !exists {
		seen[id] = struct{}{}
		return id
	}
	counter := 2
	for {
		newID := id + "_" + strconv.Itoa(counter)
		if _, exists := seen[newID]; !exists {
			seen[newID] = struct{}{}
			return newID
		}
		counter++
	}
}

func headingLevelFromTag(tag string) int {
	switch strings.ToLower(tag) {
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

func renderSelection(sel *goquery.Selection) (string, string, []string) {
	var htmlBuf strings.Builder
	var textBuf strings.Builder
	ids := []string{}

	sel.Each(func(_ int, s *goquery.Selection) {
		if s.Is("script, style, noscript") {
			return
		}

		// Render HTML
		h, _ := goquery.OuterHtml(s)
		htmlBuf.WriteString(h)

		// Render Text
		textBuf.WriteString(s.Text())
		textBuf.WriteString(" ") // Ensure separation between block elements

		s.Find("[id]").Each(func(_ int, node *goquery.Selection) {
			if id, ok := node.Attr("id"); ok && id != "" {
				ids = append(ids, id)
			}
		})
	})

	return htmlBuf.String(), textBuf.String(), ids
}
