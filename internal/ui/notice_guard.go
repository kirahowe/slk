package ui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/gammons/slk/internal/core"
	"github.com/gammons/slk/internal/ui/messages"
)

// noticeGuard refuses an action that makes no sense on a system notice
// (core.IsSystemNoticeSubtype): replying in its thread, editing, deleting
// or forwarding it. A notice of the user's own join carries their user
// ID, so the ownership checks alone would let an edit or delete through
// to Slack. what names the action in the toast ("reply to", "edit", ...).
func noticeGuard(msg messages.MessageItem, what string) (tea.Cmd, bool) {
	if !core.IsSystemNoticeSubtype(msg.Subtype) {
		return nil, false
	}
	return toastCmd("Can't " + what + " a system message"), true
}

// noticeSelectionGuard is noticeGuard on the focused pane's selection.
func (a *App) noticeSelectionGuard(what string) (tea.Cmd, bool) {
	msg, _, ok := a.selectedMessageInFocus()
	if !ok {
		return nil, false
	}
	return noticeGuard(msg, what)
}
