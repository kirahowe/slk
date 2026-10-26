// Package ansi holds the ANSI escape helpers shared by the components in
// internal/bubbles and by internal/ui. It is not a parser: for that, use
// github.com/charmbracelet/x/ansi.
package ansi

import (
	"strings"

	xansi "github.com/charmbracelet/x/ansi"
)

// ReapplyAfterResets re-emits style after every SGR reset in text, so inline
// styled spans don't leak the terminal's defaults through.
//
// style is one or more escape sequences, commonly a background colour or a
// background+foreground pair. Pass only the background to restore it; pass the
// pair as well when plain text following a styled span (e.g. the body after a
// <@user> mention) must keep the theme's text colour. An empty style returns
// text unchanged.
func ReapplyAfterResets(text, style string) string {
	if style == "" {
		return text
	}
	// lipgloss v2 emits the short form, but image renderers and other
	// ANSI producers may emit the explicit zero form.
	text = strings.ReplaceAll(text, "\x1b[0m", "\x1b[0m"+style)
	return strings.ReplaceAll(text, "\x1b[m", "\x1b[m"+style)
}

// ReapplyAfterFgResets re-emits fg after every foreground reset (\x1b[39m)
// in text. A kitty image placement ends in one, because it carries the
// image ID in the foreground colour, so without this the text after an
// inline image emoji takes the terminal's default foreground instead of
// the theme's. ReapplyAfterResets does not cover it: a foreground reset
// is not a full reset. An empty fg returns text unchanged.
func ReapplyAfterFgResets(text, fg string) string {
	if fg == "" {
		return text
	}
	return strings.ReplaceAll(text, "\x1b[39m", "\x1b[39m"+fg)
}

// Graft draws insert over line starting at display column col and
// returns the result: line's cells left of col, then insert, then
// line's cells right of it. The line keeps its own escapes on both
// sides, so a border, gutter or background (a selection tint included)
// survives around the insert.
//
// insert always starts at col and the result is as wide as line, or
// wider when insert runs past its end. Where a wide character
// straddles either edge of insert, its cells left of insert become
// spaces and its cells right of insert become one space, so no cell
// shifts; a col past line's end pads the gap with spaces.
func Graft(line, insert string, col int) string {
	col = max(col, 0)
	w := xansi.StringWidth(line)
	prefix := xansi.Cut(line, 0, col)
	if pw := xansi.StringWidth(prefix); pw < col {
		prefix += strings.Repeat(" ", col-pw)
	}
	end := col + xansi.StringWidth(insert)
	if end >= w {
		return prefix + insert
	}
	// Cut keeps a wide character that starts left of end, which would
	// make the row one cell too wide.
	tail := xansi.Cut(line, end, w)
	if xansi.StringWidth(tail) > w-end {
		tail = " " + xansi.Cut(line, end+1, w)
	}
	return prefix + insert + tail
}
