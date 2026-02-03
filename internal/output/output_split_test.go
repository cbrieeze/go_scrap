package output

import (
	"strings"
	"testing"
)

func TestSplitOnSubheadings_SplitsAtTertiaryHeadings(t *testing.T) {
	md := "## Title\n\nLead paragraph\n\n### Alpha\nAlpha content\n\n### Beta\nBeta content\n"
	blocks := splitOnSubheadings(md)
	if len(blocks) != 3 {
		t.Fatalf("expected 3 blocks, got %d", len(blocks))
	}
	if !strings.Contains(blocks[0], "## Title") {
		t.Fatalf("block 0 missing title: %q", blocks[0])
	}
	if !strings.Contains(blocks[1], "### Alpha") {
		t.Fatalf("block 1 missing alpha heading: %q", blocks[1])
	}
	if !strings.Contains(blocks[2], "### Beta") {
		t.Fatalf("block 2 missing beta heading: %q", blocks[2])
	}
}

func TestBundleParts_RespectsMaxBytesBySection(t *testing.T) {
	partA := "### A\n" + strings.Repeat("a", 250)
	partB := "### B\n" + strings.Repeat("b", 250)
	parts := []string{partA + "\n", partB + "\n"}

	bundles := bundleParts(parts, ChunkLimits{MaxBytes: len(partA) + 50})
	if len(bundles) != 2 {
		t.Fatalf("expected 2 bundles, got %d", len(bundles))
	}
	if !strings.Contains(bundles[0], "### A") {
		t.Fatalf("bundle 0 missing section A: %q", bundles[0])
	}
	if !strings.Contains(bundles[1], "### B") {
		t.Fatalf("bundle 1 missing section B: %q", bundles[1])
	}
}

func TestSplitMarkdownByHeadings_MaxChars(t *testing.T) {
	md := "## Title\n\n### One\n" + strings.Repeat("x", 200) + "\n\n### Two\n" + strings.Repeat("y", 200)
	parts := splitMarkdownByHeadings(md, ChunkLimits{MaxChars: 180})
	if len(parts) < 2 {
		t.Fatalf("expected split by chars, got %d parts", len(parts))
	}
}

func TestSplitMarkdownByHeadings_MaxTokens(t *testing.T) {
	md := "## Title\n\n### One\n" + strings.Repeat("word ", 200) + "\n\n### Two\n" + strings.Repeat("word ", 200)
	parts := splitMarkdownByHeadings(md, ChunkLimits{MaxTokens: 100})
	if len(parts) < 2 {
		t.Fatalf("expected split by tokens, got %d parts", len(parts))
	}
}
