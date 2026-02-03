package app

import (
	"strings"
	"testing"

	"go_scrap/internal/menu"
	"go_scrap/internal/parse"
)

func TestPrepareContentDoc_SlicesContainerByAnchor(t *testing.T) {
	html := `<html><body>
		<div class="content">
			<div id="intro">
				<h2>Intro</h2>
				<p>Intro body</p>
			</div>
			<div id="next">
				<h2>Next</h2>
				<p>Next body</p>
			</div>
		</div>
	</body></html>`

	doc, err := parse.NewDocument(html)
	if err != nil {
		t.Fatalf("parse document: %v", err)
	}

	opts := Options{ContentSelector: ".content"}
	sliced := prepareContentDoc(doc, opts, "intro")
	if sliced == nil {
		t.Fatal("expected sliced document")
	}

	text := sliced.Text()
	if !strings.Contains(text, "Intro body") {
		t.Fatalf("expected intro content, got: %s", text)
	}
	if strings.Contains(text, "Next body") {
		t.Fatalf("unexpected next content, got: %s", text)
	}

	htmlOut := documentOuterHTML(sliced)
	if strings.Contains(htmlOut, "<h2>Intro</h2>") {
		t.Fatalf("expected heading to be removed, got: %s", htmlOut)
	}
}

func TestPrepareContentDoc_SlicesHeadingAnchor(t *testing.T) {
	html := `<html><body>
		<div class="content">
			<h2 id="intro">Intro</h2>
			<p>First body</p>
			<h2 id="next">Next</h2>
			<p>Second body</p>
		</div>
	</body></html>`

	doc, err := parse.NewDocument(html)
	if err != nil {
		t.Fatalf("parse document: %v", err)
	}

	opts := Options{ContentSelector: ".content"}
	sliced := prepareContentDoc(doc, opts, "intro")
	if sliced == nil {
		t.Fatal("expected sliced document")
	}

	text := sliced.Text()
	if !strings.Contains(text, "First body") {
		t.Fatalf("expected first body content, got: %s", text)
	}
	if strings.Contains(text, "Second body") {
		t.Fatalf("unexpected second body content, got: %s", text)
	}
	if strings.Contains(text, "Intro") {
		t.Fatalf("expected heading to be removed, got: %s", text)
	}
}

func TestSliceByAnchor_HeadingContainer(t *testing.T) {
	html := `<html><body>
		<div id="target">
			<h2>Title</h2>
			<p>Hello</p>
		</div>
	</body></html>`

	doc, err := parse.NewDocument(html)
	if err != nil {
		t.Fatalf("parse doc: %v", err)
	}

	sliced, ok := sliceByAnchor(doc, "target")
	if !ok {
		t.Fatal("expected slice to succeed")
	}
	if !strings.Contains(documentOuterHTML(sliced), "<p>Hello") {
		t.Fatalf("expected Hello content, got %s", documentOuterHTML(sliced))
	}
}

func TestSliceByAnchor_FallbackHeading(t *testing.T) {
	html := `<html><body>
		<h2 id="t">Title</h2>
		<p>Body</p>
	</body></html>`

	doc, err := parse.NewDocument(html)
	if err != nil {
		t.Fatalf("parse doc: %v", err)
	}

	sliced, ok := sliceByAnchor(doc, "t")
	if !ok {
		t.Fatal("expected slice to succeed")
	}
	if strings.Contains(documentOuterHTML(sliced), "<h2") {
		t.Fatalf("heading should be removed, got %s", documentOuterHTML(sliced))
	}
}

func TestSliceByAnchor_Missing(t *testing.T) {
	html := `<html><body><div id="foo"></div></body></html>`

	doc, err := parse.NewDocument(html)
	if err != nil {
		t.Fatalf("parse doc: %v", err)
	}

	if _, ok := sliceByAnchor(doc, "bar"); ok {
		t.Fatal("expected false for missing anchor")
	}
}

func TestFlattenMenuAndCollectAnchors(t *testing.T) {
	nodes := []menu.Node{
		{
			Title:  "Parent",
			Anchor: "parent",
			Children: []menu.Node{
				{Title: "Child A", Anchor: "a"},
				{Title: "Child B", Anchor: "b"},
			},
		},
	}

	items := flattenMenu(nodes)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[1].Depth != 1 || items[2].Depth != 1 {
		t.Fatalf("expected children depth=1 got %d, %d", items[1].Depth, items[2].Depth)
	}

	anchors := collectAnchors(items)
	if len(anchors) != 3 {
		t.Fatalf("expected 3 anchors, got %d", len(anchors))
	}
	if anchors[1] != "a" || anchors[2] != "b" {
		t.Fatalf("unexpected anchor order: %v", anchors)
	}
}
