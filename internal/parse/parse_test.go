package parse_test

import (
	"strings"
	"testing"

	"go_scrap/internal/parse"
)

func TestExtractBySelector(t *testing.T) {
	html := `<div><main id="content"><h2 id="a">A</h2><p>Alpha</p></main></div>`
	doc, err := parse.NewDocument(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out, err := parse.ExtractBySelector(doc, "#content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	htmlOut, _ := out.Html()
	if !strings.Contains(htmlOut, "<h2") || !strings.Contains(htmlOut, "Alpha") {
		t.Fatalf("expected extracted content, got: %s", htmlOut)
	}
}

func TestExtractBySelector_NotFound(t *testing.T) {
	doc, err := parse.NewDocument("<div></div>")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = parse.ExtractBySelector(doc, "#missing")
	if err == nil {
		t.Fatal("expected error for missing selector")
	}
}

func TestParseSectionsAndSkipScripts(t *testing.T) {
	html := `
	<body>
	  <h1 id="intro">Intro</h1>
	  <p>Hello</p>
	  <script>ignore()</script>
	  <h2 id="next">Next</h2>
	  <p>World</p>
	</body>`

	htmlDoc, err := parse.NewDocument(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	doc, err := parse.Parse(htmlDoc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(doc.Sections))
	}
	if doc.Sections[0].HeadingID != "intro" {
		t.Fatalf("expected intro id, got %q", doc.Sections[0].HeadingID)
	}
	if strings.Contains(doc.Sections[0].ContentHTML, "script") {
		t.Fatalf("script should be skipped, got %s", doc.Sections[0].ContentHTML)
	}
	if !strings.Contains(doc.Sections[0].ContentText, "Hello") {
		t.Fatalf("expected Hello in content text, got %q", doc.Sections[0].ContentText)
	}
}

func TestParseAnchorsAndIDs(t *testing.T) {
	html := `
	<body>
	  <h2 id="h2">Title</h2>
	  <a href="#h2">link</a>
	  <a href="#missing">broken</a>
	</body>`

	htmlDoc, err := parse.NewDocument(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	doc, err := parse.Parse(htmlDoc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.HeadingIDs) != 1 || doc.HeadingIDs[0] != "h2" {
		t.Fatalf("expected heading id h2, got %v", doc.HeadingIDs)
	}
	if len(doc.AnchorTargets) != 2 {
		t.Fatalf("expected 2 anchor targets, got %v", doc.AnchorTargets)
	}
}

func TestParse_NestedHeading(t *testing.T) {
	html := `
	<body>
	  <div><h2 id="nested">Nested</h2></div>
	  <p>Content</p>
	  <h2 id="next">Next</h2>
	</body>`

	htmlDoc, err := parse.NewDocument(html)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	doc, err := parse.Parse(htmlDoc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Sections) != 2 {
		t.Fatalf("expected 2 sections, got %d", len(doc.Sections))
	}
	if doc.Sections[0].HeadingID != "nested" {
		t.Fatalf("expected first section to be 'nested', got %q", doc.Sections[0].HeadingID)
	}
	if !strings.Contains(doc.Sections[0].ContentText, "Content") {
		t.Fatalf("expected content 'Content' in first section, got %q", doc.Sections[0].ContentText)
	}
	if doc.Sections[1].HeadingID != "next" {
		t.Fatalf("expected second section to be 'next', got %q", doc.Sections[1].HeadingID)
	}
}

func TestParse_SlugifiesHeadingsWithoutIDs(t *testing.T) {
	docHTML, err := parse.NewDocument(`<body><h2>My Heading!</h2><p>x</p></body>`)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	doc, err := parse.Parse(docHTML)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	if len(doc.Sections) != 1 {
		t.Fatalf("expected 1 section, got %d", len(doc.Sections))
	}
	if doc.Sections[0].HeadingID != "my_heading" {
		t.Fatalf("unexpected heading id: %q", doc.Sections[0].HeadingID)
	}
}

func TestParse_HeadingIDCollisionPrevention(t *testing.T) {
	// Test that duplicate heading text gets unique IDs
	html := `<body>
		<h2>Introduction</h2><p>First intro</p>
		<h2>Introduction</h2><p>Second intro</p>
		<h2>Introduction</h2><p>Third intro</p>
	</body>`

	docHTML, err := parse.NewDocument(html)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	doc, err := parse.Parse(docHTML)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	if len(doc.Sections) != 3 {
		t.Fatalf("expected 3 sections, got %d", len(doc.Sections))
	}

	// Verify all IDs are unique
	seen := make(map[string]bool)
	for i, sec := range doc.Sections {
		if seen[sec.HeadingID] {
			t.Errorf("section %d has duplicate ID: %q", i, sec.HeadingID)
		}
		seen[sec.HeadingID] = true
	}

	// Verify expected ID pattern
	if doc.Sections[0].HeadingID != "introduction" {
		t.Errorf("expected first ID 'introduction', got %q", doc.Sections[0].HeadingID)
	}
	if doc.Sections[1].HeadingID != "introduction_2" {
		t.Errorf("expected second ID 'introduction_2', got %q", doc.Sections[1].HeadingID)
	}
	if doc.Sections[2].HeadingID != "introduction_3" {
		t.Errorf("expected third ID 'introduction_3', got %q", doc.Sections[2].HeadingID)
	}
}
