package menu

import (
	"errors"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

type Node struct {
	Title    string `json:"title"`
	Href     string `json:"href"`
	Anchor   string `json:"anchor"`
	Children []Node `json:"children,omitempty"`
}

func Extract(htmlText, selector string) ([]Node, error) {
	if strings.TrimSpace(selector) == "" {
		return nil, errors.New("nav selector is required")
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlText))
	if err != nil {
		return nil, err
	}
	nav := doc.Find(selector).First()
	if nav.Length() == 0 {
		return nil, errors.New("nav selector not found")
	}

	list := nav.Find("ul, ol").First()
	if list.Length() == 0 {
		return extractFlat(nav), nil
	}
	return extractList(list), nil
}

func extractList(list *goquery.Selection) []Node {
	nodes := []Node{}
	list.ChildrenFiltered("li").Each(func(_ int, li *goquery.Selection) {
		node := nodeFromLI(li)
		if node.Title != "" || node.Href != "" {
			nodes = append(nodes, node)
		}
	})
	return nodes
}

func nodeFromLI(li *goquery.Selection) Node {
	a := li.Find("a").First()
	href, _ := a.Attr("href")
	title := strings.TrimSpace(a.Text())
	node := Node{
		Title:  title,
		Href:   href,
		Anchor: anchorFromHref(href),
	}

	childList := li.Find("ul, ol").First()
	if childList.Length() > 0 {
		node.Children = extractList(childList)
	}
	return node
}

func extractFlat(nav *goquery.Selection) []Node {
	nodes := []Node{}
	nav.Find("a").Each(func(_ int, a *goquery.Selection) {
		href, _ := a.Attr("href")
		title := strings.TrimSpace(a.Text())
		if title == "" && href == "" {
			return
		}
		nodes = append(nodes, Node{Title: title, Href: href, Anchor: anchorFromHref(href)})
	})
	return nodes
}

func anchorFromHref(href string) string {
	href = strings.TrimSpace(href)
	if href == "" {
		return ""
	}

	if strings.HasPrefix(href, "#") {
		return strings.TrimPrefix(href, "#")
	}

	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	return u.Fragment
}
