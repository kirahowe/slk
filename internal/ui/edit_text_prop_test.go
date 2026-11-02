package ui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/slack-go/slack"
	"pgregory.net/rapid"

	"github.com/gammons/slk/internal/core/blocks"
	"github.com/gammons/slk/internal/ui/channelpicker"
	"github.com/gammons/slk/internal/ui/compose"
	"github.com/gammons/slk/internal/ui/mentionpicker"
	"github.com/gammons/slk/internal/ui/messages"
	"github.com/gammons/slk/internal/ui/messages/blockkit"
)

// The whole edit path, E then Enter with nothing changed, posts what
// the message already holds: composer text sent (TranslateMentionsForSend,
// TypedBody), seeded back into the editor (EditSource, MentionsForEdit)
// and sent again gives the same rich_text block, and the editor's text
// is a fixed point from the first edit on. The mrkdwn fallback can
// differ where the block has no room for what was typed: a blank line
// between two lists.
func TestEditUntouchedResendsTheMessage(t *testing.T) {
	c := compose.New("general")
	c.SetUsers([]mentionpicker.User{{ID: "U1", DisplayName: "alice"}, {ID: "U2", DisplayName: "bob"}})
	c.SetChannels([]channelpicker.Channel{{ID: "C1", Name: "general"}, {ID: "C2", Name: "general-help"}})
	c.SetUserGroups(map[string]string{"S1": "eng"})
	word := rapid.SampledFrom([]string{"hi", "see", "**bold**", "_it_", "`code`", "@alice", "@bob", "@eng", "@here",
		"#general", "#general-help", "[docs](https://example.com/d)", "https://example.com/x", ":wave:"})
	line := rapid.Custom(func(t *rapid.T) string {
		prefix := rapid.SampledFrom([]string{"", "", "- ", "1. ", "> "}).Draw(t, "prefix")
		return prefix + strings.Join(rapid.SliceOfN(word, 1, 4).Draw(t, "words"), " ")
	})
	send := func(text string) messages.MessageItem {
		msg := messages.MessageItem{TS: "1.0"}
		messages.TypedBody(c.TranslateMentionsForSend(text)).Apply(&msg)
		return msg
	}
	rapid.Check(t, func(t *rapid.T) {
		typed := strings.Join(rapid.SliceOfN(line, 1, 5).Draw(t, "lines"), rapid.SampledFrom([]string{"\n", "\n\n"}).Draw(t, "sep"))
		first := send(typed)
		if len(first.Blocks) == 0 {
			t.Skip("no block")
		}
		seed := c.MentionsForEdit(messages.EditSource(first))
		again := send(seed)
		if got, want := blocksJSON(t, again.Blocks), blocksJSON(t, first.Blocks); got != want {
			t.Fatalf("typed %q\nseed  %q\n got %s\nwant %s", typed, seed, got, want)
		}
		if seed2 := c.MentionsForEdit(messages.EditSource(again)); seed2 != seed {
			t.Fatalf("seed not a fixed point: %q then %q", seed, seed2)
		}
	})
}

// blocksJSON is the wire JSON of a body's rich_text blocks.
func blocksJSON(t *rapid.T, bs []blocks.Block) string {
	var out []string
	for _, b := range bs {
		rt, ok := b.(blockkit.RichTextBlock)
		if !ok {
			t.Fatalf("block %T is not rich_text", b)
		}
		raw, err := json.Marshal(slack.NewRichTextBlock("", rt.Elements...))
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, string(raw))
	}
	return strings.Join(out, "\n")
}
