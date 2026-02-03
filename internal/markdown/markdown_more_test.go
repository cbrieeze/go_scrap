package markdown

import (
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

func TestDetectLanguageVariants(t *testing.T) {
	tests := []struct {
		class string
		want  string
	}{
		{"language-go", "go"},
		{"lang-golang", "go"},
		{"language-python", "python"},
		{"", ""},
	}
	for _, tt := range tests {
		codeHTML := `<code class="` + tt.class + `">x</code>`
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(codeHTML))
		if err != nil {
			t.Fatalf("doc error: %v", err)
		}
		got := detectLanguage(doc.Find("code").First())
		if got != tt.want {
			t.Fatalf("detectLanguage(%q)=%q want %q", tt.class, got, tt.want)
		}
	}
}

func TestCodeFenceUsesFourBackticksWhenNeeded(t *testing.T) {
	conv := NewConverter()
	html := "<pre><code class=\"language-go\">fmt.Println(\"```\")</code></pre>"
	out, err := conv.SectionToMarkdown("Example", 2, html)
	if err != nil {
		t.Fatalf("SectionToMarkdown error: %v", err)
	}
	if !strings.Contains(out, "````go") {
		t.Fatalf("expected 4-backtick fence, got:\n%s", out)
	}
}
