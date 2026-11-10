package thread

import (
	"strings"
	"testing"

	"github.com/gammons/slk/internal/proptest"
	"github.com/gammons/slk/internal/ui/messages"
	"pgregory.net/rapid"
)

// A draft reply is drawn exactly as the thread pane draws the sent
// reply's row below its header, byte for byte. The pane draws no
// avatar, whatever its avatar function.
func TestRenderDraftIsTheReplyRowBelowItsHeader(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		m := New()
		m.SetUserNames(map[string]string{"U0BOB00001": "bob", "U0ALICE001": "alice"})
		m.SetAvatarFunc(func(string) string { return "████\n████" })
		width := rapid.IntRange(30, 120).Draw(t, "width")
		typed := proptest.NoVS16(proptest.ComposerText([]string{"U0BOB00001"}, []string{"C0GENERAL1"}).Draw(t, "typed"))
		parent := messages.MessageItem{TS: "1700000000.000100", UserID: "U0BOB00001", UserName: "bob", Text: "parent", Timestamp: "8:00 AM"}
		reply := messages.MessageItem{TS: "1700000000.000200", UserID: "U0ALICE001", UserName: "alice", Timestamp: "9:00 AM", ThreadTS: parent.TS}
		messages.TypedBody(typed).Apply(&reply)

		got := m.RenderDraft(reply, width).Rows
		m.SetThread(parent, []messages.MessageItem{reply}, "C1", parent.TS)
		m.View(200, width)
		var row []string
		for _, e := range m.list.Entries() {
			if e.ID == reply.TS {
				row = e.Lines
			}
		}
		header := strings.Count(m.renderMessage(reply, width, false).Header, "\n") + 1
		if len(row) < header {
			t.Fatalf("no row for the reply (%d lines)", len(row))
		}
		want := row[header:]
		if len(got) != len(want) {
			t.Fatalf("draft has %d rows, the reply row %d below its header\ndraft: %q\nrow:   %q", len(got), len(want), got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("row %d differs\ndraft: %q\nrow:   %q", i, got[i], want[i])
			}
		}
	})
}
