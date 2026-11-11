package channelfinder

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/gammons/slk/internal/core"
)

// peerItems is n joined DM rows, peer-00 most recently visited, so the
// empty-query and "peer" orders are peer-00 … peer-(n-1).
func peerItems(n int) []core.ChannelFinderItem {
	items := make([]core.ChannelFinderItem, n)
	for i := range items {
		items[i] = core.ChannelFinderItem{
			ID: fmt.Sprintf("D%02d", i), Name: fmt.Sprintf("peer-%02d", i),
			Type: "dm", Presence: "away", Joined: true, LastVisited: int64(1000 - i),
		}
	}
	return items
}

// rowOf is the plain text of the drawn row naming name, or "".
func rowOf(view, name string) string {
	for _, line := range strings.Split(ansi.Strip(view), "\n") {
		if strings.Contains(line, name) {
			return line
		}
	}
	return ""
}

func TestSetPresenceChangesOnlyThatRowsDot(t *testing.T) {
	m := New()
	m.SetItems(peerItems(3))
	m.SetPresence("D01", "active")
	m.SetPresence("D99", "active") // no such row: nothing changes
	m.Open()

	view := m.View(80)
	if row := rowOf(view, "peer-01"); !strings.Contains(row, "● peer-01") {
		t.Errorf("peer-01 row = %q; want the active dot", row)
	}
	for _, name := range []string{"peer-00", "peer-02"} {
		if row := rowOf(view, name); !strings.Contains(row, "○ "+name) {
			t.Errorf("%s row = %q; want the away dot", name, row)
		}
	}

	m.SetPresence("D01", "away")
	if row := rowOf(m.View(80), "peer-01"); !strings.Contains(row, "○ peer-01") {
		t.Errorf("peer-01 row after going away = %q; want the away dot", row)
	}
}

// A presence change while the finder is open redraws the dot and
// leaves what the user is doing alone: the query, the selected row and
// the window of rows on screen, which follows the selection.
func TestSetPresenceKeepsOpenFinderState(t *testing.T) {
	m := New()
	m.SetItems(peerItems(3 * maxVisibleRows))
	m.Open()
	m.query = "peer"
	m.filter()
	m.selected = 2*maxVisibleRows + 3
	selectedID := m.items[m.filtered[m.selected]].ID
	filtered := slices.Clone(m.filtered)
	start, end := m.visibleWindow()
	if start == 0 {
		t.Fatalf("window starts at 0; the selection must have scrolled it")
	}

	m.SetPresence(selectedID, "active")

	if m.query != "peer" || !m.visible {
		t.Errorf("query = %q, visible = %v; want peer, true", m.query, m.visible)
	}
	if !slices.Equal(m.filtered, filtered) || m.items[m.filtered[m.selected]].ID != selectedID {
		t.Errorf("the result list or the selection moved")
	}
	if s, e := m.visibleWindow(); s != start || e != end {
		t.Errorf("window = [%d,%d); want [%d,%d)", s, e, start, end)
	}
	name := m.items[m.filtered[m.selected]].Name
	if row := rowOf(m.View(80), name); !strings.Contains(row, "● "+name) {
		t.Errorf("selected row = %q; want the active dot", row)
	}
}
