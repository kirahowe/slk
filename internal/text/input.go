package text

import (
	"unicode"
	"unicode/utf8"
)

// TypedRune reports whether key, a key press as its string, types one
// character into a one-line input: a single printable rune in any
// script (a letter, mark, digit, punctuation, symbol or the space). A
// named key ("enter", "backspace") or a chord ("ctrl+a") is several
// runes and is not typed text.
func TypedRune(key string) bool {
	r, n := utf8.DecodeRuneInString(key)
	if n == 0 || n != len(key) || (r == utf8.RuneError && n == 1) {
		return false // empty, several runes, or not UTF-8
	}
	return unicode.IsPrint(r)
}

// DropLastRune returns s without its last rune, which is what backspace
// removes from a one-line input: the whole UTF-8 sequence, never one
// byte of it. An empty s stays empty.
func DropLastRune(s string) string {
	_, n := utf8.DecodeLastRuneInString(s)
	return s[:len(s)-n]
}
