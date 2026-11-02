package messages

import "testing"

// Without a rich_text block the text field is all there is: its links
// read as markdown and Slack's escapes are decoded.
func TestEditSource_TextOnly(t *testing.T) {
	for _, tc := range []struct{ text, want string }{
		{"plain", "plain"},
		{"a &lt; b &amp;&amp; c &gt; d", "a < b && c > d"},
		{"see <https://x.com/a?b=1&amp;c=2|the docs> now", "see [the docs](https://x.com/a?b=1&c=2) now"},
		{"bare <https://x.com/>", "bare https://x.com/"},
		{"mail <mailto:a@b.com|a@b.com>", "mail a@b.com"},
		{"hi <@U1> in <#C2>", "hi <@U1> in <#C2>"},
		{"&lt;https://x.com/|not a link&gt;", "<https://x.com/|not a link>"},
	} {
		if got := EditSource(MessageItem{Text: tc.text}); got != tc.want {
			t.Errorf("EditSource(%q) = %q, want %q", tc.text, got, tc.want)
		}
	}
}
