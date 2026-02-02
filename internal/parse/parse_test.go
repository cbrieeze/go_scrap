package parse_test

import (
	"strings"
	"testing"

	"go_scrap/internal/parse"
)

func TestExtractBySelector(t *testing.T) {
	html := `<div><main id="content"><h2 id="a">A</h2><p>Alpha</p></main></div>`
	out, err := parse.ExtractBySelector(html, "#content")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "<h2") || !strings.Contains(out, "Alpha") {
		t.Fatalf("expected extracted content, got: %s", out)
	}
}

func TestExtractBySelector_NotFound(t *testing.T) {
	_, err := parse.ExtractBySelector("<div></div>", "#missing")
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

	doc, err := parse.Parse(html)
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

	doc, err := parse.Parse(html)
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
