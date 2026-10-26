// Package ansi holds the ANSI escape helpers shared by the components in
// internal/bubbles and by internal/ui. It is not a parser: for that, use
// github.com/charmbracelet/x/ansi.
package ansi

import "strings"

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
