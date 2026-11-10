package mrkdwn

import (
	"reflect"
	"strings"
	"testing"

	"github.com/slack-go/slack"
)

// linkURLs returns the URL of every link element in blk, in order: in
// sections, quotes and list items.
func linkURLs(blk *slack.RichTextBlock) []string {
	var out []string
	walk := func(els []slack.RichTextSectionElement) {
		for _, el := range els {
			if l, ok := el.(*slack.RichTextSectionLinkElement); ok {
				out = append(out, l.URL)
			}
		}
	}
	if blk == nil {
		return nil
	}
	for _, e := range blk.Elements {
		switch e := e.(type) {
		case *slack.RichTextSection:
			walk(e.Elements)
		case *slack.RichTextQuote:
			walk(e.Elements)
		case *slack.RichTextList:
			for _, s := range e.Elements {
				if sec, ok := s.(*slack.RichTextSection); ok {
					walk(sec.Elements)
				}
			}
		}
	}
	return out
}

// A typed URL goes out as a link element: Slack's clients draw the block
// and do not linkify its text. goldmark's own matcher wants a dotted host
// with a TLD, so localhost and IP hosts stayed text.
func TestConvert_TypedURLs(t *testing.T) {
	cases := []struct {
		name, in string
		want     []string
	}{
		{"ip host with port", "see http://192.0.2.3:8080/browse/X-1 ok", []string{"http://192.0.2.3:8080/browse/X-1"}},
		{"https ip host", "https://192.0.2.3:8443/a", []string{"https://192.0.2.3:8443/a"}},
		{"localhost", "http://localhost:8080", []string{"http://localhost:8080"}},
		{"localhost path", "http://localhost:3000/x", []string{"http://localhost:3000/x"}},
		{"ftp ip host", "ftp://10.0.0.1/file.txt.", []string{"ftp://10.0.0.1/file.txt"}},
		{"ftp", "ftp://example.com/f", []string{"ftp://example.com/f"}},
		{"trailing period", "see http://192.0.2.3/a.", []string{"http://192.0.2.3/a"}},
		{"trailing comma", "http://localhost:8080/path?q=1, then", []string{"http://localhost:8080/path?q=1"}},
		{"trailing paren", "(http://192.0.2.3/a)", []string{"http://192.0.2.3/a"}},
		{"unbalanced paren then period", "https://example.com/x). Next", []string{"https://example.com/x"}},
		{"balanced trailing paren", "https://en.wikipedia.org/wiki/Mercury_(planet)", []string{"https://en.wikipedia.org/wiki/Mercury_(planet)"}},
		{"balanced paren inside parens", "(see https://en.wikipedia.org/wiki/Mercury_(planet))", []string{"https://en.wikipedia.org/wiki/Mercury_(planet)"}},
		{"query and fragment", "https://example.com/a?x=1&y=2#frag", []string{"https://example.com/a?x=1&y=2#frag"}},
		{"trailing semicolon bang question", "http://192.0.2.3/a; http://192.0.2.4/b! http://192.0.2.5/c?",
			[]string{"http://192.0.2.3/a", "http://192.0.2.4/b", "http://192.0.2.5/c"}},
		{"followed by a mention", "http://localhost:3000/a,<@UFAKE0001>", []string{"http://localhost:3000/a"}},
		{"followed by a channel", "http://localhost:3000/a<#CFAKE0001|general>", []string{"http://localhost:3000/a"}},
		{"followed by an emoji", "http://example.com/x:tada:", []string{"http://example.com/x"}},
		{"followed by CJK text", "https://example.com/aを確認してください", []string{"https://example.com/a"}},
		{"followed by full-width paren", "https://example.com/a）", []string{"https://example.com/a"}},
		{"pipe ends the url", "http://a.example|other", []string{"http://a.example"}},
		{"www", "visit www.example.com", []string{"http://www.example.com"}},
		{"email", "mail a@b.co", []string{"mailto:a@b.co"}},
		{"code span", "`http://192.0.2.3/a`", nil},
		{"code block", "```\nhttp://192.0.2.3/a\n```", nil},
		{"markdown link", "[x](http://192.0.2.3/a)", []string{"http://192.0.2.3/a"}},
		// Nothing to open.
		{"scheme only", "http://", nil},
		{"scheme then space", "http:// x", nil},
		// Schemes Convert does not link.
		{"javascript", "javascript:alert(1)", nil},
		{"data", "data:text/html,hi", nil},
		{"file", "open file:///tmp/a.txt", nil},
		{"tel", "call tel:+15550100", nil},
		{"ssh", "ssh://192.0.2.3", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, blk := Convert(c.in)
			got := linkURLs(blk)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("Convert(%q) links = %q, want %q; block=%s", c.in, got, c.want, blockJSON(blk))
			}
			for _, u := range got {
				if hasSentinel(u) {
					t.Errorf("link URL %q holds a tokenizer sentinel", u)
				}
			}
		})
	}
}

// The mrkdwn fallback carries a typed URL in Slack's <url> form, so the
// link survives wherever the text is drawn instead of the block.
func TestConvert_TypedURLFallback(t *testing.T) {
	mr, _ := Convert("see http://localhost:8080/x, ok")
	if want := "see <http://localhost:8080/x>, ok"; mr != want {
		t.Errorf("mrkdwn = %q, want %q", mr, want)
	}
	if strings.Contains(mr, "|") {
		t.Errorf("mrkdwn %q labels a link whose label is its URL", mr)
	}
}

