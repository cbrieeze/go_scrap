package inspect

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"go_scrap/internal/app"
	"go_scrap/internal/fetch"

	"github.com/PuerkitoBio/goquery"
)

type candidate struct {
	Selector string
	Links    int
	Text     int
}

type options struct {
	URL           string
	WaitFor       string
	TimeoutSec    int
	CheckSelector string
	UseCache      bool
	Headless      bool
}

func Run(args []string) error {
	opts, err := parseOptions(args)
	if err != nil {
		return err
	}
	if strings.TrimSpace(opts.URL) == "" {
		return errors.New("--url is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(opts.TimeoutSec)*time.Second)
	defer cancel()

	result, err := loadHTML(ctx, opts)
	if err != nil {
		return err
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(result.HTML))
	if err != nil {
		return err
	}

	if strings.TrimSpace(opts.CheckSelector) != "" {
		inspectSpecificSelector(doc, opts.CheckSelector)
		return nil
	}

	candidates := collectCandidates(doc)
	printCandidates(candidates)
	printTopLinkContainers(doc, 5)
	return nil
}

func parseOptions(args []string) (options, error) {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	opts := options{}
	fs.StringVar(&opts.URL, "url", "", "URL to inspect")
	fs.StringVar(&opts.WaitFor, "wait-for", "body", "CSS selector to wait for")
	fs.IntVar(&opts.TimeoutSec, "timeout", app.DefaultTimeoutSeconds, "Timeout seconds")
	fs.StringVar(&opts.CheckSelector, "check-selector", "", "Specific selector to validate")
	fs.BoolVar(&opts.UseCache, "cache", false, "Use disk cache for HTML content")
	fs.BoolVar(&opts.Headless, "headless", true, "Run browser headless")
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	return opts, nil
}

func loadHTML(ctx context.Context, opts options) (fetch.Result, error) {
	if opts.UseCache {
		cachePath := fetch.GetCachePath(opts.URL)
		if content, err := os.ReadFile(cachePath); err == nil {
			fmt.Printf("Loaded from cache: %s\n", cachePath)
			return fetch.Result{HTML: string(content), SourceInfo: "cache"}, nil
		}
	}

	result, err := fetch.Fetch(ctx, fetch.Options{
		URL:             opts.URL,
		Mode:            fetch.ModeDynamic,
		Timeout:         time.Duration(opts.TimeoutSec) * time.Second,
		WaitForSelector: opts.WaitFor,
		Headless:        opts.Headless,
		UserAgent:       app.DefaultUserAgent,
	})
	if err != nil {
		return fetch.Result{}, err
	}

	if opts.UseCache {
		cachePath := fetch.GetCachePath(opts.URL)
		_ = fetch.SaveToCache(cachePath, result.HTML)
	}

	return result, nil
}

func collectCandidates(doc *goquery.Document) []candidate {
	selectors := []string{
		"nav", "aside", "[role='navigation']", ".sidebar", ".toc", ".menu", ".nav",
		"#sidebar", "#toc", "#nav", "main", "article", ".content", "#content",
	}

	candidates := []candidate{}
	for _, sel := range selectors {
		doc.Find(sel).Each(func(_ int, s *goquery.Selection) {
			linkCount := s.Find("a").Length()
			textCount := len(strings.TrimSpace(s.Text()))
			if linkCount == 0 && textCount == 0 {
				return
			}
			candidates = append(candidates, candidate{Selector: sel, Links: linkCount, Text: textCount})
		})
	}
	return candidates
}

func printCandidates(candidates []candidate) {
	fmt.Println("Selector candidates (links/text length):")
	for _, c := range candidates {
		fmt.Printf("- %s: links=%d text=%d\n", c.Selector, c.Links, c.Text)
	}
}

func printTopLinkContainers(doc *goquery.Document, limit int) {
	fmt.Println("\nTop containers by link count (any element):")
	type box struct {
		Sel   string
		Links int
	}
	boxes := []box{}
	doc.Find("*").Each(func(_ int, s *goquery.Selection) {
		links := s.Find("a").Length()
		if links >= 10 {
			boxes = append(boxes, box{Sel: nodeSelector(s), Links: links})
		}
	})
	for i, b := range boxes {
		if i >= limit {
			break
		}
		fmt.Printf("- %s (links=%d)\n", b.Sel, b.Links)
	}
}

func nodeSelector(s *goquery.Selection) string {
	if s.Length() == 0 {
		return ""
	}
	if id, exists := s.Attr("id"); exists && id != "" {
		return fmt.Sprintf("#%s", id)
	}
	if classStr, exists := s.Attr("class"); exists {
		classes := strings.Fields(classStr)
		if len(classes) > 0 {
			return fmt.Sprintf("%s.%s", s.Get(0).Data, strings.Join(classes, "."))
		}
	}
	return s.Get(0).Data
}

func inspectSpecificSelector(doc *goquery.Document, selector string) {
	sel := doc.Find(selector)
	fmt.Printf("Inspecting selector: '%s'\n", selector)
	fmt.Printf("Found %d matching element(s)\n", sel.Length())

	sel.Each(func(i int, s *goquery.Selection) {
		if i >= 3 {
			return
		}
		fmt.Printf("\n--- Match #%d ---\n", i+1)

		if s.Length() > 0 && s.Get(0) != nil {
			fmt.Printf("Tag: %s\n", s.Get(0).Data)
		}

		if id, ok := s.Attr("id"); ok {
			fmt.Printf("ID: %s\n", id)
		}
		if class, ok := s.Attr("class"); ok {
			fmt.Printf("Class: %s\n", class)
		}

		text := strings.TrimSpace(s.Text())
		fmt.Printf("Text Length: %d chars\n", len(text))
		if len(text) > 100 {
			fmt.Printf("Text Preview: %s...\n", text[:100])
		} else {
			fmt.Printf("Text Preview: %s\n", text)
		}

		links := s.Find("a").Length()
		fmt.Printf("Links inside: %d\n", links)
	})
}
