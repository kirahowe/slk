package mrkdwn

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// A typed URL is a URL someone typed as bare text. It has to go out as a
// link element: Slack's clients draw the rich_text block and do not
// linkify its text. Linkify finds them inside paragraphs and list items;
// the rest of this file covers the places it does not reach:
//
//   - Linkify starts a URL only at a line head or after a space, '*',
//     '_', '~' or '(', so one after a quote mark or a colon stayed text
//     (afterPunctURL);
//   - the tokenizer runs first and read "see:https://x" as the emoji
//     :https: (tokens.go, through typedURLLen).
//
// typedURLRegexp and typedURLLen are the one rule for what a typed URL
// is, and typedURLLinkify holds Linkify to it.

// typedURLRegexp tells Linkify what a bare typed URL is: http, https or
// ftp (the schemes goldmark's Linkify links by default) followed by
// printable ASCII other than < > and |, ending on a character that does
// not close a sentence or a quote. goldmark's own pattern needs a dotted
// host with a TLD, so localhost and IP hosts stayed text. ASCII only,
// like goldmark's, so a match never swallows the tokenizer's private-use
// sentinels (a mention, channel or emoji next to the URL) or the CJK text
// after it. The last character may be ')' or ']': typedURLLen then drops
// the closers the URL did not open. www. matches stay with goldmark.
var typedURLRegexp = regexp.MustCompile(`^(?:https?|ftp)://[!-;=?-{}~]*[#-&()+\-/-9=@-\[\\\]^` + "`" + `a-{}]`)

// typedURLProtocols are the "scheme:" prefixes Linkify may start a match
// at, the schemes typedURLRegexp names.
var typedURLProtocols = []string{"http:", "https:", "ftp:"}

// typedURLFinal[c] is whether typedURLRegexp lets a URL end on the ASCII
// byte c, read off the pattern so the two cannot disagree.
var typedURLFinal = func() (final [128]bool) {
	for c := range final {
		final[c] = len(typedURLRegexp.FindString("http://"+string(rune(c)))) == len("http://")+1
	}
	return final
}()

// typedURLLen returns the length of the typed URL at the start of s, as
// Convert links it, or 0 when s does not start with one. It is the
// typedURLRegexp match, less any trailing ')' or ']' the URL holds more
// of than of '(' or '[' (counted over the whole URL), each dropped one
// at a time from the end, and less the characters the pattern does not
// let a URL end on that dropping a closer leaves last. So
// "http://[::1]" and "https://h/(x/(y))" keep their closers, and in
// "[https://h/y]" the URL does not take the ']'. The result is again a
// match of the pattern, or 0 when only the scheme is left.
func typedURLLen(s string) int { return typedURLSpan(s) }

// typedURLLenBytes is typedURLLen over bytes, for a parser reading
// goldmark's line in place: it neither copies b nor keeps it.
func typedURLLenBytes(b []byte) int { return typedURLSpan(b) }

// typedURLSpan is typedURLLen for either form of text.
func typedURLSpan[T string | []byte](s T) int {
	// The regexp's backtracker clears state for the whole of s on every
	// call, so a call where no scheme starts would cost the length of
	// the line, at every quote mark and colon in it.
	if !hasTypedURLScheme(s) {
		return 0
	}
	var n int
	switch s := any(s).(type) {
	case string:
		n = len(typedURLRegexp.FindString(s))
	case []byte:
		n = len(typedURLRegexp.Find(s))
	}
	if n == 0 {
		return 0
	}
	m := s[:n]
	start := 0
	for m[start] != '/' {
		start++
	}
	start += len("//")
	var parens, brackets int
	for i := start; i < n; i++ {
		switch m[i] {
		case '(':
			parens--
		case ')':
			parens++
		case '[':
			brackets--
		case ']':
			brackets++
		}
	}
	for ; n > start; n-- {
		switch c := m[n-1]; {
		case c == ')' && parens > 0:
			parens--
		case c == ']' && brackets > 0:
			brackets--
		case !typedURLFinal[c]:
		default:
			return n
		}
	}
	return 0
}

// hasTypedURLScheme reports whether s starts with one of
// typedURLProtocols.
func hasTypedURLScheme[T string | []byte](s T) bool {
	for _, proto := range typedURLProtocols {
		if len(s) >= len(proto) && string(s[:len(proto)]) == proto {
			return true
		}
	}
	return false
}

// typedURLLinkify is goldmark's Linkify parser held to typedURLLen. Where
// a typed URL starts, it shows Linkify the line only up to the URL's end,
// so Linkify, which balances ')' but not ']', links exactly what
// typedURLLen measures. A scheme followed by nothing typedURLLen counts,
// as in "http://)", is left as text. Elsewhere (www. names, email
// addresses) Linkify sees the whole line.
type typedURLLinkify struct{ parser.InlineParser }

func newTypedURLLinkify() typedURLLinkify {
	return typedURLLinkify{extension.NewLinkifyParser(
		extension.WithLinkifyURLRegexp(typedURLRegexp),
		extension.WithLinkifyAllowedProtocols(typedURLProtocols),
	)}
}

func (p typedURLLinkify) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, _ := block.PeekLine()
	// Linkify starts its match one byte in after these, as here.
	lead := 0
	if len(line) > 0 && strings.IndexByte(" *_~(", line[0]) >= 0 {
		lead = 1
	}
	rest := line[lead:]
	if !hasTypedURLScheme(rest) {
		return p.InlineParser.Parse(parent, block, pc)
	}
	if n := typedURLLenBytes(rest); n > 0 {
		return p.InlineParser.Parse(parent, cutLine{block, lead + n}, pc)
	}
	return nil
}

// cutLine is a text.Reader whose current line ends stop bytes in.
type cutLine struct {
	text.Reader
	stop int
}

func (r cutLine) PeekLine() ([]byte, text.Segment) {
	line, seg := r.Reader.PeekLine()
	return line[:r.stop], seg.WithStop(seg.Start + r.stop)
}

// afterPunctURL lets Linkify start a URL after a quote mark or a colon.
// goldmark offers inline parsers only punctuation and whitespace
// positions, and Linkify triggers on few of them. This parser takes the
// mark as text when a typed URL follows it; goldmark then offers the
// position right after it to Linkify, as the start of a run.
type afterPunctURL struct{}

func (afterPunctURL) Trigger() []byte { return []byte{'"', '\'', ':'} }

func (afterPunctURL) Parse(_ ast.Node, block text.Reader, _ parser.Context) ast.Node {
	line, seg := block.PeekLine()
	if len(line) < 2 || typedURLLenBytes(line[1:]) == 0 {
		return nil
	}
	block.Advance(1)
	return ast.NewTextSegment(seg.WithStop(seg.Start + 1))
}
