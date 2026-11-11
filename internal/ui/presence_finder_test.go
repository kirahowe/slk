package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/gammons/slk/internal/core"
	"github.com/gammons/slk/internal/ui/sidebar"
)

// presenceFinderApp is an App in T1 whose sidebar and channel finder
// both hold DMs D1 (Alice, U1) and D2 (Bob, U2), both away, and channel
// C1.
func presenceFinderApp(t *testing.T) *App {
	t.Helper()
	a := newTestApp(t, withActiveTeam("T1"), withChannels(
		sidebar.ChannelItem{ID: "D1", Name: "Alice", Type: "dm", DMUserID: "U1", Presence: "away"},
		sidebar.ChannelItem{ID: "D2", Name: "Bob", Type: "dm", DMUserID: "U2", Presence: "away"},
		sidebar.ChannelItem{ID: "C1", Name: "general", Type: "channel"},
	))
	a.SetChannelFinderItems([]core.ChannelFinderItem{
		{ID: "D1", Name: "Alice", Type: "dm", Presence: "away", Joined: true, LastVisited: 3},
		{ID: "D2", Name: "Bob", Type: "dm", Presence: "away", Joined: true, LastVisited: 2},
		{ID: "C1", Name: "general", Type: "channel", Joined: true, LastVisited: 1},
	})
	return a
}

// finderRow is the plain text of the open finder's row naming name.
func finderRow(a *App, name string) string {
	for _, line := range strings.Split(ansi.Strip(a.channelFinder.View(80)), "\n") {
		if strings.Contains(line, name) {
			return line
		}
	}
	return ""
}

// sidebarPresence is the Presence of the sidebar row with the given ID.
func sidebarPresence(a *App, id string) string {
	for _, it := range a.sidebar.Items() {
		if it.ID == id {
			return it.Presence
		}
	}
	return ""
}

// A presence_change gives the finder's DM row the dot the sidebar's row
// gets, and leaves every other row alone.
func TestPresenceChangeUpdatesFinderRows(t *testing.T) {
	a := presenceFinderApp(t)
	a.channelFinder.Open()

	a.Update(PresenceChangeMsg{UserID: "U1", Presence: "active"})

	if got := sidebarPresence(a, "D1"); got != "active" {
		t.Errorf("sidebar D1 presence = %q; want active", got)
	}
	if row := finderRow(a, "Alice"); !strings.Contains(row, "● Alice") {
		t.Errorf("finder Alice row = %q; want the active dot", row)
	}
	if row := finderRow(a, "Bob"); !strings.Contains(row, "○ Bob") {
		t.Errorf("finder Bob row = %q; want the away dot", row)
	}

	a.Update(PresenceChangeMsg{UserID: "U1", Presence: "away"})
	if row := finderRow(a, "Alice"); !strings.Contains(row, "○ Alice") {
		t.Errorf("finder Alice row after going away = %q; want the away dot", row)
	}
}

// PresenceChangeMsg names no workspace, and every connected workspace's
// socket sends one (rtmEventHandler.OnPresenceChange, cmd/slk). So a
// change reported by a workspace that is not on screen changes the rows
// on screen whenever the user ID matches one of their DM peers. Slack
// user IDs are unique across workspaces, so a match is the same person
// (an Enterprise Grid colleague or a Slack Connect peer seen from both).
func TestPresenceChangeAppliesWhicheverWorkspaceSentIt(t *testing.T) {
	a := presenceFinderApp(t)
	a.channelFinder.Open()

	// What T2's handler sends for U1: nothing in it says T2.
	a.Update(PresenceChangeMsg{UserID: "U1", Presence: "active"})

	if got := sidebarPresence(a, "D1"); got != "active" {
		t.Errorf("T1 sidebar D1 presence = %q; a change from any workspace applies", got)
	}
	if row := finderRow(a, "Alice"); !strings.Contains(row, "● Alice") {
		t.Errorf("T1 finder Alice row = %q; a change from any workspace applies", row)
	}
}
