package ui

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gammons/slk/internal/core"
	"github.com/gammons/slk/internal/core/blocks"
	"github.com/gammons/slk/internal/ids"
	"github.com/gammons/slk/internal/ui/messages"
	"github.com/gammons/slk/internal/ui/wintree"
)

func TestForwardMessageSelected(t *testing.T) {
	for _, pane := range []string{"channel", "thread"} {
		t.Run(pane, func(t *testing.T) {
			a := newTestApp(t, withActiveTeam("T1"), withActiveChannel("C1"),
				withMessages(messages.MessageItem{TS: "1.0", Text: "original", UserID: "OTHER"}))
			focusMessages(t, a)
			wantTS := ids.MessageTS("1.0")
			if pane == "thread" {
				focusThreadPanel(t, a)
				wantTS = "12.0"
				// The thread's channel must win over the active channel.
				a.activeChannelID = "COTHER"
			}
			a.compose.SetValue("unsent draft")
			a.threadCompose.SetValue("unsent reply")
			a.SetChannelFinderItems([]core.ChannelFinderItem{
				{ID: "C2", Name: "destination", Type: "channel", Joined: true},
				{ID: "D2", Name: "alice", Type: "dm", Joined: true},
				{ID: "C3", Name: "browse only", Type: "channel"},
			})
			calls := 0
			a.SetMessageService(core.NewMessageService(core.MessageServiceFuncs{
				Forward: func(ctx context.Context, team string, source ids.ChannelID, ts ids.MessageTS, dest ids.ChannelID) (core.ForwardResult, error) {
					calls++
					if team != "T1" || source != "C1" || ts != wantTS || dest != "C2" {
						t.Errorf("forward arguments = %q %q %q %q", team, source, ts, dest)
					}
					if _, ok := ctx.Deadline(); !ok {
						t.Error("forward request has no deadline")
					}
					return core.ForwardResult{TS: "20.0", Text: "https://example.slack.com/archives/C1/p1000000"}, nil
				},
			}))
			channel, panel, selection := a.activeChannelID, a.focusedPanel, a.messagepane.SelectedIndex()
			if cmd := dispatchModeKey(a, keyPress('F')); cmd != nil {
				t.Fatal("opening picker should not perform I/O")
			}
			if a.mode != ModeChannelFinder || !a.channelFinder.IsVisible() {
				t.Fatal("f did not open channel finder")
			}
			if !strings.Contains(a.channelFinder.View(120), "Forward message to") {
				t.Fatal("picker missing forwarding title")
			}
			if got := a.channelFinder.FilteredItems(); len(got) != 2 {
				t.Fatalf("forwarding destinations = %+v, want joined conversations only", got)
			}
			// Typing uses the local matcher, without searching for unjoined channels.
			if cmd := dispatchModeKey(a, keyPress('d')); cmd != nil {
				t.Fatal("forward picker scheduled a remote channel search")
			}
			cmd := dispatchModeKey(a, keyCode(tea.KeyEnter))
			if cmd == nil || calls != 0 {
				t.Fatal("forward must be deferred to a command")
			}
			if a.mode != ModeNormal || a.channelFinder.IsVisible() {
				t.Fatal("picker not closed after selection")
			}
			if a.activeChannelID != channel || a.focusedPanel != panel || a.messagepane.SelectedIndex() != selection ||
				a.compose.Value() != "unsent draft" || a.threadCompose.Value() != "unsent reply" {
				t.Fatal("forward changed navigation, selection, focus or drafts")
			}
			// An async workspace switch must not retarget the queued operation.
			a.activeTeamID = "T2"
			result := cmd()
			forwarded, ok := result.(messageForwardedMsg)
			if !ok || forwarded.err != nil || forwarded.channelID != "C2" || forwarded.message.TS != "20.0" || calls != 1 {
				t.Fatalf("forward result = %#v, calls = %d", result, calls)
			}
			if links := messages.ExtractLinks(forwarded.message.Text); len(links) != 1 || links[0].URL != "https://example.slack.com/archives/C1/p1000000" {
				t.Fatalf("forwarded permalink is not clickable: %q", forwarded.message.Text)
			}
			a.activeTeamID = "T1"
			a.Update(result)
			if got := statusbarText(a); !strings.Contains(got, "Message forwarded to #destination") {
				t.Fatalf("status bar = %q, want the forward confirmation", got)
			}
		})
	}
}

