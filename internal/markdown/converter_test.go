package markdown_test

import (
	"strings"
	"testing"

	"go_scrap/internal/markdown"
)

func TestSectionToMarkdown(t *testing.T) {
	tests := []struct {
		name         string
		headingText  string
		headingLevel int
		htmlContent  string
		wantContains []string
	}{
		{
			name:         "Basic Paragraph",
			headingText:  "Introduction",
			headingLevel: 1,
			htmlContent:  "<p>Hello world</p>",
			wantContains: []string{"# Introduction", "Hello world"},
		},
		{
			name:         "Complex Table",
			headingText:  "Data",
			headingLevel: 2,
			htmlContent:  `<table><tr><th>ID</th><th>Value</th></tr><tr><td>1</td><td>A</td></tr></table>`,
			// Spacing/padding is not guaranteed; assert row/column order only.
			wantContains: []string{"## Data", "| ID | Value |", "| 1 | A |"},
		},
		{
			name:         "Links and Code",
			headingText:  "API",
			headingLevel: 3,
			htmlContent:  `<p>Check <a href="/docs">docs</a> and <code>code</code>.</p>`,
			// Links should be preserved as Markdown links.
			wantContains: []string{"### API", "Check [docs](/docs) and `code`."},
		},
		{
			name:         "Fenced Code Block With Language",
			headingText:  "Example",
			headingLevel: 2,
			htmlContent: `<pre><code class="language-go">fmt.Println("hi")
</code></pre>`,
			wantContains: []string{"## Example", "```go", "fmt.Println(\"hi\")", "```"},
		},
		{
			name:         "Empty Content",
			headingText:  "Empty",
			headingLevel: 1,
			htmlContent:  "",
			wantContains: []string{"# Empty"},
		},
	}

	conv := markdown.NewConverter()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := conv.SectionToMarkdown(tt.headingText, tt.headingLevel, tt.htmlContent)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("expected output to contain %q, but got:\n%s", want, got)
				}
			}
		})
	}
}
