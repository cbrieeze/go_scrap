package output

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteMarkdownParts_ProducesIndexAndParts(t *testing.T) {
	dir := t.TempDir()
	segments := []string{
		"# A\n\nParagraph A\n",
		"# B\n\nParagraph B\n",
	}

	out, err := WriteMarkdownParts(dir, "content.md", segments, ChunkLimits{MaxBytes: 30})
	if err != nil {
		t.Fatalf("WriteMarkdownParts: %v", err)
	}

	if _, err := os.Stat(out); err != nil {
		t.Fatalf("missing index file: %v", err)
	}

	idxData, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	if !strings.Contains(string(idxData), "Split into") {
		t.Fatalf("index missing note: %s", string(idxData))
	}

	part1 := filepath.Join(dir, "content", "part-001.md")
	if _, err := os.Stat(part1); err != nil {
		t.Fatalf("missing part file: %v", err)
	}

	partData, err := os.ReadFile(part1)
	if err != nil {
		t.Fatalf("read part: %v", err)
	}
	if !strings.Contains(string(partData), "# A") {
		t.Fatalf("part content wrong: %s", string(partData))
	}
}

func TestWriteMarkdownParts_NoSplitWritesFileOnly(t *testing.T) {
	dir := t.TempDir()
	segments := []string{
		"# Short\n\ntext\n",
	}

	out, err := WriteMarkdownParts(dir, "content.md", segments, ChunkLimits{MaxBytes: 1000})
	if err != nil {
		t.Fatalf("WriteMarkdownParts: %v", err)
	}

	if _, err := os.Stat(out); err != nil {
		t.Fatalf("missing index file: %v", err)
	}

	contentDir := filepath.Join(dir, "content")
	if _, err := os.Stat(contentDir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected no content directory, got %v", err)
	}
}
