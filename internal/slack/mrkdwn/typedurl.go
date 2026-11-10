package mrkdwn

import (
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
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
// typedURLRegexp is the one rule for what a typed URL is.

// typedURLRegexp tells Linkify what a bare typed URL is: http, https or
// ftp (the schemes goldmark's Linkify links by default) followed by
// printable ASCII other than < > and |, ending on a character that does
// not close a sentence or a quote. goldmark's own pattern needs a dotted
// host with a TLD, so localhost and IP hosts stayed text. ASCII only,
// like goldmark's, so a match never swallows the tokenizer's private-use
// sentinels (a mention, channel or emoji next to the URL) or the CJK text
// after it. The last character may be ')': Linkify then drops the closing
// parens the URL did not open. www. matches stay with goldmark.
var typedURLRegexp = regexp.MustCompile(`^(?:https?|ftp)://[!-;=?-{}~]*[#-&()+\-/-9=@-\[\\^` + "`" + `a-{}]`)

// typedURLProtocols are the "scheme:" prefixes Linkify may start a match
// at, the schemes typedURLRegexp names.
var typedURLProtocols = []string{"http:", "https:", "ftp:"}

// typedURLLen returns the length of the typed URL at the start of s, as
// Linkify would link it, or 0 when s does not start with one. It is
// typedURLRegexp with Linkify's closing-paren balancing; the pattern's
// last character already excludes the punctuation Linkify trims.
func typedURLLen(s string) int {
	m := typedURLRegexp.FindString(s)
	for strings.HasSuffix(m, ")") && strings.Count(m, ")") > strings.Count(m, "(") {
		m = m[:len(m)-1]
	}
	return len(m)
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
	if len(line) < 2 || typedURLLen(string(line[1:])) == 0 {
		return nil
	}
	block.Advance(1)
	return ast.NewTextSegment(seg.WithStop(seg.Start + 1))
}
