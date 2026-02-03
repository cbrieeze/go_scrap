//go:build ignore

package markdown

import (
	"net/url"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
	"github.com/PuerkitoBio/goquery"
)

// LinkPlugin returns a plugin that resolves relative URLs to absolute ones
// based on the provided baseURL.
func LinkPlugin(baseURL string) plugin.Plugin {
	base, err := url.Parse(baseURL)
	// If baseURL is invalid, we can't resolve relative links reliably.
	// We return a no-op plugin (empty).
	if err != nil {
		return plugin.NewPlugin()
	}

	return plugin.NewPlugin(
		plugin.WithRule(plugin.Rule{
			Filter: []string{"a"},
			Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
				href, exists := selec.Attr("href")
				if !exists || strings.TrimSpace(href) == "" {
					return nil
				}

				// Ignore internal anchors or javascript links
				if strings.HasPrefix(href, "#") || strings.HasPrefix(strings.ToLower(href), "javascript:") {
					return nil
				}

				u, err := url.Parse(href)
				if err != nil {
					return nil
				}

				// Resolve reference against base URL
				absURL := base.ResolveReference(u).String()

				// Reconstruct Markdown link: [content](url "title")
				title := selec.AttrOr("title", "")
				var replacement string
				if title != "" {
					replacement = "[" + content + "](" + absURL + " \"" + title + "\")"
				} else {
					replacement = "[" + content + "](" + absURL + ")"
				}

				return &replacement
			},
		}),
	)
}
