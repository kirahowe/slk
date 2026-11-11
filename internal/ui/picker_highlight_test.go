package ui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gammons/slk/internal/ui/messages"
)

// autocompleteHighlighted is the name on the channel composer's emoji
// autocomplete row that carries the ▌ marker.
func autocompleteHighlighted(t *testing.T, a *App) string {
	t.Helper()
	for _, line := range strings.Split(stripANSI(a.compose.EmojiPickerView(80)), "\n") {
		if !strings.Contains(line, "▌") {
			continue
		}
		fields := strings.Fields(strings.Trim(line, "│ "))
		return strings.Trim(fields[len(fields)-1], ":")
	}
	t.Fatal("no highlighted autocomplete row")
	return ""
}

// The emoji history landing while the autocomplete is open (a send
// accepted after the user started the next message) reorders the list
// but leaves the highlight on the emoji the user moved to, and Tab
// inserts that emoji.
func TestHistoryReloadKeepsAutocompleteHighlight(t *testing.T) {
	a, _ := emojiHistoryApp(t, withActiveChannel("C1"))
	_ = a.beginSend(sendRequest{ChannelID: "C1", Text: "nice :smirk:"})
	localTS := lastLocalTS(t, a)

	a.focusedPanel = PanelMessages
	a.SetMode(ModeInsert)
	_ = a.compose.Focus()
	for _, r := range ":sm" {
		a.Update(keyPress(r))
	}
	a.Update(keyCode(tea.KeyDown))
	a.Update(keyCode(tea.KeyDown))
	if got := autocompleteHighlighted(t, a); got != "small_orange_diamond" {
		t.Fatalf("precondition: highlighted %q, want small_orange_diamond", got)
	}

	updateSettled(t, a, MessageSentMsg{ChannelID: "C1", LocalTS: localTS,
		Message: messages.MessageItem{TS: "1700000000.000001", Text: "nice :smirk:"}})
	if rows := strings.Join(autocompleteRowsShown(a), " "); !strings.HasPrefix(rows, "smirk ") {
		t.Fatalf("precondition: rows = %q, want smirk first after the reload", rows)
	}
	if got := autocompleteHighlighted(t, a); got != "small_orange_diamond" {
		t.Errorf("highlighted %q after the reload, want small_orange_diamond", got)
	}
	a.Update(keyCode(tea.KeyTab))
	if got := a.compose.Value(); got != ":small_orange_diamond: " {
		t.Errorf("Tab inserted %q, want :small_orange_diamond:", got)
	}
}

// autocompleteRowsShown is the names the channel composer's open
// autocomplete lists, in order, without touching the composer.
func autocompleteRowsShown(a *App) []string {
	var rows []string
	for _, line := range strings.Split(stripANSI(a.compose.EmojiPickerView(80)), "\n") {
		fields := strings.Fields(strings.Trim(line, "│╭╮╰╯─▌ "))
		if len(fields) > 0 {
			rows = append(rows, strings.Trim(fields[len(fields)-1], ":"))
		}
	}
	return rows
}

// The emoji history landing while the reaction picker is open (an
// earlier reaction accepted) leaves the highlight on the emoji the
// user moved to, and Enter reacts with it.
func TestHistoryReloadKeepsReactionPickerHighlight(t *testing.T) {
	var calls reactionCalls
	a := newTestApp(t, reactionPickerOpts(false)...)
	openReactionPicker(&calls, "tada", "rocket")(t, a)
	for range 3 {
		a.Update(keyCode(tea.KeyDown))
	}
	want := highlightedEmoji(t, a)
	if want == "tada" {
		t.Fatalf("precondition: three downs left the highlight on the top row")
	}

	updateSettled(t, a, ReactionSentMsg{ChannelID: "C1", MessageTS: "0.5", Emoji: "eyes"})
	if len(calls.frecent) != 1 {
		t.Fatalf("precondition: recorded %v, want the earlier reaction's emoji", calls.frecent)
	}
	if got := highlightedEmoji(t, a); got != want {
		t.Errorf("highlighted %q after the reload, want %q", got, want)
	}
	if got := commitAfterDowns(t, a, &calls, 0); got != want {
		t.Errorf("Enter committed %q, want %q", got, want)
	}
}
