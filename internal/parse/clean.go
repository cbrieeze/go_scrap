package parse

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func RemoveSelectors(htmlText, selector string) (string, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		return "", err
	}
	doc.Find(selector).Each(func(_ int, s *goquery.Selection) {
		s.Remove()
	})
	return doc.Html()
}
