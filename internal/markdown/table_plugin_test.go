package markdown_test

import (
	"strings"
	"testing"

	md "github.com/JohannesKaufmann/html-to-markdown"

	"go_scrap/internal/markdown"
)

func TestTablePlugin_FlattensRowspanAndColspan(t *testing.T) {
	conv := md.NewConverter("", true, nil)
	conv.Use(markdown.TablePlugin())

	html := `
<table>
  <tr><th>A</th><th>B</th></tr>
  <tr><td rowspan="2">R</td><td>1</td></tr>
  <tr><td>2</td></tr>
  <tr><td colspan="2">X</td></tr>
</table>`

	out, err := conv.ConvertString(html)
	if err != nil {
		t.Fatalf("ConvertString error: %v", err)
	}

	// Basic shape
	if !strings.Contains(out, "| A | B |") {
		t.Fatalf("missing header row, got:\n%s", out)
	}
	if !strings.Contains(out, "| --- | --- |") {
		t.Fatalf("missing separator row, got:\n%s", out)
	}

	// Rowspan repeats "R" in two rows.
	if strings.Count(out, "| R |") < 2 {
		t.Fatalf("expected rowspan value to repeat, got:\n%s", out)
	}

	// Colspan repeats "X" in both columns on that row.
	if !strings.Contains(out, "| X | X |") {
		t.Fatalf("expected colspan value to fill both columns, got:\n%s", out)
	}
}
