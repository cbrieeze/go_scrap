package parse

import (
	"strings"
	"testing"
)

func TestRemoveSelectors(t *testing.T) {
	html := `<div><p class="keep">a</p><p class="rm">b</p><div class="rm">c</div></div>`
	doc, err := NewDocument(html)
	if err != nil {
		t.Fatalf("NewDocument error: %v", err)
	}
	if err := RemoveSelectors(doc, ".rm"); err != nil {
		t.Fatalf("RemoveSelectors error: %v", err)
	}
	out, err := doc.Html()
	if err != nil {
		t.Fatalf("Html error: %v", err)
	}
	if strings.Contains(out, "class=\"rm\"") || strings.Contains(out, ">b<") || strings.Contains(out, ">c<") {
		t.Fatalf("expected removed content, got: %s", out)
	}
	if !strings.Contains(out, "class=\"keep\"") {
		t.Fatalf("expected keep content, got: %s", out)
	}
}
