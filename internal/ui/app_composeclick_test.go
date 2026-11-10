package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/gammons/slk/internal/ui/messages"
)

// composeClickApp is a 120x60 App on C123 with one message, rendered
// once so the layout holds the compose regions.
func composeClickApp(t *testing.T, h int) *App {
	t.Helper()
	return newTestApp(t, withSize(120, h), withActiveChannel("C123"), withRender(), withMessages(messages.MessageItem{
		TS: "1.0", UserName: "alice", Text: "hello",
	}))
}

// clickAt sends a left click at screen cell (x, y) through Update and
// renders the next frame.
func clickAt(t *testing.T, a *App, x, y int) {
	t.Helper()
	updateAndRender(t, a, tea.MouseClickMsg{X: x, Y: y, Button: tea.MouseLeft})
}

// TestMouseClick_OnComposeEntersInsert pins the click-to-compose
// feature: a click inside the channel pane's compose region enters
// insert mode with the channel compose focused, exactly as `i` does.
func TestMouseClick_OnComposeEntersInsert(t *testing.T) {
	app := composeClickApp(t, 60)
	top := app.layout.msgComposeTop
	if top <= 0 {
		t.Fatal("msgComposeTop not recorded after View(); the channel pane did not draw a compose region")
	}
	// Screen row = pane row + 1 for the top border; one past the
	// spacer line is inside the box.
	clickAt(t, app, app.layout.sidebarEnd+5, 1+top+1)

	if app.mode != ModeInsert {
		t.Errorf("mode = %v; want ModeInsert after clicking the compose region", app.mode)
	}
	if app.focusedPanel != PanelMessages {
		t.Errorf("focusedPanel = %v; want PanelMessages (the channel compose)", app.focusedPanel)
	}
}

// TestMouseClick_AboveComposeDoesNotInsert asserts the region boundary:
// a click on a message row above the compose region does not enter
// insert mode.
func TestMouseClick_AboveComposeDoesNotInsert(t *testing.T) {
	app := composeClickApp(t, 60)
	clickAt(t, app, app.layout.sidebarEnd+5, 1+app.messagepane.ChromeHeight()+1) // first message row

	if app.mode == ModeInsert {
		t.Error("mode = ModeInsert after clicking a message row above the compose region; want it unchanged")
	}
}

// screenRowOf is the first screen row showing marker at or right of
// column x.
func screenRowOf(t *testing.T, a *App, marker string, x int) int {
	t.Helper()
	for row, line := range strings.Split(ansi.Strip(a.View().Content), "\n") {
		if i := strings.Index(line, marker); i >= 0 && ansi.StringWidth(line[:i]) >= x {
			return row
		}
	}
	t.Fatalf("%q not on screen", marker)
	return -1
}

// TestMouseClick_ComposeRegionStartsBelowTheTypingRow pins the region's
// first row on screen: a click on the typing row does not start typing,
// and a click on the row below it (the spacer above the box) does.
func TestMouseClick_ComposeRegionStartsBelowTheTypingRow(t *testing.T) {
	a := botStatusApp(t)
	focusMessages(t, a)
	updateAndRender(t, a, UserTypingMsg{ChannelID: "C1", UserID: "U1", WorkspaceID: "T1"})
	x := a.layout.sidebarEnd + 5
	typing := screenRowOf(t, a, "alice is typing", a.layout.sidebarEnd)

	clickAt(t, a, x, typing)
	if a.mode == ModeInsert {
		t.Fatal("a click on the typing row entered insert mode")
	}
	clickAt(t, a, x, typing+1)
	if a.mode != ModeInsert || a.focusedPanel != PanelMessages {
		t.Errorf("mode = %v, focus = %v; want ModeInsert in the channel compose after clicking the first compose row", a.mode, a.focusedPanel)
	}
}

// TestMouseClick_ShortTerminalMessageRowsStayClickable pins the clamp
// order: on a terminal too short for the compose box, the renderer
// still draws the minimum three message rows, and clicking them must
// not start typing. The compose top is recorded from the clamped
// height, not the raw one.
func TestMouseClick_ShortTerminalMessageRowsStayClickable(t *testing.T) {
	// Height small enough that contentHeight - 2 - bottomHeight < 3.
	app := composeClickApp(t, 12)
	clickAt(t, app, app.layout.sidebarEnd+5, 1+app.messagepane.ChromeHeight()+1)

	if app.mode == ModeInsert {
		t.Error("mode = ModeInsert after clicking a message row on a short terminal; the compose top was recorded before the clamp")
	}
}

