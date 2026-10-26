package ansi

import (
	"strings"
	"testing"

	xansi "github.com/charmbracelet/x/ansi"
)

// A clipped image's fallback is grafted onto the pane row it replaces,
// so the row keeps its border and gutter on the left and its own
// background on the right: the selection tint, not a plain fill.
func TestGraftKeepsTheRowsSurroundings(t *testing.T) {
	// 20-cell row: border, one gutter cell, then 18 tinted fill cells.
	const tint = "\x1b[48;2;90;90;120m"
	plain := "\x1b[38;2;46;52;64;48;2;46;52;64m"
	row := plain + "▌ " + tint + strings.Repeat(" ", 18) + "\x1b[m"
	fb := strings.Repeat("▀", 10)

	got := Graft(row, fb, 2)

	if w := xansi.StringWidth(got); w != 20 {
		t.Errorf("width = %d, want 20", w)
	}
	if want := "▌ " + fb + strings.Repeat(" ", 8); xansi.Strip(got) != want {
		t.Errorf("row = %q, want %q", xansi.Strip(got), want)
	}
	if tail := got[strings.LastIndex(got, fb)+len(fb):]; !strings.Contains(tail, tint) {
		t.Errorf("the cells after the fallback lost the row's tint: %q", tail)
	}

	// A fallback wider than the rest of the row extends it.
	if w := xansi.StringWidth(Graft(row, strings.Repeat("▀", 30), 2)); w != 32 {
		t.Errorf("wide fallback row width = %d, want 32", w)
	}
}

// A wide character that the insert's edges cut becomes spaces, so the
// insert stays at its column and the row keeps its width.
func TestGraftAroundAWideCharacter(t *testing.T) {
	line := "ab中cd" // 中 is cells 2 and 3
	cases := []struct {
		col  int
		want string
	}{
		{2, "abX cd"}, // the insert covers 中's left cell
		{3, "ab Xcd"}, // and its right cell
		{1, "aX中cd"},  // ends where 中 starts: nothing to cut
	}
	for _, c := range cases {
		got := Graft(line, "X", c.col)
		if got != c.want {
			t.Errorf("Graft(%q, X, %d) = %q, want %q", line, c.col, got, c.want)
		}
	}
	if got := Graft("ab", "X", 4); got != "ab  X" {
		t.Errorf("Graft past the end = %q, want %q", got, "ab  X")
	}
}
