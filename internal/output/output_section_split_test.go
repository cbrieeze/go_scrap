package output

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go_scrap/internal/menu"
)

func TestWriteSectionFiles_CreatesIndexAndParts(t *testing.T) {
	dir := t.TempDir()
	nodes := []menu.Node{
		{Title: "Alpha Section", Anchor: "alpha"},
	}

	md := "## Alpha Section\n\n" +
		"Intro paragraph.\n\n" +
		"### Details\n" +
		strings.Repeat("word ", 200) +
		"\n\n### Summary\n" +
		strings.Repeat("note ", 200)

	if err := WriteSectionFiles(dir, nodes, map[string]string{"alpha": md}, 0, ChunkLimits{MaxBytes: 512}); err != nil {
		t.Fatalf("WriteSectionFiles error: %v", err)
	}

	base := filepath.Join(dir, "sections")
	indexPath := filepath.Join(base, "alpha-section.md")
	data, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("missing index: %v", err)
	}
	if !strings.Contains(string(data), "Split into") {
		t.Fatalf("index missing split info: %s", string(data))
	}

	partDir := filepath.Join(base, "alpha-section")
	part1 := filepath.Join(partDir, "part-001.md")
	part2 := filepath.Join(partDir, "part-002.md")
	if _, err := os.Stat(part1); err != nil {
		t.Fatalf("expected part 1: %v", err)
	}
	if _, err := os.Stat(part2); err != nil {
		t.Fatalf("expected part 2: %v", err)
	}

	content, err := os.ReadFile(part1)
	if err != nil {
		t.Fatalf("read part1: %v", err)
	}
	if !strings.Contains(string(content), "Alpha Section") {
		t.Fatalf("part1 missing heading: %s", string(content))
	}
}
