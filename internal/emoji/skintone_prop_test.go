package emoji

import (
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"pgregory.net/rapid"
)

// toneFreeName is a shortcode name with no underscore, so it can never
// read as a kyokomi "_toneN" suffix on its own.
func toneFreeName() *rapid.Generator[string] {
	return rapid.StringMatching(`[a-z][a-z0-9+-]{0,6}`)
}

// tonedShortcode draws one skin-toned shortcode in any of the three
// spellings (Slack one tone, Slack tone per person, kyokomi), returning
// the whole ":...:" form and its base name.
func tonedShortcode(t *rapid.T) (full, base string) {
	base = toneFreeName().Draw(t, "base")
	d := func(label string) string {
		return strconv.Itoa(rapid.IntRange(2, 6).Draw(t, label))
	}
	switch rapid.IntRange(0, 2).Draw(t, "form") {
	case 0:
		return ":" + base + "::skin-tone-" + d("tone") + ":", base
	case 1:
		return ":" + base + "::skin-tone-" + d("a") + "-" + d("b") + ":", base
	}
	return ":" + base + "_tone" + strconv.Itoa(rapid.IntRange(1, 5).Draw(t, "k")) + ":", base
}

// StripSkinToneFromText turns every toned shortcode into its bare base
// and touches nothing else: a run of shortcodes with and without tones
// comes out as the same run with each tone removed, so an emoji after a
// toned one survives and a plain shortcode is never changed.
func TestStripSkinToneFromTextKeepsNeighbours(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		var in, want strings.Builder
		for range rapid.IntRange(0, 6).Draw(t, "n") {
			sep := rapid.SampledFrom([]string{"", " ", "12:30", "hi "}).Draw(t, "sep")
			in.WriteString(sep)
			want.WriteString(sep)
			if rapid.Bool().Draw(t, "toned") {
				full, base := tonedShortcode(t)
				in.WriteString(full)
				want.WriteString(":" + base + ":")
			} else {
				plain := ":" + toneFreeName().Draw(t, "plain") + ":"
				in.WriteString(plain)
				want.WriteString(plain)
			}
		}
		got := StripSkinToneFromText(in.String())
		if got != want.String() {
			t.Fatalf("StripSkinToneFromText(%q) = %q, want %q", in.String(), got, want.String())
		}
		if strings.Contains(got, "skin-tone") || strings.Contains(got, "_tone") {
			t.Fatalf("tone left in %q", got)
		}
	})
}

// A text with no toned shortcode in it comes back unchanged.
func TestStripSkinToneFromTextLeavesPlainShortcodes(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		var b strings.Builder
		for range rapid.IntRange(0, 6).Draw(t, "n") {
			b.WriteString(rapid.SampledFrom([]string{" ", "a", "12:30", "::"}).Draw(t, "w"))
			b.WriteString(":" + toneFreeName().Draw(t, "name") + ":")
		}
		in := b.String()
		if got := StripSkinToneFromText(in); got != in {
			t.Fatalf("changed %q to %q", in, got)
		}
	})
}

func FuzzStripSkinToneFromText(f *testing.F) {
	for _, s := range []string{
		"", ":+1::skin-tone-5::tada:", ":handshake::skin-tone-2-5::tada:",
		":thumbsup_tone3::tada:", "Meet at 12:30:+1::skin-tone-5:", "::skin-tone-2:", ":::",
	} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if !utf8.ValidString(s) {
			t.Skip() // KNOWN: invalid bytes become U+FFFD
		}
		once := StripSkinToneFromText(s)
		if len(once) > len(s) {
			t.Fatalf("grew: %q -> %q", s, once)
		}
		if again := StripSkinToneFromText(once); again != once {
			t.Fatalf("not idempotent: %q -> %q -> %q", s, once, again)
		}
	})
}

// Slack's tone numbers 2-6 and the modifier codepoints are inverse, and
// no other digit maps to a tone.
func TestSlackSkinToneMapping(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		digit := rapid.IntRange(0, 9).Draw(t, "digit")
		modifier, ok := slackSkinTone(byte('0' + digit))
		if ok != (digit >= 2 && digit <= 6) {
			t.Fatalf("slackSkinTone(%d) ok = %v", digit, ok)
		}
		if !ok {
			return
		}
		if n, ok := SlackSkinToneNumber(modifier); !ok || n != digit {
			t.Fatalf("SlackSkinToneNumber(slackSkinTone(%d)) = %d, %v", digit, n, ok)
		}
	})
}

// The Slack and kyokomi spellings of one tone split to the same base
// and modifier: kyokomi's N is Slack's N+1.
func TestSplitSkinToneSuffixSpellingsAgree(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		base := toneFreeName().Draw(t, "base")
		k := rapid.IntRange(1, 5).Draw(t, "k")
		b1, m1, ok1 := splitSkinToneSuffix(base + "_tone" + strconv.Itoa(k))
		b2, m2, ok2 := splitSkinToneSuffix(base + "::skin-tone-" + strconv.Itoa(k+1))
		if !ok1 || !ok2 || b1 != base || b2 != base || m1 != m2 {
			t.Fatalf("kyokomi %d: (%q %U %v) vs slack %d: (%q %U %v)", k, b1, m1, ok1, k+1, b2, m2, ok2)
		}
	})
}

// tonePerPerson accepts exactly "skin-tone-A-B" with A and B in 2-6.
func TestTonePerPersonParse(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		a := rapid.IntRange(0, 9).Draw(t, "a")
		b := rapid.IntRange(0, 9).Draw(t, "b")
		first, second, ok := tonePerPerson("skin-tone-" + strconv.Itoa(a) + "-" + strconv.Itoa(b))
		valid := a >= 2 && a <= 6 && b >= 2 && b <= 6
		if ok != valid {
			t.Fatalf("tonePerPerson(%d-%d) ok = %v", a, b, ok)
		}
		if ok {
			na, _ := SlackSkinToneNumber(first)
			nb, _ := SlackSkinToneNumber(second)
			if na != a || nb != b {
				t.Fatalf("%d-%d parsed as %d-%d", a, b, na, nb)
			}
		}
	})
}
