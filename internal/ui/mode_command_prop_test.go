package ui

import (
	"slices"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// completionNames draws a set of distinct command names from a small
// alphabet, so that many of them share prefixes and a Tab has real
// choices to cycle through.
func completionNames(t *rapid.T) []string {
	return rapid.SliceOfDistinct(rapid.StringMatching(`[a-c]{1,4}`), rapid.ID[string]).Draw(t, "names")
}

// completionLine draws a buffer: usually a prefix of one of the names,
// sometimes any short string, which may hold a space.
func completionLine(t *rapid.T, names []string) string {
	if len(names) > 0 && rapid.Bool().Draw(t, "fromName") {
		name := rapid.SampledFrom(names).Draw(t, "name")
		return name[:rapid.IntRange(0, len(name)).Draw(t, "cut")]
	}
	return rapid.StringMatching(`[a-c ]{0,4}`).Draw(t, "line")
}

// TestNextCompletionStartsFromTheTypedPrefix: a Tab on a line with no
// cycle in progress completes nothing when the line holds a space or no
// name starts with it. Otherwise the cycle it starts lists exactly the
// names that start with the line, sorted, and sits on the first of them
// (the last going backwards).
func TestNextCompletionStartsFromTheTypedPrefix(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		names := completionNames(t)
		line := completionLine(t, names)
		backwards := rapid.Bool().Draw(t, "backwards")

		got := nextCompletion(cmdCompletion{}, line, backwards, names)

		var want []string
		for _, n := range names {
			if strings.HasPrefix(n, line) {
				want = append(want, n)
			}
		}
		slices.Sort(want)
		if strings.Contains(line, " ") || len(want) == 0 {
			if !cmdCompletionZero(got) {
				t.Fatalf("line %q: got cycle %+v, want none", line, got)
			}
			return
		}
		if !slices.Equal(got.matches, want) {
			t.Fatalf("line %q: matches = %q, want %q", line, got.matches, want)
		}
		wantIdx := 0
		if backwards {
			wantIdx = len(want) - 1
		}
		if got.idx != wantIdx {
			t.Fatalf("line %q: idx = %d, want %d", line, got.idx, wantIdx)
		}
	})
}

// TestNextCompletionCyclesEveryMatchOnce: pressing Tab in one direction
// as many times as there are matches shows each match exactly once and
// comes back to where it started, and a press in the other direction
// undoes a press.
func TestNextCompletionCyclesEveryMatchOnce(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		names := completionNames(t)
		line := completionLine(t, names)
		backwards := rapid.Bool().Draw(t, "backwards")

		c := nextCompletion(cmdCompletion{}, line, backwards, names)
		if cmdCompletionZero(c) {
			return
		}
		first := c.matches[c.idx]
		seen := map[string]bool{}
		for range len(c.matches) {
			shown := c.matches[c.idx]
			if seen[shown] {
				t.Fatalf("%q shown twice in one cycle of %q", shown, c.matches)
			}
			seen[shown] = true

			next := nextCompletion(c, shown, backwards, names)
			back := nextCompletion(next, next.matches[next.idx], !backwards, names)
			if got := back.matches[back.idx]; got != shown {
				t.Fatalf("Tab then reverse Tab from %q landed on %q", shown, got)
			}
			c = next
		}
		if got := c.matches[c.idx]; got != first {
			t.Fatalf("after a full cycle the line is %q, want %q", got, first)
		}
	})
}

// TestNextCompletionRestartsOnAnEditedLine: a cycle whose candidate is
// no longer the buffer (the user typed or deleted) is dropped, and the
// Tab acts exactly as it would on a fresh line.
func TestNextCompletionRestartsOnAnEditedLine(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		names := completionNames(t)
		backwards := rapid.Bool().Draw(t, "backwards")
		stale := nextCompletion(cmdCompletion{}, completionLine(t, names), backwards, names)
		line := completionLine(t, names)
		if !cmdCompletionZero(stale) && stale.matches[stale.idx] == line {
			return // still on its candidate: a continuing cycle, not an edit
		}
		got := nextCompletion(stale, line, backwards, names)
		want := nextCompletion(cmdCompletion{}, line, backwards, names)
		if !slices.Equal(got.matches, want.matches) || got.idx != want.idx {
			t.Fatalf("line %q after stale cycle %+v: got %+v, want %+v", line, stale, got, want)
		}
	})
}