// TestMouseClick_ComposeOnlyFromNormalMode: `i` works only in normal
// mode, and so does its click. A click on the compose while a `:` line
// is half typed leaves command mode alone.
func TestMouseClick_ComposeOnlyFromNormalMode(t *testing.T) {
	app := composeClickApp(t, 60)
	updateAndRender(t, app, keyPress(':'))
	if app.mode != ModeCommand {
		t.Fatalf("precondition: mode = %v, want ModeCommand", app.mode)
	}
	clickAt(t, app, app.layout.sidebarEnd+5, 1+app.layout.msgComposeTop+1)
	if app.mode != ModeCommand {
		t.Errorf("mode = %v after a compose click in command mode, want ModeCommand", app.mode)
	}
}

// TestMouseClick_ThreadComposeFocusesTheThread: the thread pane's
// compose region starts a thread reply, and an app's status row above
// it is not part of the region (the thread pane's twin of the channel
// pane's typing row).
func TestMouseClick_ThreadComposeFocusesTheThread(t *testing.T) {
	a := botStatusApp(t)
	ts := openThreadOnSelected(t, a)
	focusMessages(t, a)
	updateAndRender(t, a, atlasStatus("C1", ts, "Thinking..."))
	x := a.layout.msgEnd + 5
	status := screenRowOf(t, a, "Atlas Thinking...", a.layout.msgEnd)

	clickAt(t, a, x, status)
	if a.mode == ModeInsert {
		t.Fatal("a click on the thread's app status row entered insert mode")
	}
	focusMessages(t, a)
	clickAt(t, a, x, status+1)
	if a.mode != ModeInsert || a.focusedPanel != PanelThread {
		t.Errorf("mode = %v, focus = %v; want ModeInsert in the thread compose", a.mode, a.focusedPanel)
	}
}

// TestMouseClick_StackedThreadComposeFocusesTheThread: in the stacked
// layout the thread is drawn alone, and a click on its compose starts a
// thread reply, as `i` does there.
func TestMouseClick_StackedThreadComposeFocusesTheThread(t *testing.T) {
	a := stackedApp(t, 110)
	updateAndRender(t, a, keyCode(tea.KeyEnter))
	assertFront(t, a, false, true)
	clickAt(t, a, a.layout.msgEnd+5, 1+a.layout.threadComposeTop+1)
	if a.mode != ModeInsert || a.focusedPanel != PanelThread {
		t.Errorf("mode = %v, focus = %v; want ModeInsert in the thread compose", a.mode, a.focusedPanel)
	}
	assertFront(t, a, false, true)
}

// A click on a compose region in insert mode is not a click on a
// message. Scrolled away from the bottom, the list goes on past the
// window, and the hit tests read the row under the click as one of the
// messages below it: they selected it, and the release opened its
// thread.
func TestMouseClick_OnComposeInInsertLeavesTheList(t *testing.T) {
	t.Run("channel", func(t *testing.T) {
		a := newTestApp(t, withSize(120, 30), withActiveChannel("C1"), withMessages(testMessageItems(60)...), withRender())
		a.messagepane.GoToTop()
		_ = a.beginInsertMode(false)
		_ = a.View()
		before, _ := a.messagepane.SelectedMessage()
		x, y := a.layout.sidebarEnd+5, 1+a.layout.msgComposeTop+2
		clickAt(t, a, x, y)
		updateAndRender(t, a, tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft})
		if after, _ := a.messagepane.SelectedMessage(); after.TS != before.TS || a.threadVisible {
			t.Fatalf("selection %s -> %s, thread open %v; want the click to change neither", before.TS, after.TS, a.threadVisible)
		}
		if a.mode != ModeInsert {
			t.Fatalf("mode = %v, want insert mode kept", a.mode)
		}
	})
	t.Run("thread", func(t *testing.T) {
		a := newTestApp(t, withSize(120, 30), withActiveChannel("C1"), withMessages(testMessageItems(3)...))
		parent := messages.MessageItem{TS: "0.5", UserName: "alice", Text: "parent", ThreadTS: "0.5"}
		a.threadPanel.SetThread(parent, testMessageItems(40), "C1", "0.5")
		a.threadVisible = true
		a.focusedPanel = PanelThread
		a.threadPanel.GoToTop()
		_ = a.beginInsertMode(true)
		_ = a.View()
		before := a.threadPanel.SelectedReply()
		x, y := a.layout.ThreadEnd()-5, 1+a.layout.threadComposeTop+2
		clickAt(t, a, x, y)
		updateAndRender(t, a, tea.MouseReleaseMsg{X: x, Y: y, Button: tea.MouseLeft})
		if after := a.threadPanel.SelectedReply(); before == nil || after == nil || after.TS != before.TS {
			t.Fatalf("thread selection moved from %v to %v", before, after)
		}
	})
}
