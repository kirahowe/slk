package statusbar

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"pgregory.net/rapid"

	"github.com/gammons/slk/internal/proptest"
)

// TestCommandCandidatesFitTheirRoom: whatever the names and the room,
// the completion list never draws wider than the room it was given, and
// when it draws anything it includes the name the prompt holds. A
// narrower room never draws a wider list.
func TestCommandCandidatesFitTheirRoom(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		// NoVS16 keeps the widths clear of the base+VS16 disagreement
		// (#301); the list's own arithmetic is what is under test.
		names := rapid.SliceOfN(rapid.Map(proptest.Word(), proptest.NoVS16), 1, 12).Draw(t, "names")
		for i, n := range names {
			if n == "" {
				names[i] = "x"
			}
		}
		idx := rapid.IntRange(0, len(names)-1).Draw(t, "idx")
		room := rapid.IntRange(0, 120).Draw(t, "room")

		m := New()
		m.SetCommandLine(":x")
		m.SetCommandCandidates(names, idx)

		out := m.commandCandidatesView(room)
		if w := lipgloss.Width(out); w > room {
			t.Fatalf("list is %d wide in a room of %d: %q", w, room, ansi.Strip(out))
		}
		if out != "" && !strings.Contains(ansi.Strip(out), names[idx]) {
			t.Fatalf("list %q leaves out the current name %q", ansi.Strip(out), names[idx])
		}
		if room > 0 {
			narrower := m.commandCandidatesView(room - 1)
			if lipgloss.Width(narrower) > lipgloss.Width(out) {
				t.Fatalf("room %d drew %d wide, room %d drew %d wide",
					room-1, lipgloss.Width(narrower), room, lipgloss.Width(out))
			}
		}
	})
}

// TestCandidateWindowHoldsTheCurrentName: the window always contains the
// current name, and is refused only when that name, with both markers,
// does not fit.
func TestCandidateWindowHoldsTheCurrentName(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		widths := rapid.SliceOfN(rapid.IntRange(1, 20), 1, 12).Draw(t, "widths")
		idx := rapid.IntRange(0, len(widths)-1).Draw(t, "idx")
		room := rapid.IntRange(0, 120).Draw(t, "room")

		start, end, ok := candidateWindow(widths, idx, room)
		if !ok {
			if widths[idx]+len(candidateMarker)+len(candidateMoreMarker) <= room {
				t.Fatalf("refused a window although width %d fits room %d with both markers", widths[idx], room)
			}
			return
		}
		if start < 0 || start > idx || idx >= end || end > len(widths) {
			t.Fatalf("window [%d, %d) does not hold idx %d of %d", start, end, idx, len(widths))
		}
	})
}
