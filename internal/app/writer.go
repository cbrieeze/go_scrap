package app

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"go_scrap/internal/markdown"
	"go_scrap/internal/menu"
	"go_scrap/internal/output"
	"go_scrap/internal/parse"

	"github.com/PuerkitoBio/goquery"
)

type sectionMarkdown struct {
	HeadingID  string
	ContentIDs []string
	Markdown   string
}

func writeOutputsWithMarkdown(opts Options, baseDoc *goquery.Document, result analysisResult, md string, sectionMarkdowns []sectionMarkdown) (WriteResult, error) {
	written := WriteResult{OutputDir: opts.OutputDir}
	if opts.Strict && reportHasIssues(result.Rep) {
		return WriteResult{}, errors.New("completeness checks failed (use --strict=false to allow)")
	}

	jsonPath, err := output.WriteJSON(result.Doc, result.Rep, output.WriteOptions{OutputDir: opts.OutputDir})
	if err != nil {
		return WriteResult{}, err
	}
	written.JSONPath = jsonPath

	var mdPath string
	limits := chunkLimits(opts)
	contentParts := make([]string, 0, len(sectionMarkdowns))
	for _, sm := range sectionMarkdowns {
		contentParts = append(contentParts, sm.Markdown)
	}
	if limits.Enabled() {
		mdPath, err = output.WriteMarkdownParts(opts.OutputDir, "content.md", contentParts, limits)
	} else {
		mdPath, err = output.WriteMarkdown(opts.OutputDir, "content.md", md)
	}
	if err != nil {
		return WriteResult{}, err
	}
	written.MarkdownPath = mdPath

	if opts.Stdout {
		fmt.Println(md)
	} else {
		fmt.Printf("\nWrote markdown: %s\n", mdPath)
		fmt.Printf("Wrote json: %s\n", jsonPath)
	}

	if err := writeMenuOutputs(opts, baseDoc, result.Doc, sectionMarkdowns); err != nil {
		return WriteResult{}, err
	}
	if strings.TrimSpace(opts.NavSelector) != "" {
		written.MenuPath = filepath.Join(opts.OutputDir, "menu.json")
	}

	if !opts.Stdout {
		if indexPath, err := output.WriteIndex(opts.OutputDir, opts.URL, result.Doc.Sections); err == nil {
			fmt.Printf("Wrote index: %s\n", indexPath)
			written.IndexPath = indexPath
		}
	}

	return written, nil
}

func trimSections(doc *parse.Document, maxSections int) {
	if maxSections > 0 && maxSections < len(doc.Sections) {
		doc.Sections = doc.Sections[:maxSections]
	}
}

func chunkLimits(opts Options) output.ChunkLimits {
	return output.ChunkLimits{
		MaxBytes:  opts.MaxMarkdownBytes,
		MaxChars:  opts.MaxChars,
		MaxTokens: opts.MaxTokens,
	}
}

func applyExclusions(doc *goquery.Document, selector string) {
	if strings.TrimSpace(selector) == "" {
		return
	}
	_ = parse.RemoveSelectors(doc, selector)
}

func buildMarkdown(conv *markdown.Converter, sections []parse.Section) (string, []sectionMarkdown, error) {
	var mdBuilder strings.Builder
	parts := make([]sectionMarkdown, 0, len(sections))
	for _, section := range sections {
		md, err := conv.SectionToMarkdown(section.HeadingText, section.HeadingLevel, section.ContentHTML)
		if err != nil {
			return "", nil, err
		}
		mdBuilder.WriteString(md)
		mdBuilder.WriteString("\n")
		if !strings.HasSuffix(md, "\n") {
			md += "\n"
		}
		parts = append(parts, sectionMarkdown{
			HeadingID:  section.HeadingID,
			ContentIDs: section.ContentIDs,
			Markdown:   md,
		})
	}
	return mdBuilder.String(), parts, nil
}

func writeMenuOutputs(opts Options, baseDoc *goquery.Document, _ *parse.Document, sections []sectionMarkdown) error {
	if strings.TrimSpace(opts.NavSelector) == "" {
		return nil
	}
	nodes, err := menu.Extract(baseDoc, opts.NavSelector)
	if err != nil {
		return fmt.Errorf("menu extract failed (%s): %w", opts.NavSelector, err)
	}
	if err := output.WriteMenu(opts.OutputDir, nodes); err != nil {
		return fmt.Errorf("menu write failed: %w", err)
	}

	mdByID := map[string]string{}
	for _, section := range sections {
		md := section.Markdown
		if opts.DownloadAssets {
			md = strings.ReplaceAll(md, "(assets/", "(../assets/")
			md = strings.ReplaceAll(md, "\"assets/", "\"../assets/")
		}

		if section.HeadingID != "" {
			mdByID[section.HeadingID] = md
		}
		for _, id := range section.ContentIDs {
			if _, ok := mdByID[id]; !ok {
				mdByID[id] = md
			}
		}
	}

	limits := chunkLimits(opts)
	if err := output.WriteSectionFiles(opts.OutputDir, nodes, mdByID, opts.MaxMenuItems, limits); err != nil {
		return fmt.Errorf("section write failed: %w", err)
	}
	return nil
}