func TestForwardMessageCancelRestoresSwitcher(t *testing.T) {
	for _, cancel := range []string{"escape", "outside click", "mode change"} {
		t.Run(cancel, func(t *testing.T) {
			a := newTestApp(t, withActiveTeam("T1"), withActiveChannel("C1"), withMessages(testMessageItems(1)...))
			focusMessages(t, a)
			a.SetChannelFinderItems(channelFinderItems())
			dispatchModeKey(a, keyPress('F'))
			if a.mode != ModeChannelFinder {
				t.Fatal("forward picker did not open")
			}
			var cmd tea.Cmd
			switch cancel {
			case "escape":
				cmd = dispatchModeKey(a, keyCode(tea.KeyEscape))
			case "outside click":
				cmd = reduceModalClick(a, tea.MouseClickMsg{X: 0, Y: 0, Button: tea.MouseLeft})
			case "mode change":
				a.SetMode(ModeConfirm)
				a.SetMode(ModeNormal)
			}
			if cmd != nil || a.channelFinder.IsVisible() || a.mode != ModeNormal {
				t.Fatal("cancel did not close picker without sending")
			}
			dispatchModeKey(a, keyMod('t', tea.ModCtrl))
			if got := a.channelFinder.FilteredItems(); len(got) != 4 || !got[0].Synthetic {
				t.Fatalf("normal switcher destinations not restored: %+v", got)
			}
			cmd = dispatchModeKey(a, keyCode(tea.KeyEnter))
			if cmd == nil {
				t.Fatal("normal switcher did not select Threads")
			}
			if result := cmd(); result != (ThreadsViewActivatedMsg{}) {
				t.Fatalf("normal switcher dispatched %#v", result)
			}
		})
	}
}

func TestForwardMessageThreadsListDoesNotUseHiddenMessage(t *testing.T) {
	a := newTestApp(t, withActiveTeam("T1"), withActiveChannel("C1"), withMessages(testMessageItems(1)...))
	focusMessages(t, a)
	a.Update(ThreadsViewActivatedMsg{})
	if cmd := dispatchModeKey(a, keyPress('F')); cmd != nil || a.mode != ModeNormal || a.channelFinder.IsVisible() {
		t.Fatal("Threads list forwarded a hidden channel message")
	}
}

// forwardApp is an App on C1 in T1 (user SELF) showing one message of
// another user's, selected in a focused messages pane, with C1 a joined
// forwarding destination and C1 shown in a second window too. Forward
// answers with forward (or the error err); the returned log counts the
// calls.
func forwardApp(t *testing.T, result core.ForwardResult, err error) (*App, *int) {
	t.Helper()
	a := newTestApp(t, withSize(200, 50), withActiveTeam("T1"))
	a.SetCurrentUserID("SELF")
	a.Update(ChannelSelectedMsg{ID: "C1", Name: "general", Type: "channel"})
	if cmd := a.splitWindow(wintree.SplitSideBySide); cmd != nil {
		t.Fatal("split refused")
	}
	feedAll(a, NewMessageMsg{ChannelID: "C1", Message: messages.MessageItem{TS: "1.0", Text: "original", UserID: "OTHER"}})
	focusMessages(t, a)
	a.SetChannelFinderItems([]core.ChannelFinderItem{{ID: "C1", Name: "general", Type: "channel", Joined: true}})
	calls := 0
	a.SetMessageService(core.NewMessageService(core.MessageServiceFuncs{
		Forward: func(context.Context, string, ids.ChannelID, ids.MessageTS, ids.ChannelID) (core.ForwardResult, error) {
			calls++
			return result, err
		},
	}))
	if n := len(a.modelsForChannel("C1")); n != 2 {
		t.Fatalf("windows on C1 = %d, want 2", n)
	}
	return a, &calls
}

// startForward presses F and Enter on the selected message, forwarding
// it into C1, and returns the forward's result without applying it.
func startForward(t *testing.T, a *App) messageForwardedMsg {
	t.Helper()
	_, cmd := a.Update(keyPress('F'))
	if cmd != nil {
		drainBatch(cmd)
	}
	if a.mode != ModeChannelFinder {
		t.Fatal("F did not open the forward picker")
	}
	_, cmd = a.Update(keyCode(tea.KeyEnter))
	return deliveredMsgs[messageForwardedMsg](t, cmd)[0]
}

// feedAll runs msg through Update and every message its commands
// deliver, as the runtime would.
func feedAll(a *App, msg tea.Msg) {
	_, cmd := a.Update(msg)
	for _, m := range drainBatch(cmd) {
		if m != nil {
			feedAll(a, m)
		}
	}
}

