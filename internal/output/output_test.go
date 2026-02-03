package output_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go_scrap/internal/menu"
	"go_scrap/internal/output"
	"go_scrap/internal/parse"
	"go_scrap/internal/report"
)

func TestWriteAllAndMenuAndSections(t *testing.T) {
	dir := t.TempDir()
	doc := &parse.Document{HeadingIDs: []string{"a"}, AnchorTargets: []string{"a"}, Sections: []parse.Section{{HeadingText: "A", HeadingLevel: 1, HeadingID: "a", ContentHTML: "<p>x</p>", ContentText: "x"}}}
	rep := report.Report{}

	mdPath, jsonPath, err := output.WriteAll(doc, rep, "# A\n\nx\n", output.WriteOptions{OutputDir: dir})
	if err != nil {
		t.Fatalf("WriteAll error: %v", err)
	}
	if _, err := os.Stat(mdPath); err != nil {
		t.Fatalf("missing markdown output: %v", err)
	}
	if _, err := os.Stat(jsonPath); err != nil {
		t.Fatalf("missing json output: %v", err)
	}

	nodes := []menu.Node{{Title: "A", Href: "#a", Anchor: "a"}}
	if err := output.WriteMenu(dir, nodes); err != nil {
		t.Fatalf("WriteMenu error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "menu.json")); err != nil {
		t.Fatalf("missing menu.json: %v", err)
	}

	mdByID := map[string]string{"a": "# A\n\nx\n"}
	if err := output.WriteSectionFiles(dir, nodes, mdByID, 0, output.ChunkLimits{}); err != nil {
		t.Fatalf("WriteSectionFiles error: %v", err)
	}
	sectionPath := filepath.Join(dir, "sections", "a.md")
	data, err := os.ReadFile(sectionPath)
	if err != nil {
		t.Fatalf("missing section file: %v", err)
	}
	if !strings.Contains(string(data), "# A") {
		t.Fatalf("unexpected section content: %s", string(data))
	}
}

func TestWriteSectionFiles_SplitsLargeMarkdown(t *testing.T) {
	dir := t.TempDir()
	nodes := []menu.Node{{Title: "API Index", Href: "#api_index", Anchor: "api_index"}}
	md := "## API Index\n\n### Part A\n\n" + strings.Repeat("word ", 200) + "\n\n### Part B\n\n" + strings.Repeat("word ", 200)
	mdByID := map[string]string{"api_index": md}

	if err := output.WriteSectionFiles(dir, nodes, mdByID, 0, output.ChunkLimits{MaxBytes: 120}); err != nil {
		t.Fatalf("WriteSectionFiles error: %v", err)
	}

	indexPath := filepath.Join(dir, "sections", "api-index.md")
	indexData, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("missing split index file: %v", err)
	}
	if !strings.Contains(string(indexData), "Split into") {
		t.Fatalf("expected split index content, got: %s", string(indexData))
	}

	partPath := filepath.Join(dir, "sections", "api-index", "part-001.md")
	if _, err := os.Stat(partPath); err != nil {
		t.Fatalf("missing split part file: %v", err)
	}
}

func TestWriteMarkdownParts_BundlesBySection(t *testing.T) {
	dir := t.TempDir()
	parts := []string{
		"# One\n\n" + strings.Repeat("a", 50) + "\n",
		"# Two\n\n" + strings.Repeat("b", 50) + "\n",
		"# Three\n\n" + strings.Repeat("c", 50) + "\n",
	}

	mdPath, err := output.WriteMarkdownParts(dir, "content.md", parts, output.ChunkLimits{MaxBytes: 120})
	if err != nil {
		t.Fatalf("WriteMarkdownParts error: %v", err)
	}
	if _, err := os.Stat(mdPath); err != nil {
		t.Fatalf("missing content index: %v", err)
	}
	partPath := filepath.Join(dir, "content", "part-001.md")
	if _, err := os.Stat(partPath); err != nil {
		t.Fatalf("missing content part: %v", err)
	}
}
