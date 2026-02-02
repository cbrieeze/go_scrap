package report_test

import (
	"testing"

	"go_scrap/internal/parse"
	"go_scrap/internal/report"
)

func TestAnalyze_HeadingGaps(t *testing.T) {
	doc := &parse.Document{
		Sections: []parse.Section{
			{HeadingText: "Top", HeadingLevel: 1, HeadingID: "top", ContentText: "x"},
			{HeadingText: "Skipped", HeadingLevel: 3, HeadingID: "skipped", ContentText: "y"},
		},
		AllElementIDs: []string{"top", "skipped"},
	}

	rep := report.Analyze(doc)
	if len(rep.HeadingGaps) != 1 {
		t.Fatalf("expected 1 heading gap, got %d (%v)", len(rep.HeadingGaps), rep.HeadingGaps)
	}
	if rep.HeadingGaps[0] != "Skipped" {
		t.Fatalf("expected gap at 'Skipped', got %v", rep.HeadingGaps)
	}
}

func TestAnalyze_BrokenAnchors(t *testing.T) {
	doc := &parse.Document{
		Sections:      []parse.Section{{HeadingText: "A", HeadingLevel: 1, HeadingID: "a", ContentText: "x"}},
		AllElementIDs: []string{"a"},
		AnchorTargets: []string{"missing"},
	}

	rep := report.Analyze(doc)
	if len(rep.BrokenAnchors) != 1 || rep.BrokenAnchors[0] != "missing" {
		t.Fatalf("expected broken anchor 'missing', got %v", rep.BrokenAnchors)
	}
}

func TestAnalyze_DuplicateIDs(t *testing.T) {
	doc := &parse.Document{AllElementIDs: []string{"dup", "dup", "ok"}}
	rep := report.Analyze(doc)
	if len(rep.DuplicateIDs) != 1 || rep.DuplicateIDs[0] != "dup" {
		t.Fatalf("expected duplicate id 'dup', got %v", rep.DuplicateIDs)
	}
}

func TestAnalyze_MissingHeadingIDs(t *testing.T) {
	doc := &parse.Document{Sections: []parse.Section{{HeadingText: "NoID", HeadingLevel: 2, HeadingID: "", ContentText: "x"}}}
	rep := report.Analyze(doc)
	if len(rep.MissingHeadingIDs) != 1 || rep.MissingHeadingIDs[0] != "NoID" {
		t.Fatalf("expected missing heading id for 'NoID', got %v", rep.MissingHeadingIDs)
	}
}

func TestAnalyze_EmptySections(t *testing.T) {
	doc := &parse.Document{Sections: []parse.Section{{HeadingText: "Empty", HeadingLevel: 2, HeadingID: "e", ContentText: ""}}}
	rep := report.Analyze(doc)
	if len(rep.EmptySections) != 1 || rep.EmptySections[0] != "Empty" {
		t.Fatalf("expected empty section 'Empty', got %v", rep.EmptySections)
	}
}
