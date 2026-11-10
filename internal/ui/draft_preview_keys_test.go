package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gammons/slk/internal/ui/compose"
	"github.com/gammons/slk/internal/ui/help"
	"github.com/gammons/slk/internal/ui/mentionpicker"
)

// The draft preview's keys go through a.Update, so every gate ahead of
// the insert-mode handler runs.

var ctrlR = keyMod('r', tea.ModCtrl)

// previewKeysApp is an App in normal mode on C1 in T1, the channel box
// bound to C1; i opens the box.
func previewKeysApp(t *testing.T) *App {
	t.Helper()
	a := newTestApp(t, append(insertOpts(), withActiveTeam("T1"))...)
	a.focusedPanel = PanelMessages
	a.compose.SetDraftContext("T1", "C1", "")
	return a
}

// send delivers one key through the real Update chain.
func send(a *App, k tea.Msg) { _, _ = a.Update(k) }

// openWithPreview presses i (the thread box when thread is set, after
// focusThreadPanel), types draft and presses ctrl+r, failing unless
// that showed c's preview.
func openWithPreview(t *testing.T, a *App, c *compose.Model, draft string) {
	t.Helper()
	send(a, keyPress('i'))
	if a.mode != ModeInsert {
		t.Fatalf("precondition: i left mode %v", a.mode)
	}
	for _, r := range draft {
		send(a, keyPress(r))
	}
	send(a, ctrlR)
	if !c.PreviewOn() {
		t.Fatal("ctrl+r did not show the box's draft preview")
	}
}

func TestCtrlRTogglesOnlyTheBoxBeingTypedIn(t *testing.T) {
	for _, thread := range []bool{false, true} {
		a := previewKeysApp(t)
		c, other := &a.compose, &a.threadCompose
		if thread {
			focusThreadPanel(t, a)
			c, other = &a.threadCompose, &a.compose
		}
		openWithPreview(t, a, c, "hi")
		if other.PreviewOn() {
			t.Fatalf("thread=%v: ctrl+r also showed the other box's preview", thread)
		}
		send(a, ctrlR)
		if c.PreviewOn() {
			t.Fatalf("thread=%v: a second ctrl+r left the preview on", thread)
		}
		if got := c.Value(); got != "hi" {
			t.Fatalf("thread=%v: ctrl+r changed the draft to %q", thread, got)
		}
	}
}

func TestCtrlRInNormalModeDoesNothing(t *testing.T) {
	a := previewKeysApp(t)
	send(a, ctrlR)
	if a.compose.PreviewOn() || a.threadCompose.PreviewOn() || a.mode != ModeNormal {
		t.Fatalf("normal-mode ctrl+r: previews %v/%v, mode %v", a.compose.PreviewOn(), a.threadCompose.PreviewOn(), a.mode)
	}
}

func TestSendClosesThePreview(t *testing.T) {
	for _, thread := range []bool{false, true} {
		a := previewKeysApp(t)
		c := &a.compose
		if thread {
			focusThreadPanel(t, a)
			c = &a.threadCompose
		}
		openWithPreview(t, a, c, "hello")
		_ = pressEnter(a)
		if c.Value() != "" {
			t.Fatalf("thread=%v: precondition: Enter did not send", thread)
		}
		if c.PreviewOn() {
			t.Fatalf("thread=%v: sending left the draft preview on", thread)
		}
	}
}

// Esc out of the box closes its preview, and i opens the box without
// it. Esc that only closes a picker leaves it.
func TestLeavingTheBoxClosesThePreview(t *testing.T) {
	for _, thread := range []bool{false, true} {
		a := previewKeysApp(t)
		c := &a.compose
		if thread {
			focusThreadPanel(t, a)
			c = &a.threadCompose
		}
		c.SetUsers([]mentionpicker.User{{ID: "U2", DisplayName: "bob"}})
		openWithPreview(t, a, c, "hi ")
		send(a, keyPress('@'))
		if !c.IsMentionActive() {
			t.Fatalf("thread=%v: precondition: @ did not open the mention picker", thread)
		}
		send(a, keyCode(tea.KeyEscape))
		if a.mode != ModeInsert || c.IsMentionActive() {
			t.Fatalf("thread=%v: precondition: Esc should only close the picker", thread)
		}
		if !c.PreviewOn() {
			t.Fatalf("thread=%v: Esc closing a picker closed the preview", thread)
		}
		send(a, keyCode(tea.KeyEscape))
		if a.mode != ModeNormal {
			t.Fatalf("thread=%v: precondition: Esc left mode %v", thread, a.mode)
		}
		if c.PreviewOn() {
			t.Fatalf("thread=%v: Esc out of the box left the preview on", thread)
		}
		send(a, keyPress('i'))
		if a.mode != ModeInsert || c.PreviewOn() {
			t.Fatalf("thread=%v: i reopened the box with the preview on (mode %v)", thread, a.mode)
		}
	}
}

// ctrl+u clears the box but is not a send: the preview stays.
func TestClearingTheBoxKeepsThePreview(t *testing.T) {
	a := previewKeysApp(t)
	openWithPreview(t, a, &a.compose, "x")
	send(a, keyMod('u', tea.ModCtrl))
	if a.compose.Value() != "" {
		t.Fatal("precondition: ctrl+u did not clear the box")
	}
	if !a.compose.PreviewOn() {
		t.Fatal("ctrl+u closed the draft preview")
	}
}

// The preview works while editing a message, and saving or cancelling
// the edit closes it.
func TestEndingAnEditClosesThePreview(t *testing.T) {
	for _, save := range []bool{false, true} {
		a := previewKeysApp(t)
		ts := a.messagepane.Messages()[0].TS
		_ = a.enterEdit("C1", ts, PanelMessages, "fixed")
		send(a, ctrlR)
		if !a.compose.PreviewOn() {
			t.Fatalf("save=%v: ctrl+r did not show the preview during an edit", save)
		}
		if save {
			_ = pressEnter(a)
		} else {
			send(a, keyCode(tea.KeyEscape))
		}
		if a.editing.IsActive() {
			t.Fatalf("save=%v: precondition: the edit is still open", save)
		}
		if a.compose.PreviewOn() {
			t.Fatalf("save=%v: ending the edit left the preview on", save)
		}
	}
}

func TestRunningASlashCommandClosesThePreview(t *testing.T) {
	a, _ := slashApp(t, nil)
	withSlashBuiltin(t, "/noop", func(*App, commandArgs) tea.Cmd { return nil })
	a.compose.SetValue("/noop")
	send(a, ctrlR)
	if !a.compose.PreviewOn() {
		t.Fatal("precondition: ctrl+r did not show the preview")
	}
	_ = pressEnter(a)
	if a.compose.PreviewOn() {
		t.Fatal("running a slash command left the preview on")
	}
}

func TestHelpListsTheDraftPreview(t *testing.T) {
	for _, e := range help.FromKeyMap(DefaultKeyMap()) {
		if e.Key == "ctrl+r" {
			if e.Desc != "show / hide the draft preview" || e.Group != "Composer" {
				t.Fatalf("help entry %+v, want the draft preview under Composer", e)
			}
			return
		}
	}
	t.Fatal("the help overlay does not list ctrl+r")
}
