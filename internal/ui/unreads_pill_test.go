package ui

import (
	"strings"
	"testing"

	"github.com/gammons/slk/internal/core"
	"github.com/gammons/slk/internal/ui/sidebar"
)

// The status bar's Unreads pill follows the sidebar's Unreads section
// through every read-state change, so a waiting DM shows whatever the
// sidebar is scrolled to.
func TestUnreadsPillFollowsReadState(t *testing.T) {
	a := newTestApp(t, withChannels(
		sidebar.ChannelItem{ID: "C1", Name: "general", Type: "channel"},
		sidebar.ChannelItem{ID: "D1", Name: "Maya Chen", Type: "dm"},
	))
	state := map[string]core.ReadState{"D1": {HasUnread: true, MentionCount: 1}}
	a.setReadStateReaderForTest(func() map[string]core.ReadState { return state })
	if got := statusbarText(a); !strings.Contains(got, "Unreads 1") {
		t.Fatalf("status bar %q, want the Unreads 1 pill once wired", got)
	}

	state = map[string]core.ReadState{
		"C1": {HasUnread: true}, // unread channel, no mention: not counted
		"D1": {HasUnread: true, MentionCount: 2},
	}
	a.Update(ReadStateChangedMsg{})
	if got := statusbarText(a); !strings.Contains(got, "Unreads 1") {
		t.Fatalf("status bar %q, want Unreads 1", got)
	}

	state = map[string]core.ReadState{}
	a.Update(ReadStateChangedMsg{})
	if got := statusbarText(a); strings.Contains(got, "Unreads") {
		t.Fatalf("status bar %q still shows the pill with nothing waiting", got)
	}
}
