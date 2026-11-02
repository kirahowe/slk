package messages

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"pgregory.net/rapid"

	"github.com/gammons/slk/internal/core/blocks"
	"github.com/gammons/slk/internal/proptest"
	"github.com/gammons/slk/internal/ui/messages/blockkit"
)

// mentionTokenRe finds the mention and broadcast tokens of wire text,
// in the shapes proptest.SlackText writes (its link labels may hold a
// "<#" that is no token).
var mentionTokenRe = regexp.MustCompile(`<@[UW][A-Z0-9]+(?:\|[^<>]*)?>|<#[CG][A-Z0-9]+(?:\|[^<>]*)?>|<!(?:here|channel|everyone)(?:\|[^<>]*)?>|<!subteam\^S[A-Z0-9]+(?:\|[^<>]*)?>`)

// A message with no rich_text block is edited from its text: every
// mention token in it is kept as it is, and none is made from Slack's
// escaped text ("&lt;!here&gt;" stays text), so the edit text holds
// exactly the message's mention tokens, in order.
func TestEditSourceTextKeepsMentionTokens(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		text := proptest.SlackText(nil, nil).Draw(t, "text")
		got := EditSource(MessageItem{Text: text})
		if g, w := mentionTokenRe.FindAllString(got, -1), mentionTokenRe.FindAllString(text, -1); !slices.Equal(g, w) {
			t.Fatalf("EditSource(%q) = %q\ntokens %q, want %q", text, got, g, w)
		}
	})
}

// A message with rich_text blocks is edited from them, whatever its
// text field says: EditSource is their RichTextToMarkdown, a blank line
// apart. (That saving it untouched resends the same block is
// blockkit's TestRichTextToMarkdownResendsTheSameBlock.)
func TestEditSourcePrefersRichText(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		typed := rapid.SliceOfN(rapid.StringMatching(`([a-z]{1,5}|<@U1>|\*\*[a-z]{1,3}\*\*)( |\n|\n- )`), 1, 6).Draw(t, "typed")
		var bs []blocks.Block
		var want []string
		for _, s := range typed {
			if b := TypedBody(s).Blocks; len(b) > 0 {
				bs = append(bs, b[0])
				if md := blockkit.RichTextToMarkdown(b[0].(blockkit.RichTextBlock)); md != "" {
					want = append(want, md)
				}
			}
		}
		if len(want) == 0 {
			t.Skip("no block with text")
		}
		msg := MessageItem{Text: rapid.String().Draw(t, "text"), Blocks: bs}
		if got := EditSource(msg); got != strings.Join(want, "\n\n") {
			t.Fatalf("EditSource = %q, want %q", got, strings.Join(want, "\n\n"))
		}
	})
}
