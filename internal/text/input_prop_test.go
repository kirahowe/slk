package text

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/gammons/slk/internal/proptest"
	"pgregory.net/rapid"
)

// Typing a character and pressing backspace gives back the input as it
// was, for every character TypedRune accepts and any input, and
// backspace never leaves half a UTF-8 sequence behind.
func TestTypedRuneThenDropLastRune(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		query := proptest.PlainText().Draw(t, "query")
		key := string(rapid.Rune().Draw(t, "rune"))
		if !TypedRune(key) {
			t.Skip("not a typed character")
		}
		if got := DropLastRune(query + key); got != query {
			t.Fatalf("DropLastRune(%q + %q) = %q, want the input back", query, key, got)
		}
		for s := query; s != ""; {
			next := DropLastRune(s)
			if !utf8.ValidString(next) || !strings.HasPrefix(s, next) || utf8.RuneCountInString(next) != utf8.RuneCountInString(s)-1 {
				t.Fatalf("DropLastRune(%q) = %q, want it one whole rune shorter", s, next)
			}
			s = next
		}
	})
}

// TypedRune accepts exactly the keys that are one printable rune, so a
// key's name or a chord, which are several runes, is never typed text.
func TestTypedRuneIsOnePrintableRune(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		key := rapid.OneOf(
			rapid.SampledFrom([]string{"", "enter", "backspace", "space", "ctrl+a", "shift+tab", "\x1b", "\x00", "\xff"}),
			rapid.Map(rapid.Rune(), func(r rune) string { return string(r) }),
			proptest.Word(),
		).Draw(t, "key")
		runes := []rune(key)
		want := utf8.ValidString(key) && len(runes) == 1 && unicode.IsPrint(runes[0])
		if got := TypedRune(key); got != want {
			t.Fatalf("TypedRune(%q) = %v, want %v", key, got, want)
		}
	})
}

func TestDropLastRuneOfEmpty(t *testing.T) {
	if got := DropLastRune(""); got != "" {
		t.Fatalf("DropLastRune(\"\") = %q, want empty", got)
	}
}
