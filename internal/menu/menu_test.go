package menu_test

import (
	"testing"

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

	nodes, err := menu.Extract(html, ".nav")
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

	nodes, err := menu.Extract(html, ".nav")
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
	_, err := menu.Extract("<div></div>", ".nav")
	if err == nil {
		t.Fatal("expected error for missing nav selector")
	}
}
