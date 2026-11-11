package messages

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/slack-go/slack"

	"github.com/gammons/slk/internal/core/blocks"
	"github.com/gammons/slk/internal/slack/mrkdwn"
	"github.com/gammons/slk/internal/ui/messages/blockkit"
)

// Without a rich_text block the text field is all there is: its links
// read as markdown and Slack's escapes are decoded. Decoded text that
// would read as a mention or link on save is backslash-escaped, so it
// stays the text it was.
func TestEditSource_TextOnly(t *testing.T) {
	for _, tc := range []struct{ text, want string }{
		{"plain", "plain"},
		{"a &lt; b &amp;&amp; c &gt; d", "a < b && c > d"},
		{"see <https://x.com/a?b=1&amp;c=2|the docs> now", "see [the docs](https://x.com/a?b=1&c=2) now"},
		{"bare <https://x.com/>", "bare https://x.com/"},
		{"mail <mailto:a@b.com|a@b.com>", "mail a@b.com"},
		{"hi <@U1> in <#C2>", "hi <@U1> in <#C2>"},
		{"&lt;https://x.com/|not a link&gt;", `\<https\://x.com/\|not a link\>`},
		{"&lt;!here&gt; is text, <!here> is not", `\<!here\> is text, <!here> is not`},
		{"&lt;@U1&gt; vs <@U1>", `\<@U1\> vs <@U1>`},
	} {
		if got := EditSource(MessageItem{Text: tc.text}); got != tc.want {
			t.Errorf("EditSource(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}

// Saving an unchanged edit of a text-only message (an old one, or an
// app's) sends its mentions and links and nothing more: text that only
// looks like one stays text.
func TestEditSource_TextOnlySavesTheSameMentions(t *testing.T) {
	for _, tc := range []struct {
		text string
		want []string
	}{
		{"&lt;!here&gt; is text, <!here> is not", []string{"!here"}},
		{"&lt;@U1&gt; vs <@U1> and &lt;#C1|x&gt;", []string{"@U1"}},
		{"&lt;https://x.com/|not a link&gt; and <https://x.com/|a link>", []string{"https://x.com/"}},
	} {
		seed := EditSource(MessageItem{Text: tc.text})
		_, rt := mrkdwn.Convert(seed)
		var got []string
		for _, e := range rt.Elements {
			for _, el := range e.(*slack.RichTextSection).Elements {
				switch v := el.(type) {
				case *slack.RichTextSectionBroadcastElement:
					got = append(got, "!"+v.Range)
				case *slack.RichTextSectionUserElement:
					got = append(got, "@"+v.UserID)
				case *slack.RichTextSectionChannelElement:
					got = append(got, "#"+v.ChannelID)
				case *slack.RichTextSectionLinkElement:
					got = append(got, v.URL)
				}
			}
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("seed %q of %q saves %v, want %v", seed, tc.text, got, tc.want)
		}
	}
}

// Literal emphasis and code markers, typed escaped in slk, come back
// escaped where they would otherwise be markup, so saving the edit
// sends the same block.
func TestEditSourceLiteralMarkers(t *testing.T) {
	for _, typed := range []string{
		`\*not bold\*`,
		`2 * 3 * 4 = 24`,
		`snake_case_name and _under_`,
		`\_not italic\_`,
		`\~\~not struck\~\~`,
		"a \\`tick\\` here",
		`1\. not a list`,
		`\- not a list`,
		`\> not a quote`,
		`a\\b`,
		`\[not\](a link)`,
		`:tada: and \:tada\:`,
		`**bold with \* inside**`,
	} {
		var msg MessageItem
		TypedBody(typed).Apply(&msg)
		seed := EditSource(msg)
		_, want := mrkdwn.Convert(typed)
		_, got := mrkdwn.Convert(seed)
		wj, _ := json.Marshal(want)
		gj, _ := json.Marshal(got)
		if string(gj) != string(wj) {
			t.Errorf("typed %q, seed %q:\n got %s\nwant %s", typed, seed, gj, wj)
		}
	}
}

// Every rich_text block is kept, so saving does not drop the later ones.
func TestEditSource_AllRichTextBlocks(t *testing.T) {
	block := func(s string) blocks.Block {
		return blockkit.RichTextBlock{Elements: []slack.RichTextElement{
			&slack.RichTextSection{Type: slack.RTESection, Elements: []slack.RichTextSectionElement{
				&slack.RichTextSectionTextElement{Type: slack.RTSEText, Text: s},
			}},
		}}
	}
	msg := MessageItem{Text: "first second", Blocks: []blocks.Block{block("first"), block("second")}}
	if got, want := EditSource(msg), "first\n\nsecond"; got != want {
		t.Errorf("EditSource = %q, want %q", got, want)
	}
}

// denseLine is a line of the punctuation a person types in prose, each
// mark one the seed could escape: a colon, parens, a URL, a code span,
// " - ", "!", brackets, an HTML-shaped word, a pipe and "3.".
const denseLine = "Note: see (docs) at https://example.com/a?b=1, then run `make test` - ok! [z] <hm> | 3. done"

// denseTyped is denseLine on n lines of one paragraph.
func denseTyped(n int) string {
	return strings.TrimSuffix(strings.Repeat(denseLine+"\n", n), "\n")
}

// A long message dense with punctuation comes back as it was typed, on
// every line: none of its escapes changes what Convert sends, so the
// seed drops them all, however long the paragraph.
func TestEditSourceDenseMessageReadsAsTyped(t *testing.T) {
	for _, lines := range []int{35, 130} { // 3,254 and 12,089 characters
		typed := denseTyped(lines)
		var msg MessageItem
		TypedBody(typed).Apply(&msg)
		got := EditSource(msg)
		if got != typed {
			first := 0
			for first < min(len(got), len(typed)) && got[first] == typed[first] {
				first++
			}
			t.Errorf("%d characters: seed differs from the typed text at byte %d, with %d backslashes: ...%q",
				len(typed), first, strings.Count(got, `\`), got[first:min(len(got), first+80)])
		}
	}
}

func BenchmarkEditSourceDense(b *testing.B) {
	var msg MessageItem
	TypedBody(denseTyped(35)).Apply(&msg)
	for b.Loop() {
		EditSource(msg)
	}
}
