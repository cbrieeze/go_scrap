package menu_test

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"

	"go_scrap/internal/menu"
)

func TestExtract_NestedMenu(t *testing.T) {
	html := `
	<nav class="nav">
	  <ul>
	    <li><a href="#a">A</a></li>
	    <li>
	      <a href="#b">B</a>
	      <ul>
	        <li><a href="#b1">B1</a></li>
	      </ul>
	    </li>
	  </ul>
	</nav>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes, err := menu.Extract(doc, ".nav")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 top-level nodes, got %d", len(nodes))
	}
	if nodes[1].Title != "B" || len(nodes[1].Children) != 1 {
		t.Fatalf("expected nested child under B, got %+v", nodes[1])
	}
	if nodes[1].Children[0].Anchor != "b1" {
		t.Fatalf("expected anchor b1, got %s", nodes[1].Children[0].Anchor)
	}
}

func TestExtract_FlatMenu(t *testing.T) {
	html := `<nav class="nav"><a href="#x">X</a><a href="#y">Y</a></nav>`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nodes, err := menu.Extract(doc, ".nav")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if nodes[0].Anchor != "x" || nodes[1].Anchor != "y" {
		t.Fatalf("unexpected anchors: %+v", nodes)
	}
}

func TestExtract_SelectorMissing(t *testing.T) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader("<div></div>"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = menu.Extract(doc, ".nav")
	if err == nil {
		t.Fatal("expected error for missing nav selector")
	}
}
