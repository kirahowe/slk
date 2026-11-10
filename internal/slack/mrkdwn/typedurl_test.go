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
