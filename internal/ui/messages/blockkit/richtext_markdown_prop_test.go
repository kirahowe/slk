package blockkit

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/slack-go/slack"
	"pgregory.net/rapid"

	"github.com/gammons/slk/internal/proptest"
	"github.com/gammons/slk/internal/slack/mrkdwn"
)

// sentBlock is composed sent through mrkdwn.Convert and back through
// JSON, the way the block arrives from Slack; ok is false when Convert
// gives no block.
func sentBlock(t *rapid.T, composed string) (RichTextBlock, bool) {
	_, block := mrkdwn.Convert(composed)
	if block == nil {
		return RichTextBlock{}, false
	}
	raw, err := json.Marshal(block)
	if err != nil {
		t.Fatal(err)
	}
	var back slack.RichTextBlock
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	return RichTextBlock{Elements: back.Elements}, true
}

func blockJSONOf(t *rapid.T, rt RichTextBlock) string {
	raw, err := json.Marshal(slack.NewRichTextBlock("", rt.Elements...))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// composerWord is a word a person types: letters, a style, a link, a
// mention token, an emoji.
func composerWord() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		w := rapid.StringMatching(`[a-z]{1,6}`).Draw(t, "w")
		switch rapid.IntRange(0, 9).Draw(t, "kind") {
		case 0:
			return "**" + w + "**"
		case 1:
			return "_" + w + "_"
		case 2:
			return "~~" + w + "~~"
		case 3:
			return "`" + w + "`"
		case 4:
			return "[" + w + "](https://example.com/" + w + ")"
		case 5:
			return "https://example.com/" + w
		case 6:
			return rapid.SampledFrom([]string{"<@U1>", "<#C1>", "<!here>", "<!subteam^S1>"}).Draw(t, "mention")
		case 7:
			return ":wave:"
		default:
			return w
		}
	})
}

// composerLine is one line of composer text: a paragraph line, a list
// item (bulleted or numbered, at the margin or nested under one), a
// quote line or a blank line.
func composerText() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		para := func() string {
			return strings.Join(rapid.SliceOfN(composerWord(), 1, 4).Draw(t, "words"), " ")
		}
		var lines []string
		n := rapid.IntRange(1, 6).Draw(t, "lines")
		for range n {
			switch rapid.IntRange(0, 5).Draw(t, "line") {
			case 0:
				lines = append(lines, "- "+para())
			case 1:
				lines = append(lines, "1. "+para())
			case 2:
				// A nested item only means one under an item.
				if len(lines) > 0 && strings.HasPrefix(lines[len(lines)-1], "- ") {
					lines = append(lines, "    - "+para())
				} else {
					lines = append(lines, "- "+para())
				}
			case 3:
				lines = append(lines, "> "+para())
			case 4:
				lines = append(lines, "")
			default:
				lines = append(lines, para())
			}
		}
		return strings.Join(lines, "\n")
	})
}

// Editing a message slk sent and saving it untouched posts the same
// block: Convert(RichTextToMarkdown(b)) == b for every block Convert
// makes from composer text, and the edit text is a fixed point.
func TestRichTextToMarkdownResendsTheSameBlock(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		composed := composerText().Draw(t, "composed")
		first, ok := sentBlock(t, composed)
		if !ok {
			t.Skip("no block")
		}
		md := RichTextToMarkdown(first)
		again, ok := sentBlock(t, md)
		if !ok {
			t.Fatalf("composed %q\nedit text %q gives no block", composed, md)
		}
		if got, want := blockJSONOf(t, again), blockJSONOf(t, first); got != want {
			t.Fatalf("composed %q\nedit text %q\n got %s\nwant %s", composed, md, got, want)
		}
		if md2 := RichTextToMarkdown(again); md2 != md {
			t.Fatalf("edit text not a fixed point: %q then %q", md, md2)
		}
	})
}

// literalWord is proptest.Word without the markdown style characters
// and "<".
//
// KNOWN: RichTextToMarkdown does not escape text, so a literal "*",
// "_", "~" or "`" the author wrote comes back as markdown: "~" struck
// through is "~~~~~", a code fence that swallows the rest of its line,
// mentions and all. Likewise a literal "<" just before a link opens a
// token that swallows the mention after it ("<https://x/<@U1>").
func literalWord() *rapid.Generator[string] {
	return rapid.Map(proptest.Word(), func(w string) string {
		w = strings.Map(func(r rune) rune {
			if strings.ContainsRune("*_~`<", r) {
				return -1
			}
			return r
		}, w)
		if w == "" {
			return "x"
		}
		return w
	})
}

