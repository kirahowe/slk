// internal/ui/canvas.go
//
// The canvas viewer: c and :canvas open the focused conversation's
// canvases (a channel canvas, a DM's 1:1 notes) in a read-only overlay
// (internal/ui/canvasview). The model hands out a request number for
// every fetch, and reduceCanvas applies only the answer to the latest
// one.
package ui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
)

// canvasFetchTimeout bounds each canvas request: files.info and the
// download, or conversations.info.
const canvasFetchTimeout = 15 * time.Second

const noCanvasToast = "No canvas in this conversation"

// openCanvas is the c key.
func (a *App) openCanvas() tea.Cmd {
	return a.openConversationCanvas()
}

// cmdCanvas is :canvas.
func cmdCanvas(a *App, _ commandArgs) tea.Cmd {
	return a.openConversationCanvas()
}

// canvasConversation is the conversation c acts on: the focused pane's,
// else the channel on screen.
func (a *App) canvasConversation() (string, bool) {
	if _, ch, ok := a.focusedContentPane(); ok && ch != "" {
		return ch, true
	}
	if a.view == ViewChannels && a.activeChannelID != "" {
		return a.activeChannelID, true
	}
	return "", false
}

// openConversationCanvas opens the viewer on the focused conversation
// and lists its canvases.
func (a *App) openConversationCanvas() tea.Cmd {
	channelID, ok := a.canvasConversation()
	if !ok {
		return toastWithClear(a, noCanvasToast, 2*time.Second)
	}
	teamID := a.activeTeamID
	req := a.canvasView.OpenConversation(teamID)
	a.SetMode(ModeCanvas)
	svc := a.canvasSvc
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), canvasFetchTimeout)
		defer cancel()
		ids, err := svc.ConversationCanvases(ctx, teamID, channelID)
		return CanvasListLoadedMsg{Req: req, IDs: ids, Err: err}
	}
}

// openCanvasFile opens the viewer on one canvas.
func (a *App) openCanvasFile(teamID, fileID string) tea.Cmd {
	req := a.canvasView.OpenFile(teamID, fileID)
	a.SetMode(ModeCanvas)
	return a.fetchCanvas(teamID, fileID, req)
}

func (a *App) fetchCanvas(teamID, fileID string, req int) tea.Cmd {
	svc := a.canvasSvc
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), canvasFetchTimeout)
		defer cancel()
		c, err := svc.Canvas(ctx, teamID, fileID)
		return CanvasLoadedMsg{Req: req, Canvas: c, Err: err}
	}
}

// closeCanvas hides the viewer and returns to normal mode.
func (a *App) closeCanvas() {
	a.canvasView.Close()
	if a.mode == ModeCanvas {
		a.SetMode(ModeNormal)
	}
}

// canvasNames resolves a mention in the canvas to a display name.
func (a *App) canvasNames(userID string) (string, bool) {
	name, ok := a.userNames[userID]
	return name, ok && name != ""
}

var reduceCanvas reducerFunc = func(a *App, msg tea.Msg) (tea.Cmd, bool) {
	switch m := msg.(type) {
	case CanvasListLoadedMsg:
		if a.canvasView.Empty(m.Req, m.IDs, m.Err) {
			a.closeCanvas()
			return toastWithClear(a, noCanvasToast, 2*time.Second), true
		}
		if fileID, req, ok := a.canvasView.SetFiles(m.Req, m.IDs, m.Err); ok {
			return a.fetchCanvas(a.canvasView.TeamID(), fileID, req), true
		}
		return nil, true
	case CanvasLoadedMsg:
		a.canvasView.SetCanvas(m.Req, m.Canvas, m.Err)
		return nil, true
	}
	return nil, false
}

// handleCanvasMode is the viewer's key handler.
func handleCanvasMode(a *App, msg tea.KeyMsg) tea.Cmd {
	v := a.canvasView
	switch msg.String() {
	case "esc", "q", "c":
		a.closeCanvas()
	case "j", "down":
		v.Scroll(1)
	case "k", "up":
		v.Scroll(-1)
	case "ctrl+d":
		v.Scroll(v.HalfPage())
	case "ctrl+u":
		v.Scroll(-v.HalfPage())
	case "pgdown", "space", " ", "ctrl+f":
		v.Scroll(v.Page())
	case "pgup", "ctrl+b":
		v.Scroll(-v.Page())
	case "g", "home":
		v.Top()
	case "G", "end":
		v.Bottom()
	case "tab":
		if fileID, req, ok := v.Cycle(1); ok {
			return a.fetchCanvas(v.TeamID(), fileID, req)
		}
	case "shift+tab":
		if fileID, req, ok := v.Cycle(-1); ok {
			return a.fetchCanvas(v.TeamID(), fileID, req)
		}
	case "r":
		if fileID, req, ok := v.Reload(); ok {
			return a.fetchCanvas(v.TeamID(), fileID, req)
		}
	case "o":
		if link := v.Permalink(); link != "" {
			return a.openURLCmd(link)
		}
		return toastWithClear(a, "No link to open yet", 2*time.Second)
	}
	return nil
}
