package blockkit

import (
	"encoding/json"
	"testing"

	"github.com/slack-go/slack"

	"github.com/gammons/slk/internal/slack/mrkdwn"
)

// received sends composer text through the send path (mrkdwn.Convert)
// and back through JSON, the way the block arrives from Slack.
func received(t *testing.T, composed string) RichTextBlock {
	t.Helper()
	_, block := mrkdwn.Convert(composed)
	if block == nil {
		t.Fatalf("Convert(%q) gave no block", composed)
	}
	raw, err := json.Marshal(block)
	if err != nil {
		t.Fatal(err)
	}
	var back slack.RichTextBlock
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	return RichTextBlock{Elements: back.Elements}
}

func blockJSON(t *testing.T, rt RichTextBlock) string {
	t.Helper()
	raw, err := json.Marshal(slack.NewRichTextBlock("", rt.Elements...))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// Composer text that slk sent comes back as the same text on edit, and
// sending that again posts the same block.
func TestRichTextToMarkdown_RoundTrip(t *testing.T) {
	for _, composed := range []string{
		"hello world",
		"line one\nline two",
		"see the list\n- one\n- two\n- three",
		"steps\n1. first\n2. second",
		"- top\n- last\n    - nested\n        - deeper",
		"**bold** and _italic_ and ~~gone~~ and `code`",
		"a [labelled link](https://example.com/a/b) and https://example.com/bare",
		"**[bold link](https://example.com/b)** and _[italic link](https://example.com/i)_",
		"**https://example.com/bare-bold**",
		"hi <@U123> in <#C456> <!here>",
		"before\n\n```\nfn main() {}\n```\n\nafter",
		"a list\n- item\n\nand a paragraph after it",
		"wave :wave::skin-tone-3: done",
	} {
		t.Run(composed, func(t *testing.T) {
			first := received(t, composed)
			md := RichTextToMarkdown(first)
			if md != composed {
				t.Errorf("edit text = %q, want %q", md, composed)
			}
			if got, want := blockJSON(t, received(t, md)), blockJSON(t, first); got != want {
				t.Errorf("re-sent block differs\n got %s\nwant %s", got, want)
			}
		})
	}
}

// A message written in Slack's own client (SLK-330): its text field has
// no line breaks and an escaped link, but the block has them all. The
// client ends the paragraph before a list with a line break of its
// own, which the seed keeps as a blank line, and starts the link's
// paragraph right under the list; saving the seed sends the same block.
func TestRichTextToMarkdown_SlackClientList(t *testing.T) {
	const raw = `{"type":"rich_text","elements":[
		{"type":"rich_text_section","elements":[{"type":"text","text":"here's a test with another list\n"}]},
		{"type":"rich_text_list","style":"bullet","indent":0,"elements":[
			{"type":"rich_text_section","elements":[{"type":"text","text":"list 1"}]},
			{"type":"rich_text_section","elements":[{"type":"text","text":"list 2"}]},
			{"type":"rich_text_section","elements":[{"type":"text","text":"list 3"}]}]},
		{"type":"rich_text_section","elements":[{"type":"link","url":"https://example.com/genie/","text":"and a link"}]}]}`
	var b slack.RichTextBlock
	if err := json.Unmarshal([]byte(raw), &b); err != nil {
		t.Fatal(err)
	}
	rt := RichTextBlock{Elements: b.Elements}
	got := RichTextToMarkdown(rt)
	want := "here's a test with another list\n\n- list 1\n- list 2\n- list 3\n[and a link](https://example.com/genie/)"
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
	if saved := blockJSON(t, received(t, got)); saved != blockJSON(t, rt) {
		t.Errorf("saving the seed sends %s\nwant %s", saved, blockJSON(t, rt))
	}
}

func TestRichTextToMarkdown_Inline(t *testing.T) {
	text := func(s string, st *slack.RichTextSectionTextStyle) slack.RichTextSectionElement {
		return &slack.RichTextSectionTextElement{Type: slack.RTSEText, Text: s, Style: st}
	}
	link := func(url, label string) slack.RichTextSectionElement {
		return &slack.RichTextSectionLinkElement{Type: slack.RTSELink, URL: url, Text: label}
	}
	for _, tc := range []struct {
		name string
		el   []slack.RichTextSectionElement
		want string
	}{
		{"bold keeps spaces outside", []slack.RichTextSectionElement{text("a", nil), text(" bold ", &slack.RichTextSectionTextStyle{Bold: true}), text("b", nil)}, "a **bold** b"},
		{"style over a line break", []slack.RichTextSectionElement{text("one\ntwo", &slack.RichTextSectionTextStyle{Italic: true})}, "_one\ntwo_"},
		{"style over a blank line", []slack.RichTextSectionElement{text("one\n\ntwo", &slack.RichTextSectionTextStyle{Italic: true})}, "_one_\n\n_two_"},
		{"bold around code", []slack.RichTextSectionElement{text("see ", &slack.RichTextSectionTextStyle{Bold: true}), text("x", &slack.RichTextSectionTextStyle{Bold: true, Code: true})}, "**see `x`**"},
		{"italic outside bold", []slack.RichTextSectionElement{text("a", &slack.RichTextSectionTextStyle{Bold: true, Italic: true}), text(" b", &slack.RichTextSectionTextStyle{Italic: true})}, "_**a** b_"},
		{"code with backtick", []slack.RichTextSectionElement{text("a`b", &slack.RichTextSectionTextStyle{Code: true})}, "``a`b``"},
		{"link label is url", []slack.RichTextSectionElement{link("https://x.com", "https://x.com")}, "https://x.com"},
		{"link without label", []slack.RichTextSectionElement{link("https://x.com", "")}, "https://x.com"},
		{"mailto", []slack.RichTextSectionElement{link("mailto:a@b.com", "a@b.com")}, "a@b.com"},
		{"parens in the url", []slack.RichTextSectionElement{link("https://x.com/a_(b)", "wiki")}, "[wiki](https://x.com/a_(b))"},
		{"url markdown can't hold", []slack.RichTextSectionElement{link("https://x.com/a b", "wiki")}, "<https://x.com/a b|wiki>"},
		{"bare url glued to text", []slack.RichTextSectionElement{link("https://x.com/a", ""), text("b", nil)}, "<https://x.com/a>b"},
		{"literal text unescaped", []slack.RichTextSectionElement{text("a < b & c", nil)}, "a < b & c"},
		{"literal markup escaped", []slack.RichTextSectionElement{text("*a* - b", nil)}, `*a\* - b`},
		{"literal list marker escaped", []slack.RichTextSectionElement{text("- a", nil)}, `\- a`},
		{"literal mentions stay text", []slack.RichTextSectionElement{text("<!here> <@U1> <#C1>", nil)}, `<!here\> <@U1\> <#C1\>`},
		{"literal link stays text", []slack.RichTextSectionElement{text("<https://x.com/|x>", nil)}, `<https\://x.com/|x>`},
		{"label markdown would restyle", []slack.RichTextSectionElement{link("https://x.com", "**docs**")}, "<https://x.com|**docs**>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := RichTextToMarkdown(rtSection(tc.el...)); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// A nested list with no list above it starts at the margin; four
// spaces after a paragraph would read as code.
func TestRichTextToMarkdown_OrphanIndent(t *testing.T) {
	item := &slack.RichTextSection{Type: slack.RTESection, Elements: []slack.RichTextSectionElement{
		&slack.RichTextSectionTextElement{Type: slack.RTSEText, Text: "x"},
	}}
	rt := RichTextBlock{Elements: []slack.RichTextElement{
		&slack.RichTextSection{Type: slack.RTESection, Elements: []slack.RichTextSectionElement{
			&slack.RichTextSectionTextElement{Type: slack.RTSEText, Text: "para"},
		}},
		&slack.RichTextList{Type: slack.RTEList, Style: slack.RTEListBullet, Indent: 2, Elements: []slack.RichTextElement{item}},
	}}
	if got, want := RichTextToMarkdown(rt), "para\n- x"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// Saving an edit of literal mention-shaped text (here in plain text and
// in code) sends no mention: the composer has no escape for a literal
// token, so the text keeps Slack's escaped spelling instead of pinging.
func TestRichTextToMarkdown_LiteralMentionDoesNotPingOnSave(t *testing.T) {
	rt := rtSection(
		&slack.RichTextSectionTextElement{Type: slack.RTSEText, Text: "<!here> <@U1> "},
		&slack.RichTextSectionTextElement{Type: slack.RTSEText, Text: "<!channel>", Style: &slack.RichTextSectionTextStyle{Code: true}},
	)
	_, block := mrkdwn.Convert(RichTextToMarkdown(rt))
	for _, e := range block.Elements {
		sec, ok := e.(*slack.RichTextSection)
		if !ok {
			continue
		}
		for _, el := range sec.Elements {
			switch el.(type) {
			case *slack.RichTextSectionBroadcastElement, *slack.RichTextSectionUserElement:
				t.Errorf("saved edit has a mention element %#v", el)
			}
		}
	}
}

// Editing a message keeps its links to other messages; they go back out
// as links on save.
func TestRichTextToMarkdown_MessageMention(t *testing.T) {
	const url = "https://example.slack.com/archives/C0000TEST1/p1700000000000100"
	for _, tc := range []struct {
		name, label, want string
	}{
		{"label is url", url, url},
		{"other label", "earlier", "[earlier](" + url + ")"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt := richTextFromJSON(t, `{"type":"rich_text","elements":[{"type":"rich_text_section","elements":[`+
				`{"type":"text","text":"see "},`+
				`{"type":"message_mention","message_ts":"1700000000.000100","channel_id":"C0000TEST1",`+
				`"text":"`+tc.label+`","url":"`+url+`"}]}]}`)
			if got, want := RichTextToMarkdown(rt), "see "+tc.want; got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		})
	}
}

// A link is written bare only when Convert links the whole bare URL
// back: one the typed-URL rule would cut (at a non-ASCII character or a
// space) keeps its explicit form, so saving keeps the whole target.
func TestLinkToMarkdown_BareOnlyWhenWholeURLSurvives(t *testing.T) {
	for _, tc := range []struct {
		url      string
		wantBare bool
	}{
		{"https://x.example/a.txt", true},
		{"ftp://x.example/a.txt", true},
		{"https://x.example/résumé.txt", false},
		{"ftp://x.example/my file.txt", false},
		{"ftp://x.example/日本.txt", false},
	} {
		got := LinkToMarkdown(tc.url, "")
		if bare := got == tc.url; bare != tc.wantBare {
			t.Errorf("LinkToMarkdown(%q) = %q, bare=%v want %v", tc.url, got, bare, tc.wantBare)
		}
		_, block := mrkdwn.Convert(got)
		var targets []string
		for _, e := range block.Elements {
			for _, se := range e.(*slack.RichTextSection).Elements {
				if l, ok := se.(*slack.RichTextSectionLinkElement); ok {
					targets = append(targets, l.URL)
				}
			}
		}
		if len(targets) != 1 || targets[0] != tc.url {
			t.Errorf("saving %q links %q, want %q", got, targets, tc.url)
		}
	}
}
