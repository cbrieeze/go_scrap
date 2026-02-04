package app

import (
	"fmt"
	"sort"
	"strings"

	"go_scrap/internal/parse"
	"go_scrap/internal/report"
)

func printSummaryIfNeeded(opts Options, sourceInfo string, doc *parse.Document, rep report.Report) {
	if opts.Stdout {
		return
	}
	printSummary(sourceInfo, doc, rep)
}

func printSummary(sourceInfo string, doc *parse.Document, rep report.Report) {
	headingIDs := unique(doc.HeadingIDs)
	anchorTargets := unique(doc.AnchorTargets)

	fmt.Printf("Fetch mode: %s\n", sourceInfo)
	fmt.Printf("Sections found: %d\n", len(doc.Sections))

	fmt.Println("Heading IDs:")
	printList(headingIDs)

	fmt.Println("Anchor targets (from href=\"#...\"):")
	printList(anchorTargets)

	if reportHasIssues(rep) {
		fmt.Println("\nCompleteness report:")
		fmt.Printf("  missing heading ids: %d\n", len(rep.MissingHeadingIDs))
		fmt.Printf("  duplicate ids: %d\n", len(rep.DuplicateIDs))
		fmt.Printf("  broken anchors: %d\n", len(rep.BrokenAnchors))
		fmt.Printf("  empty sections: %d\n", len(rep.EmptySections))
		fmt.Printf("  heading gaps: %d\n", len(rep.HeadingGaps))
	}
}

func printList(items []string) {
	if len(items) == 0 {
		fmt.Println("  (none)")
		return
	}
	for _, item := range items {
		fmt.Printf("  - %s\n", item)
	}
}

func unique(list []string) []string {
	set := map[string]struct{}{}
	out := []string{}
	for _, v := range list {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := set[v]; ok {
			continue
		}
		set[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func reportHasIssues(rep report.Report) bool {
	return len(rep.MissingHeadingIDs) > 0 ||
		len(rep.DuplicateIDs) > 0 ||
		len(rep.BrokenAnchors) > 0 ||
		len(rep.EmptySections) > 0 ||
		len(rep.HeadingGaps) > 0
}
