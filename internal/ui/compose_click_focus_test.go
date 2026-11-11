package ui

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gammons/slk/internal/core"
	"github.com/gammons/slk/internal/ids"
	"github.com/gammons/slk/internal/ui/messages"
)

// sendCall is what the fake message and thread services in
// typingClickApp return for a send: which service was called, where,
// and with what text.
type sendCall struct {
	reply     bool
	edit      bool
	channelID string
	threadTS  string
	text      string
}

// typingClickApp is a 160x30 App on C1 showing three messages, with the
// thread 0.5 in #general open beside it (replies "reply-1".."reply-4"), and
// "channel draft" / "thread draft" in the two composers. Sends and
// replies return a sendCall.
func typingClickApp(t *testing.T) *App {
	t.Helper()
	a := newTestApp(t, withSize(160, 30), withActiveChannel("C1"), withMessages(testMessageItems(3)...))
	replies := make([]messages.MessageItem, 0, 4)
	for i := 1; i <= 4; i++ {
		replies = append(replies, messages.MessageItem{
			TS: fmt.Sprintf("0.%d", 5+i), UserID: "U1", UserName: "alice",
			Text: fmt.Sprintf("reply-%d", i), ThreadTS: "0.5", Timestamp: "1:00 PM",
		})
	}
	parent := messages.MessageItem{TS: "0.5", UserName: "alice", Text: "parent", ThreadTS: "0.5"}
	a.threadPanel.SetThread(parent, replies, "C1", "0.5")
	a.threadPanel.SetBreadcrumb("general", "channel")
	a.threadVisible = true
	a.compose.SetValue("channel draft")
	a.threadCompose.SetValue("thread draft")
	a.SetMessageService(core.NewMessageService(core.MessageServiceFuncs{
		Send: func(channelID ids.ChannelID, text string) core.Msg {
			return sendCall{channelID: string(channelID), text: text}
		},
		Edit: func(channelID ids.ChannelID, ts ids.MessageTS, text string) core.Msg {
			return sendCall{edit: true, channelID: string(channelID), threadTS: string(ts), text: text}
		},
	}))
	a.SetThreadService(core.NewThreadService(core.ThreadServiceFuncs{
		SendReply: func(channelID ids.ChannelID, threadTS ids.ThreadTS, text string, _ bool) core.Msg {
			return sendCall{reply: true, channelID: string(channelID), threadTS: string(threadTS), text: text}
		},
	}))
	_ = a.View()
	return a
}

// clickAndRelease is a plain left click (press, then release in the
// same cell) through Update, rendering after each.
func clickAndRelease(t *testing.T, a *App, x, y int) {
	t.Helper()
	clickAt(t, a, x, y)
	updateAndRender(t, a, tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft})
}

// typeThenSend types "!" and presses Enter through Update, and returns
// the send that Enter made.
func typeThenSend(t *testing.T, a *App) sendCall {
	t.Helper()
	updateAndRender(t, a, keyPress('!'))
	_, cmd := a.Update(keyCode(tea.KeyEnter))
	_ = a.View()
	return deliveredMsgs[sendCall](t, cmd)[0]
}

