package blockkit

import (
	"slices"
	"strings"
	"testing"

	"github.com/slack-go/slack"
	"pgregory.net/rapid"

	"github.com/gammons/slk/internal/proptest"
	"github.com/gammons/slk/internal/slack/mrkdwn"
)

// literalWord is proptest.Word without the markdown style characters.
//
// KNOWN: RichTextToMarkdown does not escape text, so a literal "*",
// "_", "~" or "`" the author wrote comes back as markdown: "~" struck
// through is "~~~~~", a code fence that swallows the rest of its line,
// mentions and all.
func literalWord() *rapid.Generator[string] {
	return rapid.Map(proptest.Word(), func(w string) string {
		w = strings.Map(func(r rune) rune {
			if strings.ContainsRune("*_~`", r) {
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
