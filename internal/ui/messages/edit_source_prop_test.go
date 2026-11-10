package messages

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/slack-go/slack"
	"pgregory.net/rapid"

	"github.com/gammons/slk/internal/proptest"
	"github.com/gammons/slk/internal/slack/mrkdwn"
	"github.com/gammons/slk/internal/ui/messages/blockkit"
)

// sentForm is what chat.update sends for composer text: Convert's
// mrkdwn and its rich_text block as JSON ("" for none).
func sentForm(t *rapid.T, typed string) (text, block string) {
	text, rt := mrkdwn.Convert(typed)
	if rt == nil {
		return text, ""
	}
	raw, err := json.Marshal(rt)
	if err != nil {
		t.Fatal(err)
	}
	return text, string(raw)
}

// receivedTyped is the message Slack hands back for composer text: its
// TypedBody, with the blocks through JSON the way a fetch or echo
// delivers them.
func receivedTyped(t *rapid.T, typed string) MessageItem {
	var msg MessageItem
	TypedBody(typed).Apply(&msg)
	_, rt := mrkdwn.Convert(typed)
	if rt == nil {
		return msg
	}
	raw, err := json.Marshal(slack.Blocks{BlockSet: []slack.Block{rt}})
	if err != nil {
		t.Fatal(err)
	}
	var back slack.Blocks
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	msg.Blocks = blockkit.Parse(back)
	return msg
}

// nestedBeforeParentItem reports whether typed has a nested list item
// with an item of a shallower list after it.
func nestedBeforeParentItem(typed string) bool {
	deepest := 0
	for _, line := range strings.Split(typed, "\n") {
		indent := len(line) - len(strings.TrimLeft(line, " "))
		if !listItemRe.MatchString(line) {
			continue
		}
		if indent < deepest {
			return true
		}
		deepest = max(deepest, indent)
	}
	return false
}

// codeEdgeBreak reports whether Convert gives typed a code-styled text
// run that starts or ends with a line break.
func codeEdgeBreak(typed string) bool {
	_, rt := mrkdwn.Convert(typed)
	if rt == nil {
		return false
	}
	for _, e := range rt.Elements {
		sec, ok := e.(*slack.RichTextSection)
		if !ok {
			continue
		}
		for _, el := range sec.Elements {
			if t, ok := el.(*slack.RichTextSectionTextElement); ok && t.Style != nil && t.Style.Code &&
				(strings.HasPrefix(t.Text, "\n") || strings.HasSuffix(t.Text, "\n")) {
				return true
			}
		}
	}
	return false
}

// siblingSections reports whether Convert gives typed two top-level
// sections in a row.
func siblingSections(typed string) bool {
	_, rt := mrkdwn.Convert(typed)
	if rt == nil {
		return false
	}
	for i := 1; i < len(rt.Elements); i++ {
		_, a := rt.Elements[i-1].(*slack.RichTextSection)
		_, b := rt.Elements[i].(*slack.RichTextSection)
		if a && b {
			return true
		}
	}
	return false
}

// tokenLabelRe matches a channel, broadcast or usergroup token with a
// label, group 1 being the token without it.
var tokenLabelRe = regexp.MustCompile(`(<(?:#[CG][A-Z0-9]+|![a-z]+(?:\^[A-Za-z0-9]+)?))\|[^>]*>`)

// listItemRe matches a line that is a list item.
var listItemRe = regexp.MustCompile(`^ *(?:[-+*]|\d+[.)]) `)

// The block of a list with a nested list in the middle has the nested
// list after the whole parent list, so the seed lists it last. Saving
// it sends the same block; only the mrkdwn fallback, which kept the
// typed order, changes.
func TestEditSourceNestedListOrder(t *testing.T) {
	const typed = "- a\n    - nested\n- b"
	var msg MessageItem
	TypedBody(typed).Apply(&msg)
	if got, want := EditSource(msg), "- a\n- b\n    - nested"; got != want {
		t.Errorf("EditSource = %q, want %q", got, want)
	}
}

