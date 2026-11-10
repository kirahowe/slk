package messages

import (
	"strings"
	"testing"

	"github.com/gammons/slk/internal/proptest"
	"github.com/gammons/slk/internal/slackurl"
	"github.com/gammons/slk/internal/ui/messages/blockkit"
	"pgregory.net/rapid"
)

// MessageCopyText never loses what the message shows as its body: the
// result starts with MessageTextSource. After it come, in order and
// each once, the texts of the attachments that share a Slack message
// (FromURL is a permalink), and nothing from any other attachment, so a
// link unfurl is never copied as if it had been typed. A message with
// no body text copies its first attachment that has any text, and one
// with neither copies nothing.
func TestMessageCopyText(t *testing.T) {
	users := []string{"U0SELF0001", "U0OTHER001"}
	channels := []string{"C0GENERAL1"}
	fromURLs := []string{
		"",
		"https://example.com/article",
		"https://acme.slack.com/archives/C0GENERAL1/p1700000000000100",
	}
	rapid.Check(t, func(t *rapid.T) {
		msg := MessageItem{TS: proptest.TS().Draw(t, "ts")}
		if rapid.Bool().Draw(t, "hasText") {
			msg.Text = proptest.SlackText(users, channels).Draw(t, "text")
		}
		for i := range rapid.IntRange(0, 3).Draw(t, "attachments") {
			la := blockkit.LegacyAttachment{FromURL: rapid.SampledFrom(fromURLs).Draw(t, "fromURL")}
			if rapid.Bool().Draw(t, "attText") {
				// Numbered, so two attachments never share a text.
				la.Text = rapid.StringMatching(`shared [a-z]{1,8}`).Draw(t, "shared") + " #" + string(rune('0'+i))
			}
			if rapid.Bool().Draw(t, "attTitle") {
				la.Title = rapid.StringMatching(`Title [a-z]{1,8}`).Draw(t, "title") + " #" + string(rune('0'+i))
			}
			msg.LegacyAttachments = append(msg.LegacyAttachments, la)
		}

		got := MessageCopyText(msg)
		body := MessageTextSource(msg)
		if body == "" {
			want := ""
			for _, la := range msg.LegacyAttachments {
				if want = legacyAttachmentText(la); want != "" {
					break
				}
			}
			if got != want {
				t.Fatalf("no body text: got %q, want the first attachment's text %q", got, want)
			}
			return
		}
		rest, ok := strings.CutPrefix(got, body)
		if !ok {
			t.Fatalf("copy text %q does not start with the body %q", got, body)
		}
		for _, la := range msg.LegacyAttachments {
			shared := legacyAttachmentText(la)
			_, isShare := slackurl.Parse(la.FromURL)
			if !isShare || shared == "" {
				if shared != "" && strings.Contains(rest, shared) {
					t.Fatalf("copy text %q holds %q from an attachment that shares no message", got, shared)
				}
				continue
			}
			rest, ok = strings.CutPrefix(rest, "\n\n"+shared)
			if !ok {
				t.Fatalf("copy text %q is missing the shared text %q in its place", got, shared)
			}
		}
		if rest != "" {
			t.Fatalf("copy text %q ends with %q, which no attachment accounts for", got, rest)
		}
	})
}