// While a composer has the keyboard, a click anywhere else on the main
// screen leaves the keyboard with it: the next character lands in that
// composer, Enter sends that composer's text to its conversation, and
// the other composer's draft stays as it was, unsent.
func TestClickWhileTyping_KeyboardStaysWithTheComposer(t *testing.T) {
	type click struct {
		name string
		at   func(t *testing.T, a *App) (x, y int)
	}
	channelCompose := click{"the channel compose box", func(t *testing.T, a *App) (int, int) {
		return a.layout.sidebarEnd + 5, 1 + a.layout.msgComposeTop + 1
	}}
	channelRow := click{"a channel message row", func(t *testing.T, a *App) (int, int) {
		return a.layout.sidebarEnd + 5, screenRowOf(t, a, "msg-2", a.layout.sidebarEnd)
	}}
	sidebarRow := click{"the sidebar's Threads row", func(t *testing.T, a *App) (int, int) {
		return a.layout.RailWidth() + 5, screenRowOf(t, a, "Threads", a.layout.RailWidth())
	}}
	threadCompose := click{"the thread compose box", func(t *testing.T, a *App) (int, int) {
		return a.layout.msgEnd + 5, 1 + a.layout.threadComposeTop + 1
	}}
	threadRow := click{"a thread reply row", func(t *testing.T, a *App) (int, int) {
		return a.layout.msgEnd + 5, screenRowOf(t, a, "reply-2", a.layout.msgEnd)
	}}
	breadcrumb := click{"the thread breadcrumb's channel label", func(t *testing.T, a *App) (int, int) {
		return a.layout.msgEnd + 3, screenRowOf(t, a, "# general", a.layout.msgEnd)
	}}

	t.Run("typing in the thread", func(t *testing.T) {
		for _, c := range []click{channelCompose, channelRow, sidebarRow} {
			t.Run(c.name, func(t *testing.T) {
				a := typingClickApp(t)
				a.focusedPanel = PanelThread
				_ = a.beginInsertMode(true)
				updateAndRender(t, a, keyPress(' '))
				x, y := c.at(t, a)
				clickAndRelease(t, a, x, y)
				if a.mode != ModeInsert || a.focusedPanel != PanelThread {
					t.Fatalf("after the click: mode %v, focus %v; want insert mode in the thread", a.mode, a.focusedPanel)
				}
				got := typeThenSend(t, a)
				want := sendCall{reply: true, channelID: "C1", threadTS: "0.5", text: "thread draft !"}
				if got != want {
					t.Errorf("Enter sent %+v; want %+v", got, want)
				}
				if v := a.compose.Value(); v != "channel draft" {
					t.Errorf("channel composer holds %q; want its draft untouched", v)
				}
			})
		}
	})

	t.Run("typing in the channel", func(t *testing.T) {
		for _, c := range []click{threadCompose, threadRow, breadcrumb} {
			t.Run(c.name, func(t *testing.T) {
				a := typingClickApp(t)
				_ = a.beginInsertMode(false)
				updateAndRender(t, a, keyPress(' '))
				x, y := c.at(t, a)
				clickAndRelease(t, a, x, y)
				if a.mode != ModeInsert || a.focusedPanel != PanelMessages || !a.threadVisible {
					t.Fatalf("after the click: mode %v, focus %v, thread open %v; want insert mode in the channel, the thread open",
						a.mode, a.focusedPanel, a.threadVisible)
				}
				got := typeThenSend(t, a)
				want := sendCall{channelID: "C1", text: "channel draft !"}
				if got != want {
					t.Errorf("Enter sent %+v; want %+v", got, want)
				}
				if v := a.threadCompose.Value(); v != "thread draft" {
					t.Errorf("thread composer holds %q; want its draft untouched", v)
				}
			})
		}
	})
}

// An edit holds the keyboard like any typing: a click on the other
// pane leaves the edit where it is, and Enter saves it with the
// character typed after the click.
func TestClickWhileTyping_EditKeepsTheKeyboard(t *testing.T) {
	a := typingClickApp(t)
	_ = a.enterEdit("C1", "0.6", PanelThread, "reply-1")
	_ = a.View()
	clickAndRelease(t, a, a.layout.sidebarEnd+5, 1+a.layout.msgComposeTop+1)
	got := typeThenSend(t, a)
	want := sendCall{edit: true, channelID: "C1", threadTS: "0.6", text: "reply-1!"}
	if got != want {
		t.Errorf("Enter sent %+v; want %+v", got, want)
	}
	if v := a.compose.Value(); v != "channel draft" {
		t.Errorf("channel composer holds %q; want its draft untouched", v)
	}
}

// A plain click on a message row in the channel pane opens its thread,
// as Enter does, only outside insert mode: opening a thread moves focus
// to the thread pane, away from the channel composer being typed in.
// The click still selects the row.
func TestClickWhileTyping_ChannelRowSelectsWithoutOpeningItsThread(t *testing.T) {
	a := newTestApp(t, withSize(160, 30), withActiveChannel("C1"), withMessages(testMessageItems(3)...), withRender())
	_ = a.beginInsertMode(false)
	_ = a.View()
	x, y := a.layout.sidebarEnd+5, screenRowOf(t, a, "msg-1", a.layout.sidebarEnd)
	clickAndRelease(t, a, x, y)
	if a.threadVisible || a.focusedPanel != PanelMessages || a.mode != ModeInsert {
		t.Fatalf("thread open %v, focus %v, mode %v; want the thread closed and insert mode in the channel",
			a.threadVisible, a.focusedPanel, a.mode)
	}
	if sel, ok := a.messagepane.SelectedMessage(); !ok || sel.TS != "1.0" {
		t.Errorf("selected %q; want the clicked row 1.0", sel.TS)
	}
}
