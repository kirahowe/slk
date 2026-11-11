package emoji

import (
	"testing"

	"pgregory.net/rapid"
)

// When a picker's list changes under a new history or new entries,
// with the query unchanged, FollowSelection keeps the highlight on the
// emoji name it was on whenever that name is still listed; otherwise
// it is the old index clamped into the new list.
func TestFollowSelectionKeepsTheHighlightedName(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		customs := drawCustoms(t)
		q := rapid.SampledFrom([]string{"", "f", "fire", "th", "u", "p", "SH", "zz"}).Draw(t, "query")
		opt := MatchOptions{
			Substring: rapid.Bool().Draw(t, "substring"),
			Limit:     rapid.IntRange(0, 6).Draw(t, "limit"),
			Frecency:  NewFrecency(rapid.SliceOf(historyName(customs)).Draw(t, "history")),
			Customs:   customs,
		}
		all := drawEntries(t)
		before := Match(all, entryName, q, opt)
		selected := rapid.IntRange(0, max(len(before)-1, 0)).Draw(t, "selected")

		opt.Frecency = NewFrecency(rapid.SliceOf(historyName(customs)).Draw(t, "newHistory"))
		if rapid.Bool().Draw(t, "newEntries") {
			all = drawEntries(t)
		}
		after := Match(all, entryName, q, opt)
		got := FollowSelection(before, selected, after, entryName)

		if len(after) == 0 {
			if got != 0 {
				t.Fatalf("FollowSelection into an empty list = %d, want 0", got)
			}
			return
		}
		if got < 0 || got >= len(after) {
			t.Fatalf("FollowSelection = %d, outside the %d listed", got, len(after))
		}
		if selected < len(before) {
			want := before[selected].Name
			for _, e := range after {
				if e.Name == want {
					if after[got].Name != want {
						t.Fatalf("highlight moved from %q to %q in %v", want, after[got].Name, names(after))
					}
					return
				}
			}
		}
		if want := min(selected, len(after)-1); got != want {
			t.Fatalf("FollowSelection = %d with the name gone, want %d clamped", got, want)
		}
	})
}
