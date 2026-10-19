package ui

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gammons/slk/internal/core"
	"github.com/gammons/slk/internal/ids"
	"github.com/gammons/slk/internal/ui/messages"
)

// hiddenTS, hiddenURL and hiddenFiles identify the channel pane's
// selected message in listViewKeyApp. A message a key produces that names
// any of them acted on that message.
const (
	hiddenTS    = "1700000000.000100"
	hiddenURL   = "https://acme.slack.com/archives/C1/p1700000000000050"
	hiddenFiles = "https://files.example/"
)

// listViewKeyApp is an App on C1 whose channel pane has one message
// selected that every message key can act on: the user's own (E, D), a
// forwarded permalink alone (Enter, o), a reaction (L, R, r), an image
// (O/v) and a file (d). Its permalink is hiddenURL (Y/C). It returns the
// App and a record of what reached the clipboard and the desktop.
func listViewKeyApp(t *testing.T, view View) (*App, *sideEffects) {
	t.Helper()
	a := newTestApp(t,
		withActiveTeam("T1"),
		withActiveChannel("C1"),
		withMessages(messages.MessageItem{
			TS:     hiddenTS,
			UserID: "U1",
			Text:   "<" + hiddenURL + ">",
			Reactions: []messages.ReactionItem{
				{Emoji: "thumbsup", Count: 1, UserIDs: []string{"U2"}},
			},
			Attachments: []messages.Attachment{
				{Kind: "image", FileID: "F1", Name: "a.png", URL: hiddenFiles + "a.png"},
				{Kind: "file", Name: "b.pdf", DownloadURL: hiddenFiles + "b.pdf"},
			},
		}),
		withView(view),
	)
	a.SetCurrentUserID("U1")
	a.focusedPanel = PanelMessages
	a.setPermalinkFetcherForTest(func(context.Context, ids.ChannelID, ids.MessageTS) (string, error) {
		return hiddenURL, nil
	})
	fx := &sideEffects{}
	a.SetClipboardWriter(func(text string) tea.Cmd {
		fx.copied = append(fx.copied, text)
		return nil
	})
	a.setDesktopForTest(func(d *core.DesktopServiceFuncs) {
		d.Open = func(target string) error {
			fx.opened = append(fx.opened, target)
			return nil
		}
	})
	if _, ok := a.messagepane.SelectedMessage(); !ok {
		t.Fatal("precondition: the channel pane has no selected message")
	}
	return a, fx
}

// sideEffects records what listViewKeyApp's clipboard writer and desktop
// service were given.
type sideEffects struct {
	copied []string
	opened []string
}

// actedOnHidden reports how a key acted on listViewKeyApp's message, or ""
// when it did not.
func actedOnHidden(a *App, fx *sideEffects, msgs []tea.Msg) string {
	switch {
	case a.mode != ModeNormal:
		return "mode " + a.mode.String()
	case a.messagepane.ReactionNavActive():
		return "reaction nav"
	case a.pendingForward != nil:
		return "forward started"
	case a.editing.IsActive():
		return "edit started"
	case len(fx.copied) > 0:
		return fmt.Sprintf("copied %q", fx.copied)
	case len(fx.opened) > 0:
		return fmt.Sprintf("opened %q", fx.opened)
	}
	for _, m := range msgs {
		if _, ok := m.(MarkUnreadMsg); ok {
			return "marked unread"
		}
		s := fmt.Sprintf("%#v", m)
		for _, id := range []string{hiddenTS, hiddenURL, hiddenFiles} {
			if strings.Contains(s, id) {
				return fmt.Sprintf("produced %T", m)
			}
		}
	}
	return ""
}

// TestMessageKeysSkipTheHiddenChannelPane: in the Threads and Activity
// views the list takes the channel pane's place, and the channel pane
// keeps the last channel's messages out of sight. A message key pressed
// with the list focused must not act on the message selected there: L
// used to list its reactions, r react to it, E edit it, D delete it.
//
// The Channels view is the control: there the same key acts on the same
// message, so a key that does nothing below is not doing nothing because
// the fixture gives it nothing to do.
func TestMessageKeysSkipTheHiddenChannelPane(t *testing.T) {
	keys := []struct {
		name string
		key  tea.KeyMsg
	}{
		{"L lists reactions", keyPress('L')},
		{"r adds a reaction", keyPress('r')},
		{"R navigates reactions", keyPress('R')},
		{"E edits", keyPress('E')},
		{"D deletes", keyPress('D')},
		{"y copies the text", keyPress('y')},
		{"Y copies the permalink", keyPress('Y')},
		{"C copies the permalink", keyPress('C')},
		{"F forwards", keyPress('F')},
		{"O previews the image", keyPress('O')},
		{"v previews the image", keyPress('v')},
		{"o opens the link", keyPress('o')},
		{"d downloads the file", keyPress('d')},
		{"U marks unread", keyPress('U')},
		{"K shows the author", keyPress('K')},
	}
	for _, k := range keys {
		t.Run(k.name, func(t *testing.T) {
			t.Parallel()
			a, fx := listViewKeyApp(t, ViewChannels)
			msgs := pressKey(t, a, k.key)
			if actedOnHidden(a, fx, msgs) == "" {
				t.Fatalf("control: in the Channels view the key did nothing to the selected message (msgs %#v)", msgs)
			}
			for _, view := range []View{ViewThreads, ViewActivity} {
				a, fx := listViewKeyApp(t, view)
				msgs := pressKey(t, a, k.key)
				if how := actedOnHidden(a, fx, msgs); how != "" {
					t.Errorf("view %d: acted on the hidden channel pane's message: %s", view, how)
				}
			}
		})
	}
}

// TestEnterInActivitySkipsHiddenForward: Enter on a forwarded message
// opens the original. In the Activity view the channel pane's selection
// is out of sight, so Enter must not follow its link.
func TestEnterInActivitySkipsHiddenForward(t *testing.T) {
	a, fx := listViewKeyApp(t, ViewActivity)
	msgs := pressKey(t, a, keyCode(tea.KeyEnter))
	if how := actedOnHidden(a, fx, msgs); how != "" {
		t.Errorf("acted on the hidden channel pane's message: %s", how)
	}
}
