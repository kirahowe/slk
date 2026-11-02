package compose

import (
	"fmt"
	"regexp"
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

// strictMentionRe is wireMentionRe with no "<" allowed in a label, so a
// token written inside another one's label is found on its own.
var strictMentionRe = regexp.MustCompile(strings.ReplaceAll(wireMentionRe.String(), `[^>]*`, `[^<>]*`))

// mentionIDs lists the mentions wire text carries, in order, as
// "@U1", "#C1", "!here", "^S1".
func mentionIDs(wire string) []string {
	var out []string
	for _, m := range strictMentionRe.FindAllStringSubmatch(wire, -1) {
		switch {
		case m[1] != "":
			out = append(out, "@"+m[1])
		case m[2] != "":
			out = append(out, "#"+m[2])
		case m[4] != "":
			out = append(out, "!"+m[4])
		case m[5] != "":
			out = append(out, "^"+m[5])
		}
	}
	return out
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

// Saving an edit without touching it sends the mentions the message
// had: translateMentions(mentionsForEdit(w)) mentions what w does,
// whatever mix of known, unknown, labelled, stale or glued tokens w
// holds. Text with no token is left alone.
func TestMentionsForEditKeepsMentions(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		names, channels, groups := distinctMaps(t)
		known := func(m map[string]string) []string {
			ids := []string{"X9"}
			for _, id := range m {
				ids = append(ids, id)
			}
			slices.Sort(ids)
			return ids
		}
		userIDs := append(known(names), "U9")
		chanIDs := append(known(channels), "C9")
		var groupIDs []string
		for id := range groups {
			groupIDs = append(groupIDs, id)
		}
		groupIDs = append(groupIDs, "S9")
		slices.Sort(groupIDs)
		// Labels are often a name the maps hold, under another ID.
		var labels []string
		for n := range names {
			labels = append(labels, n)
		}
		for n := range channels {
			labels = append(labels, n)
		}
		slices.Sort(labels)
		label := rapid.Custom(func(t *rapid.T) string {
			if len(labels) > 0 && rapid.Bool().Draw(t, "knownLabel") {
				return rapid.SampledFrom(labels).Draw(t, "known")
			}
			return rapid.StringMatching(`[a-z]{0,6}`).Draw(t, "other")
		})

		token := rapid.Custom(func(t *rapid.T) string {
			switch rapid.IntRange(0, 5).Draw(t, "kind") {
			case 0:
				return "<@" + rapid.SampledFrom(userIDs).Draw(t, "user") + ">"
			case 1:
				id := rapid.SampledFrom(chanIDs).Draw(t, "channel")
				if l := label.Draw(t, "label"); l != "" {
					return "<#" + id + "|" + l + ">"
				}
				return "<#" + id + ">"
			case 2:
				return "<!" + rapid.SampledFrom([]string{"here", "channel", "everyone"}).Draw(t, "special") + ">"
			case 3:
				id := rapid.SampledFrom(groupIDs).Draw(t, "group")
				return "<!subteam^" + id + "|@" + label.Draw(t, "handle") + ">"
			case 4:
				return rapid.StringMatching(`[a-z]{1,5}`).Draw(t, "word")
			default:
				return rapid.SampledFrom(append([]string{"-", ".x", "_", "'"}, editSeps...)).Draw(t, "glue")
			}
		})
		w := strings.Join(rapid.SliceOfN(token, 0, 10).Draw(t, "tokens"), "")
		edit := mentionsForEdit(w, names, channels, groups)
		if !strings.Contains(w, "<") && edit != w {
			t.Fatalf("mentionsForEdit(%q) = %q", w, edit)
		}
		sent := translateMentions(edit, names, channels, groups)
		if got, want := mentionIDs(sent), mentionIDs(w); !slices.Equal(got, want) {
			t.Fatalf("w     %q\nedit  %q\nsent  %q\nmentions %v, want %v", w, edit, sent, got, want)
		}
	})
}
