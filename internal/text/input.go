package text

import (
	"unicode"
	"unicode/utf8"
)

// Typed returns the text a key press types into a one-line input, key
// being the press as its string (bubbletea's Key.String()), and whether
// it types anything. A press types text when it is one graphic rune in
// any script (a letter, mark, digit, punctuation, symbol, or a space
// separator such as U+3000, the ideographic space a CJK input method
// types), or the space bar, which Key.String() names "space" and which
// types " ". Any other named key ("enter", "backspace", "shift+space")
// or chord ("ctrl+a") types nothing.
func Typed(key string) (string, bool) {
	if key == "space" {
		return " ", true
	}
	r, n := utf8.DecodeRuneInString(key)
	if n == 0 || n != len(key) || (r == utf8.RuneError && n == 1) {
		return "", false // empty, several runes, or not UTF-8
	}
	if !unicode.IsGraphic(r) {
		return "", false
	}
	return key, true
}

// DropLastRune returns s without its last rune, which is what backspace
// removes from a one-line input: the whole UTF-8 sequence, never one
// byte of it. An empty s stays empty.
func DropLastRune(s string) string {
	_, n := utf8.DecodeLastRuneInString(s)
	return s[:len(s)-n]
}