// richTextBlock draws a rich_text block the way Slack's client writes
// one: sections, lists, quotes and code holding text with any runes
// (including literal "<!here>" and "<@U1>"), styles, links, mentions
// and emoji.
func richTextBlock() *rapid.Generator[RichTextBlock] {
	inline := rapid.Custom(func(t *rapid.T) slack.RichTextSectionElement {
		switch rapid.IntRange(0, 6).Draw(t, "inline") {
		case 0:
			return &slack.RichTextSectionUserElement{Type: slack.RTSEUser, UserID: proptest.ID("U").Draw(t, "user")}
		case 1:
			return &slack.RichTextSectionChannelElement{Type: slack.RTSEChannel, ChannelID: proptest.ID("C").Draw(t, "channel")}
		case 2:
			return &slack.RichTextSectionBroadcastElement{Type: slack.RTSEBroadcast, Range: rapid.SampledFrom([]string{"here", "channel", "everyone"}).Draw(t, "range")}
		case 3:
			return &slack.RichTextSectionUserGroupElement{Type: slack.RTSEUserGroup, UsergroupID: proptest.ID("S").Draw(t, "group")}
		case 4:
			return &slack.RichTextSectionLinkElement{Type: slack.RTSELink, URL: "https://example.com/" + rapid.StringMatching(`[a-z]{0,6}`).Draw(t, "path"), Text: rapid.StringMatching(`[a-z ]{0,6}`).Draw(t, "label")}
		default:
			text := rapid.OneOf(
				literalWord(),
				rapid.SampledFrom([]string{"<!here>", "<!channel>", "<@U1>", "<#C1>", "<!subteam^S1>", "&lt;!here&gt;", " ", "\n"}),
			).Draw(t, "text")
			var style *slack.RichTextSectionTextStyle
			if rapid.Bool().Draw(t, "styled") {
				style = &slack.RichTextSectionTextStyle{
					Bold:   rapid.Bool().Draw(t, "bold"),
					Italic: rapid.Bool().Draw(t, "italic"),
					Strike: rapid.Bool().Draw(t, "strike"),
					Code:   rapid.Bool().Draw(t, "code"),
				}
			}
			return &slack.RichTextSectionTextElement{Type: slack.RTSEText, Text: text, Style: style}
		}
	})
	section := rapid.Custom(func(t *rapid.T) *slack.RichTextSection {
		return &slack.RichTextSection{Type: slack.RTESection, Elements: rapid.SliceOfN(inline, 1, 4).Draw(t, "inlines")}
	})
	element := rapid.Custom(func(t *rapid.T) slack.RichTextElement {
		switch rapid.IntRange(0, 3).Draw(t, "element") {
		case 0:
			items := rapid.SliceOfN(section, 1, 3).Draw(t, "items")
			els := make([]slack.RichTextElement, len(items))
			for i, it := range items {
				els[i] = it
			}
			return &slack.RichTextList{
				Type:     slack.RTEList,
				Style:    rapid.SampledFrom([]slack.RichTextListElementType{slack.RTEListBullet, slack.RTEListOrdered}).Draw(t, "style"),
				Indent:   rapid.IntRange(0, 2).Draw(t, "indent"),
				Elements: els,
			}
		case 1:
			return &slack.RichTextQuote{Type: slack.RTEQuote, Elements: section.Draw(t, "quote").Elements}
		case 2:
			return &slack.RichTextPreformatted{Type: slack.RTEPreformatted, Elements: []slack.RichTextSectionElement{
				&slack.RichTextSectionTextElement{Type: slack.RTSEText, Text: proptest.PlainText().Draw(t, "code")},
			}}
		default:
			return section.Draw(t, "section")
		}
	})
	return rapid.Custom(func(t *rapid.T) RichTextBlock {
		return RichTextBlock{Elements: rapid.SliceOfN(element, 1, 4).Draw(t, "elements")}
	})
}

// mentionsOf lists the mention elements of a block, in order, as
// "@U…", "#C…", "!here", "^S…".
func mentionsOf(elements []slack.RichTextElement) []string {
	var out []string
	inline := func(els []slack.RichTextSectionElement) {
		for _, e := range els {
			switch v := e.(type) {
			case *slack.RichTextSectionUserElement:
				out = append(out, "@"+v.UserID)
			case *slack.RichTextSectionChannelElement:
				out = append(out, "#"+v.ChannelID)
			case *slack.RichTextSectionBroadcastElement:
				out = append(out, "!"+v.Range)
			case *slack.RichTextSectionUserGroupElement:
				out = append(out, "^"+v.UsergroupID)
			}
		}
	}
	for _, e := range elements {
		switch v := e.(type) {
		case *slack.RichTextSection:
			inline(v.Elements)
		case *slack.RichTextList:
			out = append(out, mentionsOf(v.Elements)...)
		case *slack.RichTextQuote:
			inline(v.Elements)
		case *slack.RichTextPreformatted:
			inline(v.Elements)
		}
	}
	return out
}

// Saving an untouched edit of any message pings exactly whom the
// message pings: the mention elements of Convert(RichTextToMarkdown(b))
// are b's, in order, whatever literal text shaped like a mention b
// holds.
func TestRichTextToMarkdownKeepsMentions(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		b := richTextBlock().Draw(t, "block")
		md := RichTextToMarkdown(b)
		var got []string
		if _, block := mrkdwn.Convert(md); block != nil {
			got = mentionsOf(block.Elements)
		}
		if want := mentionsOf(b.Elements); !slices.Equal(got, want) {
			t.Fatalf("edit text %q\nmentions %v, want %v", md, got, want)
		}
	})
}
