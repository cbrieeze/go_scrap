//go:build ignore

package markdown

import (
	"regexp"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
	"github.com/PuerkitoBio/goquery"
)

// CodePlugin returns a plugin that cleans up code blocks.
// It detects language from classes (e.g. language-go) and removes
// common UI artifacts like "Copy" buttons.
func CodePlugin() plugin.Plugin {
	return plugin.NewPlugin(
		plugin.WithRule(plugin.Rule{
			Filter: []string{"pre"},
			Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
				// 1. Detect Language
				language := ""

				// Check <pre> class
				if lang := extractLanguage(selec.AttrOr("class", "")); lang != "" {
					language = lang
				}

				// Check <code> descendants for language class (often overrides pre)
				selec.Find("code").Each(func(_ int, s *goquery.Selection) {
					if language == "" {
						if lang := extractLanguage(s.AttrOr("class", "")); lang != "" {
							language = lang
						}
					}
				})

				// 2. Clean Artifacts
				// Remove "Copy" buttons and other common UI elements
				selec.Find("button").Remove()
				selec.Find(".copy-btn, .clipboard, .line-numbers").Remove()

				// 3. Extract Text
				// If <pre> contains exactly one <code> child, use its text to avoid
				// capturing <pre> headers/footers. Otherwise, use <pre> text.
				var text string
				codeChildren := selec.Children().Filter("code")
				if codeChildren.Length() == 1 {
					text = codeChildren.Text()
				} else {
					text = selec.Text()
				}

				// 4. Format
				text = strings.TrimRight(text, " \t\r\n")
				// Remove leading newline if present (common in <pre><code>\n...)
				text = strings.TrimPrefix(text, "\n")

				var builder strings.Builder
				builder.WriteString("```")
				builder.WriteString(language)
				builder.WriteString("\n")
				builder.WriteString(text)
				builder.WriteString("\n```")

				res := builder.String()
				return &res
			},
		}),
	)
}

var langRe = regexp.MustCompile(`(?i)\b(?:lang|language)-([a-z0-9\+\-\#]+)\b`)

func extractLanguage(class string) string {
	match := langRe.FindStringSubmatch(class)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}
