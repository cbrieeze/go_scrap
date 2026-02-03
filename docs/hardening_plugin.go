//go:build ignore

package markdown

import (
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
	"github.com/PuerkitoBio/goquery"
)

// HardeningPlugin returns a plugin that improves conversion for common
// documentation patterns like admonitions and description lists.
func HardeningPlugin() plugin.Plugin {
	return plugin.NewPlugin(
		// Rule for Admonitions (divs/asides with specific classes)
		plugin.WithRule(plugin.Rule{
			Filter: []string{"div", "aside"},
			Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
				classes := selec.AttrOr("class", "")
				lowerClasses := strings.ToLower(classes)

				// Detect type based on class name
				var title string
				if strings.Contains(lowerClasses, "note") {
					title = "Note"
				} else if strings.Contains(lowerClasses, "warning") || strings.Contains(lowerClasses, "caution") {
					title = "Warning"
				} else if strings.Contains(lowerClasses, "tip") {
					title = "Tip"
				} else if strings.Contains(lowerClasses, "important") {
					title = "Important"
				} else if strings.Contains(lowerClasses, "info") {
					title = "Info"
				}

				// If not an admonition, let the default converter handle it
				if title == "" {
					return nil
				}

				// Convert to Blockquote with bold title
				var builder strings.Builder
				builder.WriteString("> **" + title + "**\n")

				lines := strings.Split(content, "\n")
				for _, line := range lines {
					trimmed := strings.TrimSpace(line)
					if trimmed == "" {
						builder.WriteString(">\n")
					} else {
						builder.WriteString("> " + line + "\n")
					}
				}
				builder.WriteString("\n")

				res := builder.String()
				return &res
			},
		}),

		// Rules for Description Lists (dl, dt, dd)
		// Markdown doesn't support these natively, so we format them as Bold Term + Indented Definition.
		plugin.WithRule(plugin.Rule{
			Filter: []string{"dt"},
			Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
				// Add newline before term for spacing
				res := "\n**" + strings.TrimSpace(content) + "**\n"
				return &res
			},
		}),
		plugin.WithRule(plugin.Rule{
			Filter: []string{"dd"},
			Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
				res := ": " + strings.TrimSpace(content) + "\n"
				return &res
			},
		}),
	)
}
