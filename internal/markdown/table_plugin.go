package markdown

import (
	"strconv"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/PuerkitoBio/goquery"
)

// TablePlugin returns a plugin that handles tables with rowspan/colspan
// by flattening them into a grid. It repeats content for spanned cells
// to ensure data integrity for RAG ingestion.
func TablePlugin() md.Plugin {
	return func(conv *md.Converter) []md.Rule {
		return []md.Rule{{
			Filter: []string{"table"},
			Replacement: func(content string, selec *goquery.Selection, opt *md.Options) *string {
				grid, ok := buildTableGrid(conv, selec)
				if !ok {
					return nil
				}
				res := renderTable(grid)
				return &res
			},
		}}
	}
}

type tableGrid struct {
	grid     map[int]map[int]string
	maxCol   int
	rowCount int
}

func buildTableGrid(conv *md.Converter, table *goquery.Selection) (tableGrid, bool) {
	rows := table.Find("tr")
	if rows.Length() == 0 {
		return tableGrid{}, false
	}
	grid := tableGrid{grid: map[int]map[int]string{}}
	rows.Each(func(rIdx int, tr *goquery.Selection) {
		grid.rowCount++
		ensureRow(&grid, rIdx)
		fillRow(conv, tr, rIdx, &grid)
	})
	return grid, true
}

func ensureRow(grid *tableGrid, rIdx int) {
	if _, ok := grid.grid[rIdx]; !ok {
		grid.grid[rIdx] = make(map[int]string)
	}
}

func fillRow(conv *md.Converter, tr *goquery.Selection, rIdx int, grid *tableGrid) {
	cIdx := 0
	tr.Children().Filter("td, th").Each(func(_ int, td *goquery.Selection) {
		cIdx = nextFreeCol(grid, rIdx, cIdx)

		cellContent := cleanCell(conv.Convert(td))
		rowSpan := spanValue(td, "rowspan")
		colSpan := spanValue(td, "colspan")

		placeCell(grid, rIdx, cIdx, rowSpan, colSpan, cellContent)
		cIdx += colSpan
	})
}

func nextFreeCol(grid *tableGrid, rIdx, cIdx int) int {
	for {
		if _, occupied := grid.grid[rIdx][cIdx]; !occupied {
			return cIdx
		}
		cIdx++
	}
}

func spanValue(td *goquery.Selection, attr string) int {
	if val, err := strconv.Atoi(td.AttrOr(attr, "1")); err == nil && val > 1 {
		return val
	}
	return 1
}

func placeCell(grid *tableGrid, rIdx, cIdx, rowSpan, colSpan int, cellContent string) {
	for r := 0; r < rowSpan; r++ {
		for c := 0; c < colSpan; c++ {
			targetRow := rIdx + r
			targetCol := cIdx + c

			ensureRow(grid, targetRow)
			grid.grid[targetRow][targetCol] = cellContent
			if targetCol > grid.maxCol {
				grid.maxCol = targetCol
			}
		}
	}
}

func renderTable(grid tableGrid) string {
	var builder strings.Builder
	for r := 0; r < grid.rowCount; r++ {
		writeRow(&builder, grid, r)
		if r == 0 {
			writeSeparator(&builder, grid.maxCol)
		}
	}
	builder.WriteString("\n")
	return builder.String()
}

func writeRow(builder *strings.Builder, grid tableGrid, rIdx int) {
	builder.WriteString("|")
	for c := 0; c <= grid.maxCol; c++ {
		builder.WriteString(" ")
		builder.WriteString(grid.grid[rIdx][c])
		builder.WriteString(" |")
	}
	builder.WriteString("\n")
}

func writeSeparator(builder *strings.Builder, maxCol int) {
	builder.WriteString("|")
	for c := 0; c <= maxCol; c++ {
		builder.WriteString(" --- |")
	}
	builder.WriteString("\n")
}

func cleanCell(text string) string {
	text = strings.TrimSpace(text)
	text = strings.ReplaceAll(text, "|", "\\|") // Escape pipes
	text = strings.ReplaceAll(text, "\n", " ")  // Flatten newlines
	return text
}
