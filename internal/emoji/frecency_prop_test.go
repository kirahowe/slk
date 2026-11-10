package emoji

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// frecencyPool is a few built-in names with several aliases of one
// emoji among them (+1 / thumbsup, laughing / satisfied, hankey / poop
// / shit), so histories and entry lists drawn from it collide often.
var frecencyPool = []string{
	"+1", "thumbsup", "-1", "thumbsdown", "laughing", "satisfied",
	"hankey", "poop", "shit", "tada", "fire", "fireworks", "campfire",
	"wave", "rocket", "smile", "heart", "eyes",
}

// poolCustoms are custom emoji drawn alongside the pool; "shit" shadows
// the standard alias of the same name.
var poolCustoms = map[string]string{"partyparrot": "https://e/p.gif", "shit": "https://e/s.png"}

// historyName is a recorded name: a canonical pool or custom name,
// sometimes toned in Slack's form.
func historyName(customs map[string]string) *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		n := rapid.SampledFrom(append(slices.Clone(frecencyPool), "partyparrot")).Draw(t, "name")
		if rapid.IntRange(0, 3).Draw(t, "toned") == 0 {
			n += "::skin-tone-" + strconv.Itoa(rapid.IntRange(2, 6).Draw(t, "tone"))
		}
		return CanonicalSlackName(n, customs)
	})
}

// drawCustoms is no customs or poolCustoms.
func drawCustoms(t *rapid.T) map[string]string {
	if rapid.Bool().Draw(t, "withCustoms") {
		return poolCustoms
	}
	return nil
}

// drawEntries is some distinct pool and custom names in alphabetical
// order, as BuildEntries lists them.
func drawEntries(t *rapid.T) []EmojiEntry {
	ns := rapid.SliceOfDistinct(rapid.SampledFrom(append(slices.Clone(frecencyPool), "partyparrot")),
		func(s string) string { return s }).Draw(t, "entries")
	slices.Sort(ns)
	return entries(ns...)
}

// Rank is a stable permutation of its candidates that puts every used
// candidate before every unused one: the used ones in non-decreasing
// position, a tie in the order given, the unused ones in the order
// given; and the count it returns is the number of used candidates.
func TestRankIsAStablePermutationUsedFirst(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		customs := drawCustoms(t)
		cands := drawEntries(t)
		f := NewFrecency(rapid.SliceOf(historyName(customs)).Draw(t, "history"))
		got, n := Rank(cands, entryName, f, customs)

		if a, b := slices.Sorted(slices.Values(names(got))), slices.Sorted(slices.Values(names(cands))); !slices.Equal(a, b) {
			t.Fatalf("Rank(%v) = %v: not a permutation", names(cands), names(got))
		}
		index := map[string]int{}
		for i, c := range cands {
			index[c.Name] = i
		}
		for i, e := range got {
			pos, used := f.Position(e.Name, customs)
			if used != (i < n) {
				t.Fatalf("Rank = %v with %d used: %q at %d is used=%v", names(got), n, e.Name, i, used)
			}
			if i == 0 || i == n {
				continue
			}
			prev := got[i-1]
			if used {
				prevPos, _ := f.Position(prev.Name, customs)
				if prevPos > pos || (prevPos == pos && index[prev.Name] > index[e.Name]) {
					t.Fatalf("Rank = %v: %q before %q is out of order", names(got), prev.Name, e.Name)
				}
			} else if index[prev.Name] > index[e.Name] {
				t.Fatalf("Rank = %v: unused %q before %q reverses the input", names(got), prev.Name, e.Name)
			}
		}
	})
}

// A toned use raises exactly its base: recording "<base>::skin-tone-N"
// after a history gives every spelling of that base, and of that toned
// emoji, the new use's position when it had none (and keeps the better
// one it had), and moves no other name.
func TestTonedUseRaisesExactlyItsBase(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		history := rapid.SliceOf(historyName(nil)).Draw(t, "history")
		base := CanonicalSlackName(rapid.SampledFrom(frecencyPool).Draw(t, "base"), nil)
		toned := CanonicalSlackName(base+"::skin-tone-"+strconv.Itoa(rapid.IntRange(2, 6).Draw(t, "tone")), nil)
		before := NewFrecency(history)
		after := NewFrecency(append(slices.Clone(history), toned))

		var spellings []string
		for _, n := range frecencyPool {
			spellings = append(spellings, n)
			for tone := 2; tone <= 6; tone++ {
				spellings = append(spellings, n+"::skin-tone-"+strconv.Itoa(tone))
			}
		}
		for _, n := range spellings {
			p0, ok0 := before.Position(n, nil)
			p1, ok1 := after.Position(n, nil)
			if c := CanonicalSlackName(n, nil); c == base || c == toned {
				want := len(history)
				if ok0 {
					want = p0
				}
				if !ok1 || p1 != want {
					t.Fatalf("after %q, Position(%q) = %d, %v; want %d, true", toned, n, p1, ok1, want)
				}
				continue
			}
			if p0 != p1 || ok0 != ok1 {
				t.Fatalf("after %q, Position(%q) moved from %d, %v to %d, %v", toned, n, p0, ok0, p1, ok1)
			}
		}
	})
}