// permutations is every order of n events, as index lists.
func permutations(n int) [][]int {
	if n == 0 {
		return [][]int{{}}
	}
	var out [][]int
	for _, p := range permutations(n - 1) {
		for i := 0; i <= len(p); i++ {
			q := append(append(append([]int{}, p[:i]...), n-1), p[i:]...)
			out = append(out, q)
		}
	}
	return out
}

// The forwarded message appears exactly once in every window showing
// the destination, as Slack's copy (the one carrying the shared
// attachment), whatever order the forward's HTTP result and its
// WebSocket echo arrive in. A send pending in the same conversation is
// not confused with it: its own result and echo, in any order among
// the forward's, leave it shown once too, and nothing stays pending.
func TestForwardShowsOnceInEveryOrder(t *testing.T) {
	const permalink = "https://example.slack.com/archives/C1/p1000000"
	forwardEcho := NewMessageMsg{ChannelID: "C1", Message: messages.MessageItem{
		TS: "20.0", UserID: "SELF", LegacyAttachments: []blocks.LegacyAttachment{{Text: "original"}},
	}}
	for _, withSend := range []bool{false, true} {
		n := 2
		if withSend {
			n = 4
		}
		for _, order := range permutations(n) {
			name := fmt.Sprintf("send=%v/order=%v", withSend, order)
			t.Run(name, func(t *testing.T) {
				a, _ := forwardApp(t, core.ForwardResult{TS: "20.0", Text: permalink}, nil)
				result := startForward(t, a)
				events := []tea.Msg{result, forwardEcho}
				want := []string{"1.0", "20.0"}
				if withSend {
					a.beginSend(sendRequest{ChannelID: "C1", Text: "concurrent", TeamID: "T1"})
					local := ""
					for _, r := range a.messagepane.Messages() {
						if messages.IsLocalTS(r.TS) && r.Text == "concurrent" {
							local = r.TS
						}
					}
					if local == "" {
						t.Fatal("no placeholder for the send")
					}
					sent := messages.MessageItem{TS: "21.0", Text: "concurrent", UserID: "SELF"}
					events = append(events,
						MessageSentMsg{ChannelID: "C1", LocalTS: local, Message: sent},
						NewMessageMsg{ChannelID: "C1", Message: sent})
					want = append(want, "21.0")
				}
				for _, i := range order {
					feedAll(a, events[i])
				}
				for w, mm := range a.modelsForChannel("C1") {
					rows := mm.Messages()
					var got []string
					for _, r := range rows {
						got = append(got, r.TS)
					}
					if !slices.Equal(got, want) {
						t.Fatalf("window %d rows = %v, want %v", w, got, want)
					}
					if fwd := rows[1]; len(fwd.LegacyAttachments) != 1 {
						t.Errorf("window %d shows the stand-in %q, want Slack's copy with the shared attachment", w, fwd.Text)
					}
				}
				if len(a.pendingOps.inserts) != 0 || len(a.pendingOps.held) != 0 {
					t.Errorf("left pending: inserts %v, held %v", a.pendingOps.inserts, a.pendingOps.held)
				}
			})
		}
	}
}

// Until the echo comes, the destination shows the stand-in: the
// permalink as a link, or plain words when the permalink could not be
// fetched after the share.
func TestForwardStandIn(t *testing.T) {
	for _, tc := range []struct{ permalink, want string }{
		{"https://example.slack.com/archives/C1/p1000000", "<https://example.slack.com/archives/C1/p1000000>"},
		{"", "Forwarded message"},
	} {
		a, _ := forwardApp(t, core.ForwardResult{TS: "20.0", Text: tc.permalink}, nil)
		result := startForward(t, a)
		if got := rowsOf(a.messagepane.Messages()); len(got) != 2 || !messages.IsLocalTS(a.messagepane.Messages()[1].TS) {
			t.Fatalf("rows while forwarding = %q, want the placeholder after the message", got)
		}
		feedAll(a, result)
		if got := rowsOf(a.messagepane.Messages()); !slices.Equal(got, []string{"1.0=original", "20.0=" + tc.want}) {
			t.Errorf("rows = %q, want the stand-in %q", got, tc.want)
		}
	}
}

