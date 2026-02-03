package parse

import "github.com/PuerkitoBio/goquery"

func RemoveSelectors(doc *goquery.Document, selector string) error {
	if doc == nil {
		return nil
	}
	doc.Find(selector).Each(func(_ int, s *goquery.Selection) {
		s.Remove()
	})
	return nil
}
