package text

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/gammons/slk/internal/proptest"
	"pgregory.net/rapid"
)

// Typing a key and pressing backspace gives back the input as it was,
// for every key Typed accepts (the space bar included) and any input,
// and backspace never leaves half a UTF-8 sequence behind.
func TestTypedThenDropLastRune(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		query := proptest.PlainText().Draw(t, "query")
		key := rapid.OneOf(
			rapid.Just("space"),
			rapid.Map(rapid.Rune(), func(r rune) string { return string(r) }),
		).Draw(t, "key")
		typed, ok := Typed(key)
		if !ok {
			t.Skip("not a typed character")
		}
		if got := DropLastRune(query + typed); got != query {
			t.Fatalf("DropLastRune(%q + %q) = %q, want the input back", query, typed, got)
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

// Typed accepts exactly the space bar, as " ", and the keys that are
// one graphic rune, as themselves (every space separator among them,
// the ideographic space too), so any other key name or a chord, which
// are several runes, never types text.
func TestTypedIsTheSpaceBarOrOneGraphicRune(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		key := rapid.OneOf(
			rapid.SampledFrom([]string{"", "space", " ", "\u3000", "\u00a0", "shift+space", "ctrl+space",
				"enter", "backspace", "tab", "\t", "ctrl+a", "shift+tab", "\x1b", "\x00", "\xff"}),
			rapid.Map(rapid.Rune(), func(r rune) string { return string(r) }),
			proptest.Word(),
		).Draw(t, "key")
		runes := []rune(key)
		want, wantOK := "", false
		switch {
		case key == "space":
			want, wantOK = " ", true
		case utf8.ValidString(key) && len(runes) == 1 && (unicode.IsPrint(runes[0]) || unicode.Is(unicode.Zs, runes[0])):
			want, wantOK = key, true
		}
		if got, ok := Typed(key); got != want || ok != wantOK {
			t.Fatalf("Typed(%q) = %q, %v; want %q, %v", key, got, ok, want, wantOK)
		}
	})
}

// The space bar and the ideographic space both type a space.
func TestTypedSpaces(t *testing.T) {
	for key, want := range map[string]string{"space": " ", " ": " ", "\u3000": "\u3000"} {
		if got, ok := Typed(key); !ok || got != want {
			t.Errorf("Typed(%q) = %q, %v; want %q, true", key, got, ok, want)
		}
	}
}

func TestDropLastRuneOfEmpty(t *testing.T) {
	if got := DropLastRune(""); got != "" {
		t.Fatalf("DropLastRune(\"\") = %q, want empty", got)
	}
}
