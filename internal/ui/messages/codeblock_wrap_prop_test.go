package messages

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"pgregory.net/rapid"
)

// A fenced block's line is cut only at the block's width, and a tab
// counts as the four columns it is drawn in. So a line of tabs and one
// unbroken word takes ceil(drawn width / text width) rows, every row is
// exactly the block's width, and the word comes out whole and in order.
func TestCodeBlockCutsALineOnlyAtTheWidth(t *testing.T) {
	const tabWidth = 4
	rapid.Check(t, func(t *rapid.T) {
		tabs := rapid.IntRange(0, 4).Draw(t, "tabs")
		word := rapid.StringMatching(`[a-z0-9=(){}.,]{1,90}`).Draw(t, "word")
		// The text width is the block's less a padding column each side.
		// Wide enough that the indentation leaves room for the word on
		// the first row: Hardwrap drops spaces that start a cut row.
		width := rapid.IntRange(tabs*tabWidth+3, 60).Draw(t, "width")
		textWidth := width - 2

		out := RenderSlackMarkdownWith("```\n"+strings.Repeat("\t", tabs)+word+"\n```",
			RenderSlackMarkdownOpts{Width: width})
		var rows []string
		for _, l := range strings.Split(WordWrap(out, width), "\n") {
			if strings.TrimSpace(ansi.Strip(l)) == "" {
				continue
			}
			if w := ansi.StringWidth(l); w != width {
				t.Fatalf("row %q is %d wide, want %d", ansi.Strip(l), w, width)
			}
			rows = append(rows, ansi.Strip(l))
		}

		drawn := tabs*tabWidth + len(word)
		if want := (drawn + textWidth - 1) / textWidth; len(rows) != want {
			t.Fatalf("%d columns in a text width of %d took %d rows, want %d:\n%q",
				drawn, textWidth, len(rows), want, rows)
		}
		if got := noSpace(strings.Join(rows, "")); got != word {
			t.Fatalf("rows hold %q, want %q", got, word)
		}
	})
}
