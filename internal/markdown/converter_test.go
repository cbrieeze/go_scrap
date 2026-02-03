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
			htmlContent:  `<table><tr><th rowspan="2">A</th><th>B</th></tr><tr><td>C</td></tr></table>`,
			// Spacing/padding is not guaranteed; assert row/column order only.
			wantContains: []string{"## Data", "| A | B |", "| A | C |"},
		},
		{
			name:         "Links and Code",
			headingText:  "API",
			headingLevel: 3,
			htmlContent:  `<p>Check <a href="/docs">relative</a>, <a href="https://example.com/abs">absolute</a>, <a href="#anchor">anchor</a> and <code>code</code>.</p>`,
			// Links should be preserved as Markdown links.
			wantContains: []string{"### API", "relative", "absolute", "anchor", "`code`"},
		},
		{
			name:         "Fenced Code Block With Language",
			headingText:  "Example",
			headingLevel: 2,
			htmlContent: `<pre><button>Copy</button><code class="language-go">fmt.Println("hi")
</code></pre>`,
			wantContains: []string{"## Example", "```go", "fmt.Println(\"hi\")", "```"},
		},
		{
			name:         "Admonition Blockquote",
			headingText:  "Notes",
			headingLevel: 2,
			htmlContent:  `<div class="note"><p>This is a note.</p></div>`,
			wantContains: []string{"## Notes", "> **Note**", "> This is a note."},
		},
		{
			name:         "Description List",
			headingText:  "Defs",
			headingLevel: 2,
			htmlContent: `
				<dl>
				  <dt>Term 1</dt><dd>Definition 1</dd>
				  <dt>Term 2</dt><dd>Definition 2</dd>
				</dl>`,
			wantContains: []string{"## Defs", "**Term 1**", ": Definition 1", "**Term 2**", ": Definition 2"},
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
