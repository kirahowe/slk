package mrkdwn

import (
	"encoding/json"
	"regexp"
	"testing"

	"pgregory.net/rapid"

	"github.com/gammons/slk/internal/proptest"
)

// typedWordRe matches the words typedWord draws: no ID, URL, emoji name
// or markup the generator writes looks like one.
var typedWordRe = regexp.MustCompile(`Q[a-z]+`)

// channelLabelRe matches the label of a channel mention, "|x" in
// "<#C1|x>".
var channelLabelRe = regexp.MustCompile(`(<#[CG][A-Z0-9]+)\|[^>]*>`)

// Slack's clients draw the block, not the mrkdwn fallback, so text the
// block leaves out is text the user typed and nobody sees. Every word
// of composer text is in the block Convert makes for it: in a text,
// link, code or quote element, a list item, a code block.
//
// One part of a message is not text and is left out on purpose: the
// label of a channel mention ("<#C1|name>"), which the block's channel
// element does not carry (Slack draws the channel's current name).
// The words of a code fence's info string ("```go") are lost as well;
// ComposerTextWithMarkers writes no info string.
func TestConvertKeepsEveryTypedWord(t *testing.T) {
	typedWord := rapid.StringMatching(`Q[a-z]{1,5}`)
	rapid.Check(t, func(t *rapid.T) {
		typed := proptest.ComposerTextWithMarkers(typedWord, []string{"U1"}, []string{"C1"}).Draw(t, "typed")
		_, block := Convert(typed)
		raw, err := json.Marshal(block)
		if err != nil {
			t.Fatal(err)
		}
		sent := map[string]int{}
		for _, word := range typedWordRe.FindAllString(string(raw), -1) {
			sent[word]++
		}
		for _, word := range typedWordRe.FindAllString(channelLabelRe.ReplaceAllString(typed, "$1>"), -1) {
			if sent[word] == 0 {
				t.Fatalf("typed %q: the block is short of %q\nblock %s", typed, word, raw)
			}
			sent[word]--
		}
	})
}
