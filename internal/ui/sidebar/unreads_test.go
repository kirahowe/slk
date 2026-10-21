package sidebar

import (
	"slices"
	"strings"
	"testing"

	"github.com/gammons/slk/internal/core"
)

// unreadsFixture is a config-mode sidebar with a custom section, two
// channels, a muted channel, two DMs and a group DM, reading state.
func unreadsFixture(state *map[string]core.ReadState) *Model {
	m := New([]ChannelItem{
		{ID: "C1", Name: "deploys", Type: "channel", Section: "Eng"},
		{ID: "C2", Name: "general", Type: "channel"},
		{ID: "C3", Name: "firehose", Type: "channel", IsMuted: true},
		{ID: "D1", Name: "Maya Chen", Type: "dm"},
		{ID: "D2", Name: "Sam Okafor", Type: "dm"},
		{ID: "G1", Name: "Priya, Sam", Type: "group_dm"},
	})
	m.SetReadStateReader(func() map[string]core.ReadState { return *state })
	return &m
}

// sidebarLines is the rendered sidebar, ANSI-stripped and trimmed, one
// entry per row.
func sidebarLines(m *Model) []string {
	var out []string
	for _, l := range strings.Split(m.View(30, 32), "\n") {
		if l = strings.TrimSpace(ansiRe.ReplaceAllString(l, "")); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// section returns the rows under the header named title, up to the
// next blank line.
func section(m *Model, title string) []string {
	var lines []string
	for _, l := range strings.Split(m.View(30, 32), "\n") {
		lines = append(lines, strings.TrimSpace(ansiRe.ReplaceAllString(l, "")))
	}
	for i, l := range lines {
		if strings.HasSuffix(l, " "+title) || strings.Contains(l, " "+title+" ") {
			var rows []string
			for _, r := range lines[i+1:] {
				if r == "" {
					break
				}
				rows = append(rows, r)
			}
			return rows
		}
	}
	return nil
}

func TestUnreads_DMsAndMentionsMoveToTheTop(t *testing.T) {
	state := map[string]core.ReadState{
		"C1": {HasUnread: true},                  // unread, no mention: stays in Eng
		"C2": {HasUnread: true, MentionCount: 1}, // mentioned
		"C3": {HasUnread: true, MentionCount: 2}, // muted, mentioned
		"D2": {HasUnread: true},                  // unread DM, count not known
		"G1": {HasUnread: true, MentionCount: 3},
	}
	m := unreadsFixture(&state)

	lines := sidebarLines(m)
	if i := slices.IndexFunc(lines, func(l string) bool { return strings.HasSuffix(l, "Unreads") }); i != 2 {
		t.Fatalf("Unreads header at row %d, want right under Threads and Activity:\n%s", i, strings.Join(lines, "\n"))
	}
	// People first, then channels; by name within each.
	got := section(m, "Unreads")
	want := []string{"Priya, Sam  3", "Sam Okafor ●", "# firehose  2", "# general  1"}
	if len(got) != len(want) {
		t.Fatalf("Unreads rows %q, want %q", got, want)
	}
	for i := range want {
		if !strings.HasSuffix(got[i], want[i]) {
			t.Fatalf("Unreads row %d = %q, want %q (all: %q)", i, got[i], want[i], got)
		}
	}
	if eng := section(m, "Eng"); len(eng) != 1 || !strings.Contains(eng[0], "deploys") {
		t.Errorf("Eng = %q, want the unread, unmentioned deploys left in place", eng)
	}
	if dms := section(m, "Direct Messages"); len(dms) != 1 || !strings.Contains(dms[0], "Maya Chen") {
		t.Errorf("Direct Messages = %q, want only the read Maya Chen", dms)
	}
	if m.UnreadsCount() != 4 {
		t.Errorf("UnreadsCount = %d, want 4", m.UnreadsCount())
	}
}

func TestUnreads_RowGoesBackWhenRead(t *testing.T) {
	state := map[string]core.ReadState{"D1": {HasUnread: true, MentionCount: 1}}
	m := unreadsFixture(&state)
	if !m.unreads["D1"] {
		t.Fatal("precondition: the unread DM is in Unreads")
	}
	// Somewhere else (another device) reads it.
	state = map[string]core.ReadState{}
	m.Invalidate()
	if m.unreads["D1"] || section(m, "Unreads") != nil {
		t.Fatalf("read DM still in Unreads: %q", sidebarLines(m))
	}
	if dms := section(m, "Direct Messages"); len(dms) != 3 {
		t.Errorf("Direct Messages = %q, want Maya Chen back", dms)
	}
	if m.UnreadsCount() != 0 {
		t.Errorf("UnreadsCount = %d, want 0", m.UnreadsCount())
	}
}

// Opening a DM marks it read; the row must not jump away from the user
// until they go somewhere else.
func TestUnreads_OpenConversationStaysUntilLeft(t *testing.T) {
	state := map[string]core.ReadState{"D1": {HasUnread: true, MentionCount: 2}}
	m := unreadsFixture(&state)
	m.SelectByID("D1")
	m.SetActiveChannelID("D1")

	state = map[string]core.ReadState{}
	m.Invalidate()
	if rows := section(m, "Unreads"); len(rows) != 1 || !strings.Contains(rows[0], "Maya Chen") {
		t.Fatalf("open DM left Unreads once read: %q", rows)
	}
	if m.SelectedID() != "D1" {
		t.Errorf("cursor moved to %q", m.SelectedID())
	}
	if m.UnreadsCount() != 0 {
		t.Errorf("UnreadsCount = %d, want 0: the open one is read", m.UnreadsCount())
	}

	m.SetActiveChannelID("C2")
	if section(m, "Unreads") != nil {
		t.Errorf("DM still in Unreads after leaving it: %q", sidebarLines(m))
	}
	// And opening a read conversation never pulls it in.
	m.SetActiveChannelID("D2")
	if m.unreads["D2"] {
		t.Error("opening a read DM put it in Unreads")
	}
}

func TestUnreads_OffLeavesEveryRowInItsSection(t *testing.T) {
	state := map[string]core.ReadState{"D1": {HasUnread: true, MentionCount: 1}}
	m := unreadsFixture(&state)
	m.SetUnreadsSection(false)
	if section(m, "Unreads") != nil {
		t.Fatalf("Unreads drawn while off: %q", sidebarLines(m))
	}
	if dms := section(m, "Direct Messages"); len(dms) != 3 {
		t.Errorf("Direct Messages = %q, want all three", dms)
	}
	if m.UnreadsCount() != 1 {
		t.Errorf("UnreadsCount = %d, want 1: the status bar still counts", m.UnreadsCount())
	}
}

// A user's own section titled "Unreads" is a different section.
func TestUnreads_CustomSectionNamedUnreadsStaysSeparate(t *testing.T) {
	m := New([]ChannelItem{
		{ID: "C1", Name: "inbox", Type: "channel", Section: "Unreads"},
		{ID: "D1", Name: "Maya Chen", Type: "dm"},
	})
	m.SetReadStateReader(func() map[string]core.ReadState {
		return map[string]core.ReadState{"D1": {HasUnread: true}}
	})
	var headers []string
	for _, n := range m.nav {
		if n.kind == navHeader {
			headers = append(headers, n.header)
		}
	}
	if want := []string{unreadsSection, "Unreads"}; !slices.Equal(headers[:2], want) {
		t.Fatalf("headers %q, want %q first", headers, want)
	}
}
