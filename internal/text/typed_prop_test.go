package text

import (
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/gammons/slk/internal/proptest"
	"pgregory.net/rapid"
)

// Typing a printable rune and then backspacing gives back the query
// as it was, and the query stays valid UTF-8 throughout.
func TestDropLastRuneUndoesATypedRune(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		query := proptest.PlainText().Draw(t, "query")
		r := rapid.Rune().Filter(unicode.IsPrint).Draw(t, "rune")
		typed := string(r)
		if !IsTypedRune(typed) {
			t.Fatalf("IsTypedRune(%q) = false for a printable rune", typed)
		}
		if got := DropLastRune(query + typed); got != query {
			t.Fatalf("DropLastRune(%q) = %q, want %q", query+typed, got, query)
		}
		if !utf8.ValidString(DropLastRune(query)) {
			t.Fatalf("DropLastRune(%q) is not valid UTF-8", query)
		}
	})
}

// A key string is a typed rune exactly when it is one printable rune;
// named keys such as "enter" and "ctrl+a" never are.
func TestIsTypedRuneIsOnePrintableRune(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		key := rapid.StringN(0, 6, -1).Draw(t, "key")
		rs := []rune(key)
		want := len(rs) == 1 && unicode.IsPrint(rs[0])
		if got := IsTypedRune(key); got != want {
			t.Fatalf("IsTypedRune(%q) = %v, want %v", key, got, want)
		}
	})
	for _, k := range []string{"enter", "ctrl+a", "backspace", "space", ""} {
		if IsTypedRune(k) {
			t.Errorf("IsTypedRune(%q) = true", k)
		}
	}
}
