package report

import (
	"sort"

	"go_scrap/internal/parse"
)

type Report struct {
	MissingHeadingIDs []string `json:"missing_heading_ids"`
	DuplicateIDs      []string `json:"duplicate_ids"`
	BrokenAnchors     []string `json:"broken_anchors"`
	EmptySections     []string `json:"empty_sections"`
	HeadingGaps       []string `json:"heading_gaps"`
}

func Analyze(doc *parse.Document) Report {
	missing := []string{}
	empty := []string{}
	gaps := []string{}

	for _, s := range doc.Sections {
		if s.HeadingID == "" {
			missing = append(missing, s.HeadingText)
		}
		if s.ContentText == "" {
			empty = append(empty, s.HeadingText)
		}
	}

	if len(doc.Sections) > 1 {
		prev := doc.Sections[0].HeadingLevel
		for i := 1; i < len(doc.Sections); i++ {
			cur := doc.Sections[i].HeadingLevel
			if prev > 0 && cur-prev > 1 {
				gaps = append(gaps, doc.Sections[i].HeadingText)
			}
			if cur > 0 {
				prev = cur
			}
		}
	}

	duplicates := findDuplicates(doc.AllElementIDs)
	broken := findBrokenAnchors(doc.AnchorTargets, doc.AllElementIDs)

	sort.Strings(missing)
	sort.Strings(duplicates)
	sort.Strings(broken)
	sort.Strings(empty)
	sort.Strings(gaps)

	return Report{
		MissingHeadingIDs: missing,
		DuplicateIDs:      duplicates,
		BrokenAnchors:     broken,
		EmptySections:     empty,
		HeadingGaps:       gaps,
	}
}

func findDuplicates(ids []string) []string {
	counts := map[string]int{}
	for _, id := range ids {
		if id == "" {
			continue
		}
		counts[id]++
	}
	dups := []string{}
	for id, count := range counts {
		if count > 1 {
			dups = append(dups, id)
		}
	}
	return dups
}

func findBrokenAnchors(anchors []string, ids []string) []string {
	idset := map[string]struct{}{}
	for _, id := range ids {
		if id == "" {
			continue
		}
		idset[id] = struct{}{}
	}
	broken := []string{}
	for _, a := range anchors {
		if a == "" {
			continue
		}
		if _, ok := idset[a]; !ok {
			broken = append(broken, a)
		}
	}
	return broken
}
