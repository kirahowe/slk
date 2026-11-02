package compose

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// distinctMaps draws the three mention maps with one name per ID, as a
// workspace has: users and channels named by distinct lowercase words,
// usergroups by handles that may repeat a user's name.
func distinctMaps(t *rapid.T) (names, channels, groups map[string]string) {
	word := rapid.StringMatching(`[a-z]{1,6}`)
	userNames := rapid.SliceOfNDistinct(word, 0, 6, rapid.ID[string]).Draw(t, "users")
	chanNames := rapid.SliceOfNDistinct(word, 0, 6, rapid.ID[string]).Draw(t, "channels")
	handles := rapid.SliceOfNDistinct(word, 0, 4, rapid.ID[string]).Draw(t, "groups")
	names, channels, groups = map[string]string{}, map[string]string{}, map[string]string{}
	for i, n := range userNames {
		names[n] = fmt.Sprintf("U%d", i)
	}
	for i, n := range chanNames {
		channels[n] = fmt.Sprintf("C%d", i)
	}
	for i, h := range handles {
		groups[fmt.Sprintf("S%d", i)] = h
	}
	return
}

var editSeps = []string{" ", ", ", ". ", "\n", "! ", "? ", ": ", "; ", ") "}

// Typed text whose mentions are names the maps hold comes back as typed
// after a send and an edit: mentionsForEdit(translateMentions(s)) == s.
func TestMentionsForEditInvertsTranslate(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		names, channels, groups := distinctMaps(t)
		var pool []string
		for n := range names {
			pool = append(pool, "@"+n)
		}
		for n := range channels {
			pool = append(pool, "#"+n)
		}
		for _, h := range groups {
			pool = append(pool, "@"+h)
		}
		pool = append(pool, "@here", "@channel", "@everyone", "plain", "words")
		slices.Sort(pool)
		parts := rapid.SliceOfN(rapid.SampledFrom(pool), 1, 8).Draw(t, "parts")
		var b strings.Builder
		for i, p := range parts {
			if i > 0 {
				b.WriteString(rapid.SampledFrom(editSeps).Draw(t, "sep"))
			}
			b.WriteString(p)
		}
		typed := b.String()
		wire := translateMentions(typed, names, channels, groups)
		if got := mentionsForEdit(wire, names, channels, groups); got != typed {
			t.Fatalf("typed %q\nwire  %q\nedit  %q", typed, wire, got)
		}
	})
}
