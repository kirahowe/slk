package canvasview

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"pgregory.net/rapid"

	"github.com/gammons/slk/internal/canvas"
	"github.com/gammons/slk/internal/proptest"
)

// spanText is text the parser could produce: no newlines or controls.
func spanText() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		s := proptest.NoVS16(proptest.PlainText().Draw(t, "text"))
		return strings.ReplaceAll(s, "\n", " ")
	})
}

func genSpan() *rapid.Generator[canvas.Span] {
	return rapid.Custom(func(t *rapid.T) canvas.Span {
		s := canvas.Span{
			Text:   spanText().Draw(t, "text"),
			Bold:   rapid.Bool().Draw(t, "bold"),
			Italic: rapid.Bool().Draw(t, "italic"),
			Code:   rapid.Bool().Draw(t, "code"),
		}
		if rapid.Bool().Draw(t, "link") {
			s.Link = "https://example.com/" + proptest.Word().Draw(t, "path")
		}
		switch rapid.IntRange(0, 4).Draw(t, "chip") {
		case 0:
			s.Chip = canvas.DateChip
		case 1:
			s.Chip, s.ID = canvas.UserChip, proptest.ID("U").Draw(t, "id")
			s.Text = "@" + s.ID
		}
		return s
	})
}

func genBlock() *rapid.Generator[canvas.Block] {
	return rapid.Custom(func(t *rapid.T) canvas.Block {
		b := canvas.Block{
			Kind:  canvas.BlockKind(rapid.IntRange(0, int(canvas.Code)).Draw(t, "kind")),
			Spans: rapid.SliceOfN(genSpan(), 0, 4).Draw(t, "spans"),
		}
		switch b.Kind {
		case canvas.Heading:
			b.Level = rapid.IntRange(1, 3).Draw(t, "level")
		case canvas.ListItem:
			b.List = canvas.ListKind(rapid.IntRange(0, 1).Draw(t, "list"))
			b.Depth = rapid.IntRange(0, 3).Draw(t, "depth")
			b.Number = rapid.IntRange(1, 99).Draw(t, "number")
			b.Continued = rapid.Bool().Draw(t, "continued")
		}
		return b
	})
}

// Every line Render draws is exactly the width it was given, so the
// overlay's box never wraps or ragged-edges.
func TestRenderFillsWidth(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		doc := canvas.Document{Blocks: rapid.SliceOfN(genBlock(), 0, 8).Draw(t, "blocks")}
		width := rapid.IntRange(20, 100).Draw(t, "width")
		for i, line := range Render(doc, width, noNames) {
			if w := lipgloss.Width(line); w != width {
				t.Fatalf("line %d is %d wide, want %d: %q", i, w, width, line)
			}
		}
	})
}

// Wrapping keeps a block's text: its words, in order, with only the
// date glyph added. (Code is hard-wrapped, so it may split a word; its
// characters are compared with every space removed.)
func TestWrapBlockKeepsText(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		b := genBlock().Draw(t, "block")
		width := rapid.IntRange(1, 60).Draw(t, "width")
		var want strings.Builder
		for _, s := range b.Spans {
			want.WriteString(s.Text + " ")
		}
		got := xansi.Strip(strings.Join(wrapBlock(b, width, noNames), " "))
		got = strings.ReplaceAll(got, "📅", "")
		squash := func(s string) string { return strings.Join(strings.Fields(s), "") }
		if squash(got) != squash(want.String()) {
			t.Fatalf("text changed\n got: %q\nwant: %q", squash(got), squash(want.String()))
		}
	})
}

func noNames(string) (string, bool) { return "", false }
