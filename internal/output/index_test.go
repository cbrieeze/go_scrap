package output

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go_scrap/internal/parse"
)

func TestWriteIndex_BuildsHierarchyAndStableIDs(t *testing.T) {
	dir := t.TempDir()
	baseURL := "https://example.com/docs"

	sections := []parse.Section{
		{HeadingText: "Intro", HeadingLevel: 1, HeadingID: "intro", ContentHTML: "<p>a</p>"},
		{HeadingText: "Child", HeadingLevel: 2, HeadingID: "child", ContentHTML: "<p>abcd</p>"},
		{HeadingText: "Sibling", HeadingLevel: 2, HeadingID: "sibling", ContentHTML: "<p>xyz</p>"},
	}

	outPath, err := WriteIndex(dir, baseURL, sections)
	if err != nil {
		t.Fatalf("WriteIndex error: %v", err)
	}
	if outPath != filepath.Join(dir, "index.jsonl") {
		t.Fatalf("unexpected path: %s", outPath)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read index: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}

	var rec0, rec1, rec2 IndexRecord
	if err := json.Unmarshal([]byte(lines[0]), &rec0); err != nil {
		t.Fatalf("unmarshal line 1: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &rec1); err != nil {
		t.Fatalf("unmarshal line 2: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[2]), &rec2); err != nil {
		t.Fatalf("unmarshal line 3: %v", err)
	}

	if rec0.HeadingPath != "Intro" {
		t.Fatalf("unexpected heading path: %q", rec0.HeadingPath)
	}
	if rec1.HeadingPath != "Intro > Child" {
		t.Fatalf("unexpected heading path: %q", rec1.HeadingPath)
	}
	if rec2.HeadingPath != "Intro > Sibling" {
		t.Fatalf("unexpected heading path: %q", rec2.HeadingPath)
	}

	if rec1.SourceURL != baseURL+"#child" {
		t.Fatalf("unexpected source url: %q", rec1.SourceURL)
	}

	wantIDRaw := baseURL + "|" + rec1.HeadingPath + "|" + "child"
	sum := sha256.Sum256([]byte(wantIDRaw))
	wantID := hex.EncodeToString(sum[:])[:16]
	if rec1.ID != wantID {
		t.Fatalf("unexpected stable id: got %q want %q", rec1.ID, wantID)
	}

	if rec1.TokenEstimate != len(sections[1].ContentHTML)/4 {
		t.Fatalf("unexpected token estimate: %d", rec1.TokenEstimate)
	}
}

func TestSlugify(t *testing.T) {
	if got := slugify("Hello / World?"); got != "hello---world" {
		t.Fatalf("unexpected slug: %q", got)
	}
	if got := slugify("  Many   Spaces  "); got != "many---spaces" {
		t.Fatalf("unexpected slug: %q", got)
	}
}
