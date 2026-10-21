package sidebar

import (
	"maps"
	"slices"
	"testing"

	"github.com/gammons/slk/internal/core"
	"pgregory.net/rapid"
)

// unreadsModel builds a Model over items reading state, with every
// section expanded so nav holds every filtered row, optionally in
// Slack-sections mode.
func unreadsModel(t *rapid.T, items []ChannelItem, state *map[string]core.ReadState) *Model {
	m := New(items)
	if rapid.Bool().Draw(t, "slackMode") {
		// Slack sections come from the provider; a config section name
		// would name none of them.
		for i := range items {
			items[i].Section = ""
		}
		m.SetSectionsProvider(&fakeProvider{ready: true, sections: []SectionMeta{
			{ID: "SD", Type: "direct_messages"}, {ID: "SA", Type: "recent_apps"}, {ID: "SC", Type: "channels"},
		}})
	}
	m.collapsed = map[string]bool{}
	m.SetReadStateReader(func() map[string]core.ReadState { return *state })
	return &m
}

// navChannelIDs is the channel rows of nav, top to bottom.
func navChannelIDs(m *Model) []string {
	var out []string
	for _, n := range m.nav {
		if n.kind == navChannel {
			out = append(out, m.items[m.filtered[n.fi]].ID)
		}
	}
	return out
}

// With nothing open, a row is in the Unreads section exactly when
// InUnreads holds for it; the section's rows come before every other
// row in nav, under the first header; every row appears once; and
// UnreadsCount is the section's size.
func TestUnreadsSectionHoldsExactlyTheRowsThatNeedTheUser(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		items := channelItems(14).Draw(t, "items")
		state := readStates(items).Draw(t, "state")
		m := unreadsModel(t, items, &state)

		want := map[string]bool{}
		for _, it := range items {
			if it.InUnreads(state[it.ID]) {
				want[it.ID] = true
			}
		}
		if !maps.Equal(m.unreads, want) {
			t.Fatalf("members %v, want %v", m.unreads, want)
		}
		if m.UnreadsCount() != len(want) {
			t.Fatalf("UnreadsCount = %d, want %d", m.UnreadsCount(), len(want))
		}

		ids := navChannelIDs(m)
		if len(ids) != len(items) {
			t.Fatalf("nav has %d rows for %d items", len(ids), len(items))
		}
		seen := map[string]bool{}
		left := false
		for _, id := range ids {
			if seen[id] {
				t.Fatalf("%s drawn twice", id)
			}
			seen[id] = true
			if !want[id] {
				left = true
			} else if left {
				t.Fatalf("Unreads row %s after a row outside it: %v", id, ids)
			}
		}
		firstHeader := ""
		for _, n := range m.nav {
			if n.kind == navHeader {
				firstHeader = n.header
				break
			}
		}
		if (firstHeader == unreadsSection) != (len(want) > 0) {
			t.Fatalf("first header %q with %d Unreads rows", firstHeader, len(want))
		}
	})
}

// inNav reports whether nav still has the stop key names.
func inNav(m *Model, key cursorKey) bool {
	for i := range m.nav {
		saved := m.cursor
		m.cursor = i
		k, _ := m.currentCursorKey()
		m.cursor = saved
		if k == key {
			return true
		}
	}
	return false
}

// The open conversation keeps its place in Unreads once read, and only
// while it stays open; a read conversation does not enter by being
// opened. A read-state change never moves the cursor off a row still in
// nav.
func TestUnreadsKeepsTheOpenConversationUntilLeft(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		items := channelItems(10).Draw(t, "items")
		if len(items) == 0 {
			return
		}
		state := readStates(items).Draw(t, "before")
		m := unreadsModel(t, items, &state)
		open := rapid.SampledFrom(items).Draw(t, "open").ID
		m.SetActiveChannelID(open)
		wasIn := m.unreads[open]

		if len(m.nav) > 2 {
			m.cursor = rapid.IntRange(0, len(m.nav)-1).Draw(t, "cursor")
		}
		key, _ := m.currentCursorKey()

		state = readStates(items).Draw(t, "after")
		m.Invalidate()
		item := items[slices.IndexFunc(items, func(it ChannelItem) bool { return it.ID == open })]
		if want := item.InUnreads(state[open]) || wasIn; m.unreads[open] != want {
			t.Fatalf("open %s in Unreads = %v, want %v (was %v)", open, m.unreads[open], want, wasIn)
		}
		if after, _ := m.currentCursorKey(); after != key && inNav(m, key) {
			t.Fatalf("cursor moved from %+v to %+v", key, after)
		}

		m.SetActiveChannelID("elsewhere")
		if want := item.InUnreads(state[open]); m.unreads[open] != want {
			t.Fatalf("left %s: in Unreads = %v, want %v", open, m.unreads[open], want)
		}
	})
}

// From a row outside Unreads, a goes to the section's first unread row
// and A to its last; from inside, it walks on in sidebar order.
func TestNextUnreadEntersUnreadsFirst(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		items := channelItems(14).Draw(t, "items")
		state := readStates(items).Draw(t, "state")
		m := unreadsModel(t, items, &state)
		var members, outside []string
		for _, id := range navChannelIDs(m) {
			if m.unreads[id] {
				members = append(members, id)
			} else {
				outside = append(outside, id)
			}
		}
		if len(members) == 0 {
			return
		}
		after := rapid.SampledFrom(append(outside, "nope")).Draw(t, "after")
		if id, _, _, ok := m.NextUnread(after, 1); !ok || id != members[0] {
			t.Fatalf("a from %s = %s %v, want %s", after, id, ok, members[0])
		}
		if id, _, _, ok := m.NextUnread(after, -1); !ok || id != members[len(members)-1] {
			t.Fatalf("A from %s = %s %v, want %s", after, id, ok, members[len(members)-1])
		}
		if len(members) > 1 {
			if id, _, _, _ := m.NextUnread(members[0], 1); id != members[1] {
				t.Fatalf("a from %s = %s, want %s", members[0], id, members[1])
			}
		}
	})
}
