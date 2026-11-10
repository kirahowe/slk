package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gammons/slk/internal/ui/messages"
)

// selectedOwnApp has a normal message and then msg in C1 of T1, msg
// selected in a focused messages pane, the user being USELF.
func selectedOwnApp(t *testing.T, msg messages.MessageItem) *App {
	t.Helper()
	a, _ := actionApp(t, nil, messages.MessageItem{TS: "1.000000", UserID: "U1", UserName: "alice", Text: "hi"}, msg)
	a.currentUserID = "USELF"
	a.activeTeamID = "T1"
	a.focusedPanel = PanelMessages
	a.messagepane.SelectByIndex(1)
	return a
}

// ownJoinNotice is the user's own join notice. It carries the user's ID,
// as Slack's channel_join does for the user who joined, so ownership
// alone does not stop an edit or delete.
func ownJoinNotice() messages.MessageItem {
	return messages.MessageItem{TS: "2.000000", UserID: "USELF", UserName: "me", Text: "<@USELF> has joined the channel", Subtype: "channel_join"}
}

// Replying in a join notice's thread, editing, deleting or forwarding it
// makes no sense. The key toasts instead and sends nothing.
func TestMessageActionsRefuseOnSystemNotice(t *testing.T) {
	for _, tc := range []struct {
		key  tea.KeyMsg
		want string
	}{
		{keyCode(tea.KeyEnter), "Can't reply to a system message"},
		{keyPress('E'), "Can't edit a system message"},
		{keyPress('D'), "Can't delete a system message"},
		{keyPress('F'), "Can't forward a system message"},
	} {
		t.Run(tc.key.String(), func(t *testing.T) {
			a := selectedOwnApp(t, ownJoinNotice())
			if got := toastText(pressKey(t, a, tc.key)); got != tc.want {
				t.Errorf("toast = %q, want %q", got, tc.want)
			}
			if a.mode != ModeNormal || a.threadVisible || a.editing.IsActive() || a.confirmPrompt.IsVisible() || a.pendingForward != nil {
				t.Errorf("mode=%v threadVisible=%v editing=%v prompt=%v forward=%v: the key must do nothing else",
					a.mode, a.threadVisible, a.editing.IsActive(), a.confirmPrompt.IsVisible(), a.pendingForward != nil)
			}
		})
	}
}

// The guard is only for notices: on an ordinary message of the user's
// own the same keys act.
func TestMessageActionsStillWorkOnOwnMessage(t *testing.T) {
	a := selectedOwnApp(t, messages.MessageItem{TS: "2.000000", UserID: "USELF", UserName: "me", Text: "mine"})
	pressKey(t, a, keyPress('E'))
	if !a.editing.IsActive() {
		t.Error("E on the user's own message should start an edit")
	}
	a = selectedOwnApp(t, messages.MessageItem{TS: "2.000000", UserID: "USELF", UserName: "me", Text: "mine"})
	pressKey(t, a, keyPress('F'))
	if a.pendingForward == nil {
		t.Error("F on the user's own message should start a forward")
	}
}
