package ansi

import (
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"
	"pgregory.net/rapid"

	"github.com/gammons/slk/internal/proptest"
)

// graftLine draws one styled line: proptest.StyledText with its
// newlines turned into spaces, clear of the base+VS16 width
// disagreement.
func graftLine() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		return proptest.NoVS16(strings.ReplaceAll(proptest.StyledText().Draw(t, "line"), "\n", " "))
	})
}

// graftInsert draws a styled halfblock row, the shape of an image's
// fallback rows: an SGR, then halfblock and space cells, then a reset.
func graftInsert() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		cells := rapid.SliceOfN(rapid.SampledFrom([]string{"▀", "▄", "█", " "}), 0, 12).Draw(t, "cells")
		return proptest.SGR().Draw(t, "sgr") + strings.Join(cells, "") + "\x1b[m"
	})
}

// Graft puts the insert at exactly col and never moves a cell: the
// result is as wide as the line or as col plus the insert, whichever
// is wider; the cells at col show the insert; the cells left of col
// show the line's (a wide character cut by col turns into spaces);
// and the line's cells right of the insert are still there.
func TestGraftPlacesInsertAtCol(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		line := graftLine().Draw(t, "line")
		insert := graftInsert().Draw(t, "insert")
		w := xansi.StringWidth(line)
		col := rapid.IntRange(0, w+3).Draw(t, "col")
		end := col + xansi.StringWidth(insert)

		got := Graft(line, insert, col)

		if gw := xansi.StringWidth(got); gw != max(w, end) {
			t.Fatalf("width %d, want %d: %q", gw, max(w, end), got)
		}
		if slot := xansi.Strip(xansi.Cut(got, col, end)); slot != xansi.Strip(insert) {
			t.Fatalf("cells %d..%d are %q, want the insert %q", col, end, slot, xansi.Strip(insert))
		}
		left := xansi.Strip(xansi.Cut(got, 0, col))
		want := xansi.Strip(xansi.Cut(line, 0, col))
		if !strings.HasPrefix(left, want) || strings.TrimLeft(left[len(want):], " ") != "" {
			t.Fatalf("left of col is %q, want %q then spaces", left, want)
		}
		if end+1 < w {
			tail := xansi.Strip(xansi.Cut(line, end+1, w))
			if !strings.HasSuffix(xansi.Strip(got), tail) {
				t.Fatalf("result %q lost the line's tail %q", xansi.Strip(got), tail)
			}
		}
	})
}

// An insert as wide as the line, at col 0, replaces it outright.
func TestGraftOverWholeLineIsInsert(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		insert := graftInsert().Draw(t, "insert")
		line := strings.Repeat("x", xansi.StringWidth(insert))
		if got := Graft(line, insert, 0); got != insert {
			t.Fatalf("Graft(%q, %q, 0) = %q, want the insert", line, insert, got)
		}
	})
}
