package ui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/gammons/slk/internal/ids"
	"github.com/gammons/slk/internal/ui/channelfinder"
	"github.com/gammons/slk/internal/ui/messages"
)

type forwardSource struct {
	teamID    string
	channelID ids.ChannelID
	ts        ids.MessageTS
}

// messageForwardedMsg is a forward's result: the stand-in for the
// shared message (TS set) on success, err on failure. localTS is the
// forward's placeholder, as for a send.
type messageForwardedMsg struct {
	localTS     string
	channelID   string
	destination string
	message     messages.MessageItem
	err         error
}

// forwardToastDuration is how long a forward's result toast stays up.
const forwardToastDuration = 3 * time.Second

func (a *App) beginForwardOfSelected() tea.Cmd {
	channelID, ts, _, _, _, ok := a.selectedMessageContext()
	if !ok || channelID == "" || ts == "" || a.activeTeamID == "" {
		return nil
	}
	if cmd, blocked := a.sendingGuard(ts); blocked {
		return cmd
	}
	if cmd, blocked := a.noticeSelectionGuard("forward"); blocked {
		return cmd
	}
	a.pendingForward = &forwardSource{
		teamID: a.activeTeamID, channelID: ids.ChannelID(channelID), ts: ids.MessageTS(ts),
	}
	// Invalidate any search queued by a previous channel-switching session.
	// Forwarding only offers joined conversations, all available locally.
	a.pendingChannelSearchGen++
	a.channelFinder.OpenForForwarding()
	a.SetMode(ModeChannelFinder)
	return nil
}

// forwardBody is what the forward's rows show until Slack's copy of
// the shared message replaces them: the source permalink in Slack's
// link mrkdwn (so it is clickable), or plain words when there is none
// (the placeholder, and a result whose best-effort permalink failed).
func forwardBody(permalink string) messages.Body {
	if permalink == "" {
		return messages.Body{Text: "Forwarded message"}
	}
	return messages.Body{Text: "<" + permalink + ">"}
}

// forwardToChannel starts the forward of the pending source into
// destination, in the Update of the key that picked it. The forward is
// a pending insert in the destination's top level: its placeholder
// shows in every window on the destination, the user's own messages
// there are held until the result, and the result (reduceForwarded)
// settles or removes the placeholder.
func (a *App) forwardToChannel(destination channelfinder.ChannelResult) tea.Cmd {
	source := *a.pendingForward
	a.SetMode(ModeNormal)
	if source.teamID != a.activeTeamID {
		return toastWithClear(a, "Forward cancelled: workspace changed", forwardToastDuration)
	}
	localTS := a.pendingOps.NextLocalTS()
	ins := pendingInsert{
		localTS: localTS, channelID: destination.ID, body: forwardBody(""), postTeam: source.teamID,
	}
	ins.row, ins.begun = a.localRow(localTS, ins.body, "", ""), a.now()
	a.pendingOps.BeginInsert(ins)
	// Added as any new message is, not selected as a send's
	// placeholder is (InsertLocal): forwarding leaves the selection
	// where it was, unless the pane follows its newest row.
	for _, mm := range a.modelsForChannel(destination.ID) {
		mm.AppendMessage(cloneMessageItem(ins.row))
	}
	service := a.messageSvc
	standIn := ins.row
	name := destination.Name
	if destination.Type == "channel" || destination.Type == "private" {
		name = "#" + name
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		result, err := service.Forward(ctx, source.teamID, source.channelID, source.ts, ids.ChannelID(destination.ID))
		msg := messageForwardedMsg{localTS: localTS, channelID: destination.ID, destination: name, err: err}
		if err == nil {
			standIn.TS = string(result.TS)
			forwardBody(result.Text).Apply(&standIn)
			msg.message = standIn
		}
		return msg
	}
}

// reduceForwarded lands a forward's result. On success the placeholder
// settles with the share's own echo when it arrived first (held), else
// with the stand-in, which the echo replaces when it comes
// (reduceNewMessage, ForwardEcho). On failure the placeholder goes.
// Either way the user's messages held while it was pending are
// released, so a share Slack posted despite a failed call shows from
// its echo.
func reduceForwarded(a *App, m messageForwardedMsg) tea.Cmd {
	sentTS := ""
	if m.err == nil {
		sentTS = m.message.TS
	}
	ins, echo, echoed, release, ok := a.pendingOps.EndForward(m.localTS, sentTS)
	if !ok {
		ins = pendingInsert{channelID: m.channelID, postTeam: a.activeTeamID}
	}
	// Panes are written only while the workspace the forward posted to
	// is on screen: another one's window on a channel with the same ID
	// (Slack Connect) never showed the placeholder.
	onScreen := ins.postTeam == a.activeTeamID
	var toast tea.Cmd
	if m.err != nil {
		if onScreen {
			a.insertSinks(ins).removeLocal(m.localTS)
		}
		toast = toastWithClear(a, "Failed to forward message: "+m.err.Error(), forwardToastDuration)
	} else {
		item := m.message
		if echoed {
			item = echo.Message
		}
		if onScreen {
			a.insertSinks(ins).settle(m.localTS, item)
		}
		toast = toastWithClear(a, "Message forwarded to "+m.destination, forwardToastDuration)
	}
	return tea.Batch(toast, a.releaseHeld(release))
}