// Opening the editor on a message typed in slk and saving it unchanged
// sends the same rich_text block, which is what Slack's clients draw:
// EditSource of the message Convert made from typed text, with wide,
// combining and emoji runes and literal "<", ">", "&", "|", "#", "@",
// ":", "-" and "." in its words, converts to the block the typed text
// did.
//
// The words leave out the emphasis and code markers ("*", "_", "~",
// "`"). Typed as literals next to real markers they make runs Convert
// reads in ways no seed reproduces ("- ~~.~a~~" is a "~" and a struck
// ".a", and "\~~~.a~~" is plain text); TestEditSourceLiteralMarkers
// covers them one at a time.
func TestEditSourceRoundTripsTheBlock(t *testing.T) {
	noMarkers := func(s string) string {
		return strings.Map(func(r rune) rune {
			if strings.ContainsRune("*_~`", r) {
				return -1
			}
			return r
		}, s)
	}
	word := rapid.Custom(func(t *rapid.T) string {
		if w := noMarkers(proptest.Word().Draw(t, "word")); w != "" {
			return w
		}
		return "x"
	})
	rapid.Check(t, func(t *rapid.T) {
		typed := rapid.OneOf(
			proptest.ComposerTextWith(word, []string{"U1"}, []string{"C1"}),
			rapid.Map(proptest.PlainText(), noMarkers),
		).Draw(t, "typed")
		_, wantBlock := sentForm(t, typed)
		if strings.Contains(wantBlock, `"elements":null`) {
			// An item with nothing in it ("- " then a bare fence):
			// not valid rich text, which Slack refuses.
			t.Skip("empty element")
		}
		if siblingSections(typed) {
			// Two sections in a row (an empty "> " line between two
			// paragraphs drops out): the seed writes them as one
			// paragraph, which draws the same.
			t.Skip("sibling sections")
		}
		if codeEdgeBreak(typed) {
			// A code span starting or ending in a line break ("`\n`",
			// "`a\n`"): Convert keeps the break, but a seed with one
			// there puts the closing backticks at a line start, where
			// they open a code block.
			t.Skip("code line break")
		}
		seed := EditSource(receivedTyped(t, typed))
		_, gotBlock := sentForm(t, seed)
		if gotBlock != wantBlock {
			t.Fatalf("typed %q, edit seed %q:\nblock %s\nwant  %s", typed, seed, gotBlock, wantBlock)
		}
	})
}

// For text written the usual way (words, and markdown that means what
// it says), saving an unchanged edit sends the same mrkdwn fallback as
// well. Literal markup characters can come back spelled another way
// that draws the same ("**a*.**" as "**a.***"): the block keeps the
// text, not how it was typed.
func TestEditSourceRoundTripsTheMrkdwn(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		word := rapid.StringMatching(`[a-z0-9]{1,6}`)
		// The block has no room for a blank line between two lists,
		// code blocks or quotes, so they are typed a line apart; the
		// first property covers blank lines next to paragraphs.
		typed := proptest.ComposerTextWith(word, []string{"U1"}, []string{"C1"}).Draw(t, "typed")
		for strings.Contains(typed, "\n\n") {
			typed = strings.ReplaceAll(typed, "\n\n", "\n")
		}
		seed := EditSource(receivedTyped(t, typed))
		gotText, gotBlock := sentForm(t, seed)
		wantText, wantBlock := sentForm(t, typed)
		if gotBlock != wantBlock {
			t.Fatalf("typed %q, edit seed %q:\nblock %s\nwant  %s", typed, seed, gotBlock, wantBlock)
		}
		// Convert puts every nested list after its whole parent list
		// in the block (walkList), so the block of a nested item with
		// more of its parent after it no longer says where it went;
		// the seed has it last, and only the mrkdwn fallback differs.
		// TestEditSourceNestedListOrder pins that. A channel or
		// usergroup element keeps no label, so the seed's token has
		// none; the composer writes the current name back
		// (compose.MentionsForEdit).
		gotText, wantText = tokenLabelRe.ReplaceAllString(gotText, "$1>"), tokenLabelRe.ReplaceAllString(wantText, "$1>")
		if gotText != wantText && !nestedBeforeParentItem(typed) {
			t.Fatalf("typed %q, edit seed %q: text %q, want %q", typed, seed, gotText, wantText)
		}
	})
}
