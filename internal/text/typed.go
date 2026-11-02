package text

import (
	"unicode"
	"unicode/utf8"
)

// IsTypedRune reports whether key, a bubbletea key string, is one
// printable rune the user typed into a query or command line: any
// script, not only ASCII. Named keys ("enter", "ctrl+a") and multi-rune
// strings are not.
func IsTypedRune(key string) bool {
	r := []rune(key)
	return len(r) == 1 && unicode.IsPrint(r[0])
}

// DropLastRune returns s without its last rune, so backspace never
// leaves half of a multi-byte character behind. Empty stays empty.
func DropLastRune(s string) string {
	_, n := utf8.DecodeLastRuneInString(s)
	return s[:len(s)-n]
}
