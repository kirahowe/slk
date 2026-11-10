package mrkdwn

import "regexp"

// A typed URL is a URL someone typed as bare text. It has to go out as a
// link element: Slack's clients draw the rich_text block and do not
// linkify its text.

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