// Typed URLs in the places Linkify does not reach on its own: after a
// quote mark or a colon, and in a quote inside a quote. "see:https://x"
// also went out with the emoji :https: in it, the tokenizer having read
// the scheme's colon as the emoji's closing one.
func TestConvert_TypedURLGaps(t *testing.T) {
	cases := []struct {
		in    string
		mr    string
		links []string
	}{
		{`"https://example.com"`, `"<https://example.com>"`, []string{"https://example.com"}},
		{`'https://example.com/x'`, `'<https://example.com/x>'`, []string{"https://example.com/x"}},
		{"see:https://x.y", "see:<https://x.y>", []string{"https://x.y"}},
		{"see:ftp://10.0.0.1/f", "see:<ftp://10.0.0.1/f>", []string{"ftp://10.0.0.1/f"}},
		{`he said "see http://localhost:8080".`, `he said "see <http://localhost:8080>".`, []string{"http://localhost:8080"}},
		// A quote inside a quote keeps its own "> " marker as text, and
		// each of its lines is its own text, so URLs on adjacent lines
		// stay two links.
		{"> > nested https://example.com", "> > nested <https://example.com>", []string{"https://example.com"}},
		{"> > https://one.example\n> > https://two.example", "> > <https://one.example>\n> > <https://two.example>", []string{"https://one.example", "https://two.example"}},
		{"> > https://one.example\n> >\n> > https://two.example", "> > <https://one.example>\n> >\n> > <https://two.example>", []string{"https://one.example", "https://two.example"}},
		{"> > > deep http://localhost:3000/x.", "> > > deep <http://localhost:3000/x>.", []string{"http://localhost:3000/x"}},
		{"> - item https://example.com", "> - item <https://example.com>", []string{"https://example.com"}},
		{"> > mail a@b.co", "> > mail <mailto:a@b.co|a@b.co>", []string{"mailto:a@b.co"}},
		// Already linked: a heading is a paragraph here, and a quote's
		// own paragraphs are walked inline.
		{"# Title https://example.com", "# Title <https://example.com>", []string{"https://example.com"}},
		{"# see (https://example.com/x)", "# see (<https://example.com/x>)", []string{"https://example.com/x"}},
		{"> quote https://example.com", "> quote <https://example.com>", []string{"https://example.com"}},
		{"> https://one.example\n> https://two.example", "> <https://one.example>\n> <https://two.example>", []string{"https://one.example", "https://two.example"}},
		// Still not links.
		{"foo:bar", "foo:bar", nil},
		{`say "hi"`, `say "hi"`, nil},
		{"#xhttps://example.com", "#xhttps://example.com", nil},
		{"> > `https://example.com`", "> > `https://example.com`", nil},
		{"> ```\n> https://example.com\n> ```", "> ```\n> https://example.com\n> ```", nil},
		// An emoji next to a URL is still an emoji.
		{":smile: https://example.com", ":smile: <https://example.com>", []string{"https://example.com"}},
	}
	for _, c := range cases {
		mr, blk := Convert(c.in)
		if mr != c.mr {
			t.Errorf("Convert(%q) mrkdwn = %q, want %q", c.in, mr, c.mr)
		}
		if got := linkURLs(blk); !reflect.DeepEqual(got, c.links) {
			t.Errorf("Convert(%q) links = %q, want %q; block=%s", c.in, got, c.links, blockJSON(blk))
		}
	}
}

// "see:https://x" holds no emoji: the colon after "https" is the
// scheme's. An emoji whose name is not followed by a typed URL is one.
func TestTokenize_SchemeColonIsNotAnEmoji(t *testing.T) {
	for _, in := range []string{"see:https://x.y", "a:http://localhost", "b:ftp://h/f"} {
		if _, table := tokenize(in); len(table) != 0 {
			t.Errorf("tokenize(%q) table = %+v, want none", in, table)
		}
	}
	for _, in := range []string{":https: x", "x:https:y", "a:tada:b"} {
		if _, table := tokenize(in); len(table) != 1 || table[0].kind != tokEmoji {
			t.Errorf("tokenize(%q) table = %+v, want one emoji", in, table)
		}
	}
}

// A blank line between a quote's paragraphs is kept on both sides.
func TestConvert_QuoteBlankLine(t *testing.T) {
	mr, blk := Convert("> one\n>\n> two")
	if want := "> one\n>\n> two"; mr != want {
		t.Errorf("mrkdwn = %q, want %q", mr, want)
	}
	q, ok := blk.Elements[0].(*slack.RichTextQuote)
	if !ok {
		t.Fatalf("element is %T, want *RichTextQuote; block=%s", blk.Elements[0], blockJSON(blk))
	}
	var text strings.Builder
	for _, e := range q.Elements {
		if te, ok := e.(*slack.RichTextSectionTextElement); ok {
			text.WriteString(te.Text)
		}
	}
	if want := "one\n\ntwo"; text.String() != want {
		t.Errorf("quote text = %q, want %q; block=%s", text.String(), want, blockJSON(blk))
	}
}
