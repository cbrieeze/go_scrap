package main

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestNodeSelector(t *testing.T) {
	html := `<div id="main"><span class="a b">x</span><p>y</p></div>`
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	div := doc.Find("div").First()
	if got := nodeSelector(div); got != "#main" {
		t.Fatalf("expected #main, got %q", got)
	}

	span := doc.Find("span").First()
	if got := nodeSelector(span); got != "span.a.b" {
		t.Fatalf("expected span.a.b, got %q", got)
	}

	p := doc.Find("p").First()
	if got := nodeSelector(p); got != "p" {
		t.Fatalf("expected p, got %q", got)
	}
}
