package emoji

import (
	"slices"
	"strings"

	slktext "github.com/gammons/slk/internal/text"
)

// Frecency ranks the emoji the user has used, most frecent first, in
// the order the frecent_emoji cache table returns them (use count
// discounted by days since last use). Names are as recorded: Slack's
// canonical short name (CanonicalSlackName). The zero value ranks
// nothing. A Frecency is never changed once built, so the pickers that
// hold one can share it.
type Frecency struct {
	pos map[string]int
}

// NewFrecency builds a Frecency from names in most-to-least frecent
// order. A repeated name keeps its first (better) position. A
// skin-toned name ("+1::skin-tone-4") also ranks its base emoji ("+1")
// unless the base ranks better on its own: the pickers list base names
// only, so the base is the row a toned use lifts. Only Slack's
// "::skin-tone-N" form, the one CanonicalSlackName records, counts as
// toned; a name ending "_tone1" may be a custom emoji of its own.
func NewFrecency(names []string) Frecency {
	f := Frecency{pos: make(map[string]int, len(names))}
	add := func(k string, i int) {
		if _, dup := f.pos[k]; !dup && k != "" {
			f.pos[k] = i
		}
	}
	for i, n := range names {
		add(n, i)
		if j := strings.Index(n, "::skin-tone-"); j > 0 {
			add(n[:j], i)
		}
	}
	return f
}

// Position returns where name stands in the history (0 is the most
// frecent) and whether it is there at all. name may be any alias or
// skin-tone spelling of a recorded emoji ("thumbsup" finds a recorded
// "+1", "+1_tone2" a recorded "+1::skin-tone-3"). customs is the
// workspace's custom emoji map, which shadows standard names as it
// does in CanonicalSlackName.
func (f Frecency) Position(name string, customs map[string]string) (int, bool) {
	if len(f.pos) == 0 {
		return 0, false
	}
	if p, ok := f.pos[name]; ok {
		return p, true
	}
	p, ok := f.pos[CanonicalSlackName(name, customs)]
	return p, ok
}

// Rank orders candidates the way a picker lists them: the ones the
// user has used first, most frecent first, then the rest in the order
// given. The sort is stable, so candidates at the same position (two
// aliases of one emoji) keep their order. It returns the ranked slice,
// a new one, and how many of its entries are used emoji. name reads a
// candidate's emoji name; customs is as for Frecency.Position.
func Rank[E any](candidates []E, name func(E) string, f Frecency, customs map[string]string) ([]E, int) {
	type used struct {
		e   E
		pos int
	}
	var hits []used
	rest := make([]E, 0, len(candidates))
	for _, c := range candidates {
		if p, ok := f.Position(name(c), customs); ok {
			hits = append(hits, used{c, p})
			continue
		}
		rest = append(rest, c)
	}
	slices.SortStableFunc(hits, func(a, b used) int { return a.pos - b.pos })
	out := make([]E, 0, len(candidates))
	for _, h := range hits {
		out = append(out, h.e)
	}
	return append(out, rest...), len(hits)
}

// MatchOptions tunes Match for one picker.
type MatchOptions struct {
	// Substring also matches names that contain the query, after every
	// name that starts with it.
	Substring bool
	// Limit caps the result; 0 means no cap.
	Limit int
	// Frecency puts the emoji the user uses first.
	Frecency Frecency
	// Customs is the workspace custom emoji map, for Frecency.Position.
	Customs map[string]string
}

// Match returns the entries whose name matches query (case- and
// accent-insensitive, through text.Fold), in the order an emoji picker
// lists them:
//
//  1. used emoji whose name starts with query, most frecent first;
//  2. with Substring, used emoji whose name only contains it, most
//     frecent first;
//  3. every other name that starts with query, in entries' order;
//  4. with Substring, every other name that contains it.
//
// A used emoji is listed once even when several of its aliases match
// (the first in that order stays), so "+1" and "thumbsup" do not take
// two rows. An empty query matches every entry. entries is expected in
// alphabetical order (BuildEntries' order); name reads an entry's
// emoji name.
func Match[E any](entries []E, name func(E) string, query string, opt MatchOptions) []E {
	q := slktext.Fold(query)
	var prefix, substring []E
	for _, e := range entries {
		folded := slktext.Fold(name(e))
		switch {
		case strings.HasPrefix(folded, q):
			prefix = append(prefix, e)
		case opt.Substring && strings.Contains(folded, q):
			substring = append(substring, e)
		}
	}
	p, np := Rank(prefix, name, opt.Frecency, opt.Customs)
	s, ns := Rank(substring, name, opt.Frecency, opt.Customs)

	out := make([]E, 0, len(p)+len(s))
	seen := make(map[int]bool, np+ns)
	for _, e := range slices.Concat(p[:np], s[:ns]) {
		pos, _ := opt.Frecency.Position(name(e), opt.Customs)
		if seen[pos] {
			continue
		}
		seen[pos] = true
		out = append(out, e)
	}
	out = append(out, p[np:]...)
	out = append(out, s[ns:]...)
	if opt.Limit > 0 && len(out) > opt.Limit {
		out = out[:opt.Limit]
	}
	return out
}

// Used returns the emoji a message uses, under the name the history
// records them by (CanonicalSlackName), each once, in order of first
// use. shortcodes are the message's emoji as it sends them, without
// colons, a skin tone joined to its emoji in Slack's
// "<name>::skin-tone-N" form (mrkdwn.EmojiShortcodes gives them for
// typed text). A shortcode that names no emoji, built-in or in customs
// (the ":30:" of "12:30:45"), is left out.
func Used(shortcodes []string, customs map[string]string) []string {
	var out []string
	seen := map[string]bool{}
	for _, sc := range shortcodes {
		_, custom := customs[sc]
		if _, builtin := slackCodeMap[":"+StripSkinTone(sc)+":"]; !custom && !builtin {
			continue
		}
		name := CanonicalSlackName(sc, customs)
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	return out
}
