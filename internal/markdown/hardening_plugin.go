package markdown

import (
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
)

// HardeningPlugin improves conversion for common documentation patterns
// like admonitions and description lists.
func HardeningPlugin() md.Plugin {
	return func(conv *md.Converter) []md.Rule {
		return []md.Rule{
			{
				Filter: []string{"div", "aside"},
				Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
					classes := strings.ToLower(selec.AttrOr("class", ""))

					title := ""
					switch {
					case strings.Contains(classes, "note"):
						title = "Note"
					case strings.Contains(classes, "warning"), strings.Contains(classes, "caution"):
						title = "Warning"
					case strings.Contains(classes, "tip"):
						title = "Tip"
					case strings.Contains(classes, "important"):
						title = "Important"
					case strings.Contains(classes, "info"):
						title = "Info"
					}

					if title == "" {
						return nil
					}

					var b strings.Builder
					b.WriteString("> **" + title + "**\n")
					lines := strings.Split(content, "\n")
					for _, line := range lines {
						trimmed := strings.TrimSpace(line)
						if trimmed == "" {
							b.WriteString(">\n")
						} else {
							b.WriteString("> " + line + "\n")
						}
					}
					b.WriteString("\n")
					out := b.String()
					return &out
				},
			},
			{
				Filter: []string{"dt"},
				Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
					res := "\n**" + strings.TrimSpace(content) + "**\n"
					return &res
				},
			},
			{
				Filter: []string{"dd"},
				Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
					res := ": " + strings.TrimSpace(content) + "\n"
					return &res
				},
			},
		}
	}
}
