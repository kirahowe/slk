package messages

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/gammons/slk/internal/proptest"
	"pgregory.net/rapid"
)

// draftPaneRows is the channel pane's own row for msg, the only message
// in it, drawn at width: the entry's unselected lines, and how many of
// them the author header takes.
func draftPaneRows(t *rapid.T, m *Model, msg MessageItem, width int) (rows []string, headerRows int) {
	m.SetMessages([]MessageItem{msg})
	m.View(200, width)
	for _, e := range m.list.Entries() {
		if e.ID == msg.TS {
			r := RenderMessage(msg, width, m.renderContext(), m.rowOptions(msg))
			return e.Lines, strings.Count(r.Header, "\n") + 1
		}
	}
	t.Fatalf("no entry for %s", msg.TS)
	return nil, 0
}

// draftMessage is a message the user typed, as a send stores it
// (TypedBody), from alice.
func draftMessage(t *rapid.T) MessageItem {
	typed := proptest.NoVS16(proptest.ComposerText([]string{"U0BOB00001"}, []string{"C0GENERAL1"}).Draw(t, "typed"))
	msg := MessageItem{TS: "1700000000.000100", UserID: "U0ALICE001", UserName: "alice", Timestamp: "9:00 AM"}
	TypedBody(typed).Apply(&msg)
	return msg
}

// A draft is drawn exactly as the pane draws the sent message's row
// below its header: the same lines, byte for byte, with no avatar; and
// the same cells right of the avatar gutter with one.
func TestRenderDraftIsThePaneRowBelowItsHeader(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		m := New(nil, "general")
		m.SetUserNames(map[string]string{"U0BOB00001": "bob", "U0ALICE001": "alice"})
		avatar := rapid.Bool().Draw(t, "avatar")
		if avatar {
			m.SetAvatarFunc(func(string) string { return "████\n████" })
		}
		width := rapid.IntRange(30, 120).Draw(t, "width")
		msg := draftMessage(t)

		got := m.RenderDraft(msg, width).Rows
		rows, header := draftPaneRows(t, m, msg, width)
		want := rows[header:]
		if len(got) != len(want) {
			t.Fatalf("draft has %d rows, the pane row %d below its header\ndraft: %q\nrow:   %q", len(got), len(want), got, want)
		}
		for i := range want {
			g, w := got[i], want[i]
			if avatar {
				// The avatar's second row is beside the first body row.
				g, w = ansi.Strip(ansi.Cut(g, 6, width)), ansi.Strip(ansi.Cut(w, 6, width))
			}
			if g != w {
				t.Fatalf("row %d differs\ndraft: %q\nrow:   %q", i, g, w)
			}
		}
	})
}

// Every row of a draft is exactly the width it is drawn at, also where
// that is narrower than the renderer's 20-cell floor, for text with
// wide and combining runes.
func TestRenderDraftRowsAreTheWidth(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		m := New(nil, "general")
		if rapid.Bool().Draw(t, "avatar") {
			m.SetAvatarFunc(func(string) string { return "████\n████" })
		}
		width := rapid.IntRange(10, 120).Draw(t, "width")
		for i, row := range m.RenderDraft(draftMessage(t), width).Rows {
			if w := ansi.StringWidth(row); w != width {
				t.Fatalf("row %d is %d cells wide, want %d: %q", i, w, width, row)
			}
		}
	})
}

// BodyOnly drops the label rows and the header, whatever the options.
func TestRenderMessageBodyOnlyHasNoHeader(t *testing.T) {
	msg := MessageItem{
		TS: "1.000001", UserID: "U1", UserName: "alice", Timestamp: "3:04 PM",
		Text: "ship it", Ephemeral: true, Subtype: "thread_broadcast", IsEdited: true,
	}
	rc := New(nil, "general").renderContext()
	opt := RenderOptions{BodyOnly: true, EditedMark: true, BroadcastLabel: true, Avatar: "████\n████"}
	got := ansi.Strip(RenderMessage(msg, 60, rc, opt).Content)
	if strings.TrimSpace(got) != "ship it" {
		t.Fatalf("body only = %q, want the body alone", got)
	}
	for _, bad := range []string{"alice", "3:04 PM", "edited", "replied", "█"} {
		if strings.Contains(got, bad) {
			t.Errorf("body only %q has %q", got, bad)
		}
	}
	if !strings.HasPrefix(got, strings.Repeat(" ", 5)+"ship it") {
		t.Errorf("body only %q does not keep the avatar's gutter", got)
	}
}
