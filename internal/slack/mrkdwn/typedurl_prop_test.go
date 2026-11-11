package mrkdwn

import (
	"strings"
	"testing"

	"github.com/gammons/slk/internal/proptest"
	"pgregory.net/rapid"
)

// bracketedURL draws a typed URL whose path may hold '[', ']', '(' and
// ')' anywhere, matched or not, and that ends in a letter or digit, so
// all of it is the URL.
func bracketedURL() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		scheme := rapid.SampledFrom([]string{"http://", "https://", "ftp://"}).Draw(t, "scheme")
		host := rapid.SampledFrom([]string{"a.b", "localhost", "[::1]", "192.0.2.3:80"}).Draw(t, "host")
		path := rapid.StringMatching(`(/[a-z0-9\[\]()]{0,6}[a-z0-9]){0,3}`).Draw(t, "path")
		return scheme + host + path
	})
}

// A run of closing brackets or parens after a typed URL is kept one for
// each opener of its kind the URL left unmatched (counting every opener
// and closer of that kind in it), and the rest is text: typedURLLen and
// the link Convert makes agree on that.
func TestTypedURLBalancesClosers(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		url := bracketedURL().Draw(t, "url")
		pair := rapid.SampledFrom([]string{"[]", "()"}).Draw(t, "pair")
		opener, closer := pair[:1], pair[1:]
		run := rapid.IntRange(1, 4).Draw(t, "run")
		unmatched := max(0, strings.Count(url, opener)-strings.Count(url, closer))
		want := url + strings.Repeat(closer, min(run, unmatched))
		s := url + strings.Repeat(closer, run)
		if got := s[:typedURLLen(s)]; got != want {
			t.Fatalf("typedURLLen(%q) takes %q, want %q", s, got, want)
		}
		in := "see " + s + " ok"
		if _, blk := Convert(in); !equalStrings(linkURLs(blk), []string{want}) {
			t.Fatalf("Convert(%q) links = %q, want [%q]", in, linkURLs(blk), want)
		}
	})
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// typedURLLen and typedURLLenBytes measure the same URL in any text:
// Slack wire text, composer text or plain text, after a typed URL or
// on its own.
func TestTypedURLLenBytesAgrees(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		text := rapid.OneOf(
			proptest.SlackText(nil, nil),
			proptest.ComposerText(nil, nil),
			proptest.PlainText(),
		).Draw(t, "text")
		prefix := rapid.SampledFrom([]string{"", "http://", "https://a.b/", "ftp://[::1]/("}).Draw(t, "prefix")
		var url string
		if rapid.Bool().Draw(t, "url") {
			url = rapid.OneOf(typedURL(), bracketedURL()).Draw(t, "typedURL")
		}
		s := prefix + url + text
		if got, want := typedURLLenBytes([]byte(s)), typedURLLen(s); got != want {
			t.Fatalf("typedURLLenBytes(%q) = %d, typedURLLen = %d", s, got, want)
		}
	})
}