// A failed forward takes its placeholder out of every window and says
// why. The user's messages held meanwhile show: Slack may have posted
// the share despite the failed call.
func TestForwardFailureLeavesNoRow(t *testing.T) {
	a, calls := forwardApp(t, core.ForwardResult{}, errors.New("restricted_action"))
	result := startForward(t, a)
	mine := messages.MessageItem{TS: "5.0", Text: "from my phone", UserID: "SELF"}
	feedAll(a, NewMessageMsg{ChannelID: "C1", Message: mine})
	// Not drained: tests fire the toast's clear tick at once.
	a.Update(result)
	if got := statusbarText(a); !strings.Contains(got, "Failed to forward message: restricted_action") {
		t.Errorf("status bar = %q, want the failure toast", got)
	}
	for w, mm := range a.modelsForChannel("C1") {
		if got := rowsOf(mm.Messages()); !slices.Equal(got, []string{"1.0=original", "5.0=from my phone"}) {
			t.Errorf("window %d rows = %q, want no forward row", w, got)
		}
	}
	if *calls != 1 || len(a.pendingOps.inserts) != 0 {
		t.Errorf("calls = %d, pending inserts = %v", *calls, a.pendingOps.inserts)
	}
}

func TestForwardMessageNoSelection(t *testing.T) {
	a := newTestApp(t, withActiveTeam("T1"), withActiveChannel("C1"))
	for _, panel := range []Panel{PanelMessages, PanelThread, PanelSidebar} {
		a.focusedPanel = panel
		if cmd := dispatchModeKey(a, keyPress('F')); cmd != nil || a.mode != ModeNormal || a.channelFinder.IsVisible() {
			t.Errorf("panel %v opened forwarding without a selected message", panel)
		}
	}
}

func TestForwardMessageClickAndFailure(t *testing.T) {
	a := newTestApp(t, withActiveTeam("T1"), withActiveChannel("C1"), withMessages(testMessageItems(1)...))
	focusMessages(t, a)
	a.SetChannelFinderItems([]core.ChannelFinderItem{{ID: "D2", Name: "alice", Type: "dm", Joined: true}})
	calls := 0
	a.SetMessageService(core.NewMessageService(core.MessageServiceFuncs{
		Forward: func(_ context.Context, _ string, _ ids.ChannelID, _ ids.MessageTS, dest ids.ChannelID) (core.ForwardResult, error) {
			calls++
			if dest != "D2" {
				t.Errorf("destination = %s, want D2", dest)
			}
			return core.ForwardResult{}, errors.New("restricted_action")
		},
	}))
	dispatchModeKey(a, keyPress('F'))
	if a.mode != ModeChannelFinder {
		t.Fatal("forward picker did not open")
	}
	w, h := a.channelFinder.BoxSize(a.width, a.height)
	cmd := reduceModalClick(a, tea.MouseClickMsg{X: (a.width-w)/2 + 3, Y: (a.height-h)/2 + 5, Button: tea.MouseLeft})
	if cmd == nil {
		t.Fatal("row click did not forward")
	}
	result, ok := cmd().(messageForwardedMsg)
	if !ok || result.err == nil || !strings.Contains(result.err.Error(), "restricted_action") || calls != 1 {
		t.Fatalf("failure = %#v, calls = %d", result, calls)
	}
}

func TestForwardMessageWorkspaceChangeBeforeSelection(t *testing.T) {
	a := newTestApp(t, withActiveTeam("T1"), withActiveChannel("C1"), withMessages(testMessageItems(1)...))
	focusMessages(t, a)
	a.SetChannelFinderItems(channelFinderItems())
	dispatchModeKey(a, keyPress('F'))
	if a.mode != ModeChannelFinder {
		t.Fatal("forward picker did not open")
	}
	a.activeTeamID = "T2"
	cmd := dispatchModeKey(a, keyCode(tea.KeyEnter))
	if a.mode != ModeNormal || a.channelFinder.IsVisible() || cmd == nil {
		t.Fatal("workspace mismatch should cancel forwarding with feedback")
	}
	if got := statusbarText(a); !strings.Contains(got, "workspace changed") {
		t.Fatalf("status bar = %q, want the cancellation toast", got)
	}
}

// Forwarding into the conversation on screen leaves the selection on
// the message the user was reading: the placeholder is added as any
// new message is, not selected as a send's is.
func TestForwardIntoOpenConversationKeepsSelection(t *testing.T) {
	a, _ := forwardApp(t, core.ForwardResult{TS: "20.0"}, nil)
	feedAll(a, NewMessageMsg{ChannelID: "C1", Message: messages.MessageItem{TS: "2.0", Text: "newer", UserID: "OTHER"}})
	a.messagepane.MoveUp()
	if msg, ok := a.messagepane.SelectedMessage(); !ok || msg.TS != "1.0" {
		t.Fatalf("selected %+v, want 1.0", msg)
	}
	feedAll(a, startForward(t, a))
	if msg, ok := a.messagepane.SelectedMessage(); !ok || msg.TS != "1.0" {
		t.Errorf("selected %+v after forwarding, want 1.0 still", msg)
	}
}
