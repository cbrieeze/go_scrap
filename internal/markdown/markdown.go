package markdown

import (
	"regexp"
	"strings"

	htmltomd "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
	"github.com/PuerkitoBio/goquery"
)

type Converter struct {
	md *htmltomd.Converter
}

func NewConverter() *Converter {
	conv := htmltomd.NewConverter("", true, nil)
	conv.Use(plugin.GitHubFlavored())
	conv.Use(TablePlugin())
	conv.Use(HardeningPlugin())

	// Custom rule to preserve fenced code blocks with language hints.
	conv.AddRules(codeBlockRule())

	return &Converter{md: conv}
}

func (c *Converter) SectionToMarkdown(headingText string, headingLevel int, contentHTML string) (string, error) {
	heading := "#"
	if headingLevel > 1 {
		heading = strings.Repeat("#", headingLevel)
	}
	headingLine := strings.TrimSpace(heading + " " + headingText)

	body, err := c.md.ConvertString(contentHTML)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(body) == "" {
		return headingLine + "\n", nil
	}
	return headingLine + "\n\n" + strings.TrimSpace(body) + "\n", nil
}

func codeBlockRule() htmltomd.Rule {
	return htmltomd.Rule{
		Filter: []string{"pre"},
		Replacement: func(_ string, selec *goquery.Selection, _ *htmltomd.Options) *string {
			if selec == nil {
				empty := ""
				return &empty
			}

			code := selec.Find("code").First()
			if code.Length() == 0 {
				// Fall back to default content conversion.
				return nil
			}

			lang := detectLanguage(code)
			text := code.Text()
			text = strings.ReplaceAll(text, "\r\n", "\n")
			text = strings.TrimSuffix(text, "\n")

			fence := "```"
			if strings.Contains(text, "```") {
				fence = "````"
			}

			var b strings.Builder
			b.WriteString("\n")
			b.WriteString(fence)
			if lang != "" {
				b.WriteString(lang)
			}
			b.WriteString("\n")
			b.WriteString(text)
			b.WriteString("\n")
			b.WriteString(fence)
			b.WriteString("\n")
			out := b.String()
			return &out
		},
	}
}

func detectLanguage(code *goquery.Selection) string {
	class, _ := code.Attr("class")
	class = strings.TrimSpace(class)
	if class == "" {
		return ""
	}

	// Common patterns: "language-go", "lang-go", "language-golang".
	re := regexp.MustCompile(`(?:^|\s)(?:language|lang)-([a-zA-Z0-9_+-]+)(?:\s|$)`) // keep simple
	m := re.FindStringSubmatch(class)
	if len(m) == 2 {
		lang := strings.ToLower(m[1])
		if lang == "golang" {
			lang = "go"
		}
		return lang
	}
	return ""
}