// Match lists only entries that match the query, the used ones before
// the unused ones and prefix matches before substring matches within
// each, one row per used emoji, and every matching entry unless a row
// for the same emoji is already there; a limit only cuts the tail.
func TestMatchOrdersAndCovers(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		customs := drawCustoms(t)
		all := drawEntries(t)
		f := NewFrecency(rapid.SliceOf(historyName(customs)).Draw(t, "history"))
		q := rapid.SampledFrom([]string{"", "f", "fire", "th", "u", "p", "SH", "zz"}).Draw(t, "query")
		opt := MatchOptions{Substring: rapid.Bool().Draw(t, "substring"), Frecency: f, Customs: customs}
		got := Match(all, entryName, q, opt)

		fq := strings.ToLower(q)
		tier := func(e EmojiEntry) int {
			_, used := f.Position(e.Name, customs)
			t := 0
			if !used {
				t = 2
			}
			if !strings.HasPrefix(e.Name, fq) {
				t++
			}
			return t
		}
		shown := map[int]bool{}
		for i, e := range got {
			if !strings.HasPrefix(e.Name, fq) && (!opt.Substring || !strings.Contains(e.Name, fq)) {
				t.Fatalf("Match(%q) = %v lists %q, which does not match", q, names(got), e.Name)
			}
			if i > 0 && tier(got[i-1]) > tier(e) {
				t.Fatalf("Match(%q) = %v: %q is listed after %q", q, names(got), e.Name, got[i-1].Name)
			}
			if pos, used := f.Position(e.Name, customs); used {
				if shown[pos] {
					t.Fatalf("Match(%q) = %v lists emoji %d twice", q, names(got), pos)
				}
				shown[pos] = true
			}
		}
		for _, e := range all {
			matches := strings.HasPrefix(e.Name, fq) || (opt.Substring && strings.Contains(e.Name, fq))
			pos, used := f.Position(e.Name, customs)
			if matches && !slices.Contains(names(got), e.Name) && (!used || !shown[pos]) {
				t.Fatalf("Match(%q) = %v leaves out %q", q, names(got), e.Name)
			}
		}

		limit := rapid.IntRange(1, 6).Draw(t, "limit")
		opt.Limit = limit
		capped := Match(all, entryName, q, opt)
		if want := got[:min(limit, len(got))]; !slices.Equal(names(capped), names(want)) {
			t.Fatalf("Match(%q, limit %d) = %v, want the first %d of %v", q, limit, names(capped), limit, names(got))
		}
	})
}

// Used gives each emoji once, by a canonical name (Used of its own
// output changes nothing), only for shortcodes that name an emoji, in
// the order the message first uses them.
func TestUsedIsCanonicalKnownAndOrdered(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		customs := drawCustoms(t)
		in := rapid.SliceOf(rapid.OneOf(
			rapid.SampledFrom(frecencyPool),
			rapid.SampledFrom([]string{"partyparrot", "30", "not_an_emoji", "skin-tone-3"}),
			rapid.Custom(func(t *rapid.T) string {
				return rapid.SampledFrom(frecencyPool).Draw(t, "base") + "::skin-tone-" + strconv.Itoa(rapid.IntRange(2, 6).Draw(t, "tone"))
			}),
		)).Draw(t, "shortcodes")
		got := Used(in, customs)

		if again := Used(got, customs); !slices.Equal(again, got) {
			t.Fatalf("Used(%q) = %q, but Used of that = %q", in, got, again)
		}
		last := -1
		for _, name := range got {
			if CanonicalSlackName(name, customs) != name {
				t.Fatalf("Used(%q) = %q: %q is not canonical", in, got, name)
			}
			first := slices.IndexFunc(in, func(sc string) bool { return CanonicalSlackName(sc, customs) == name })
			if first < 0 {
				t.Fatalf("Used(%q) = %q: %q comes from no shortcode", in, got, name)
			}
			if first < last {
				t.Fatalf("Used(%q) = %q: %q is out of first-use order", in, got, name)
			}
			last = first
		}
		for _, sc := range in {
			_, custom := customs[sc]
			_, builtin := slackCodeMap[":"+StripSkinTone(sc)+":"]
			if (custom || builtin) && !slices.Contains(got, CanonicalSlackName(sc, customs)) {
				t.Fatalf("Used(%q) = %q leaves out the emoji %q", in, got, sc)
			}
			if !custom && !builtin && slices.Contains(got, sc) {
				t.Fatalf("Used(%q) = %q keeps %q, which names no emoji", in, got, sc)
			}
		}
	})
}
