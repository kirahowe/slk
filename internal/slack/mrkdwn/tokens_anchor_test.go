package mrkdwn

import (
	"regexp"
	"strings"
	"testing"

	"github.com/gammons/slk/internal/proptest"
	"pgregory.net/rapid"
)

// tokenPatterns is every pattern tokenAt reads a token with, and a
// token each one matches.
var tokenPatterns = []struct {
	name  string
	re    *regexp.Regexp
	token string
}{
	{"user", reUser, "<@U1>"},
	{"channel", reChannel, "<#C1|general>"},
	{"broadcast", reBroadcast, "<!subteam^S1|@team>"},
	{"link", reLink, "<https://x.com|x>"},
	{"emoji", reEmoji, ":tada:"},
}

// A token pattern matches a token at the start of the text and nowhere
// else, so tokenAt's failed match at one '<' or ':' does not search the
// rest of the input.
func TestTokenPatternsMatchOnlyAtStart(t *testing.T) {
	for _, p := range tokenPatterns {
		if loc := p.re.FindStringIndex(p.token + " rest"); loc == nil || loc[0] != 0 || loc[1] != len(p.token) {
			t.Errorf("%s: %q at the start matched at %v, want [0 %d]", p.name, p.token, loc, len(p.token))
		}
		for _, prefix := range []string{"x", " ", "<", ":", "a\n"} {
			if loc := p.re.FindStringIndex(prefix + p.token); loc != nil {
				t.Errorf("%s: matched %q at %v, want no match", p.name, prefix+p.token, loc)
			}
		}
	}
}

// Property: in any text, a token pattern's match, if it has one, starts
// at byte 0.
func TestTokenPatternsMatchOnlyAtStartProp(t *testing.T) {
	text := proptest.SlackText([]string{"U1", "W2"}, []string{"C1", "G2"})
	rapid.Check(t, func(t *rapid.T) {
		s := text.Draw(t, "text")
		for _, p := range tokenPatterns {
			if loc := p.re.FindStringIndex(s); loc != nil && loc[0] != 0 {
				t.Fatalf("%s matched %q at %v", p.name, s, loc)
			}
		}
	})
}

func BenchmarkConvertColonRun(b *testing.B) {
	s := strings.Repeat(":", 20000)
	for b.Loop() {
		Convert(s)
	}
}

func BenchmarkConvertAngleRun(b *testing.B) {
	s := strings.Repeat("<", 20000)
	for b.Loop() {
		Convert(s)
	}
}
