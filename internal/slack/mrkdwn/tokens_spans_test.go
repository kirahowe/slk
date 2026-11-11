package mrkdwn

import (
	"slices"
	"testing"

	"github.com/gammons/slk/internal/proptest"
	"pgregory.net/rapid"
)

func TestTokens(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want []string
	}{
		{"cc <@bob>", nil},
		{"<!-- @bob --> hi", nil},
		{"a <!b and @bob -> c", nil},
		{"x < 5 and <#5 for @bob -> skip", nil},
		{"<@U1|alice> <mailto:a@b.co|x>", nil},
		{"hi <@U1>, <#C1|general> <!here>", []string{"<@U1>", "<#C1|general>", "<!here>"}},
		{"<!subteam^S1|@eng> <https://x.com|@bob> :tada:", []string{"<!subteam^S1|@eng>", "<https://x.com|@bob>", ":tada:"}},
		{"<<@U1>>", []string{"<@U1>"}},
	} {
		var got []string
		for _, sp := range Tokens(tc.in) {
			got = append(got, tc.in[sp.Start:sp.End])
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("Tokens(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// Property: Tokens lists the tokens tokenize takes out of the text, in
// order and without overlap: as many as tokenize's table holds, each
// one a token tokenAt reads whole at its start, every mention Mentions
// lists among them.
func TestTokensAreWhatTokenizeReads(t *testing.T) {
	text := proptest.SlackText([]string{"U1", "W2"}, []string{"C1", "G2"})
	rapid.Check(t, func(t *rapid.T) {
		s := text.Draw(t, "text")
		spans := Tokens(s)
		if _, table := tokenize(s); len(table) != len(spans) {
			t.Fatalf("Tokens(%q) = %v, tokenize read %d tokens", s, spans, len(table))
		}
		last := 0
		for _, sp := range spans {
			if sp.Start < last || sp.End <= sp.Start {
				t.Fatalf("Tokens(%q) = %v: out of order or empty", s, spans)
			}
			if _, n, ok := tokenAt(s, sp.Start); !ok || sp.Start+n != sp.End {
				t.Fatalf("Tokens(%q): %v is not the token read at %d", s, sp, sp.Start)
			}
			last = sp.End
		}
		for _, m := range Mentions(s) {
			if !slices.Contains(spans, Span{m.Start, m.End}) {
				t.Fatalf("Tokens(%q) = %v misses mention %+v", s, spans, m)
			}
		}
	})
}
