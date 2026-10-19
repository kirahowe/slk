package messages

import (
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/gammons/slk/internal/ui/styles"
)

// CardLine draws one line of a list card (the Threads and Activity
// views): line clipped to contentWidth with an ellipsis, filled with the
// theme background, or the selection tint when selected, behind a
// one-column left border that is the ▌ selection bar on the selected card
// and invisible otherwise. Each styled span in line ends in a reset that
// drops the row's background, so the background is restored after every
// one and the row is filled edge to edge.
func CardLine(line string, contentWidth int, selected, focused bool) string {
	line = ClipToWidth(line, contentWidth)
	bg := BgANSI()
	if selected {
		bg = SelectionTintBgANSI(focused)
	}
	line = ReapplyBgAfterResets(line, bg)
	border := lipgloss.NewStyle().
		BorderStyle(thickLeftBorder).BorderLeft(true).
		BorderForeground(styles.Background).
		BorderBackground(styles.Background)
	fill := lipgloss.NewStyle().Background(styles.Background).Width(contentWidth)
	if selected {
		border = lipgloss.NewStyle().
			BorderStyle(thickLeftBorder).BorderLeft(true).
			BorderForeground(styles.SelectionBorderColor(focused)).
			BorderBackground(styles.SelectionTintColor(focused)).
			Background(styles.SelectionTintColor(focused))
		fill = lipgloss.NewStyle().
			Background(styles.SelectionTintColor(focused)).
			Width(contentWidth)
	}
	return border.Render(fill.Render(line))
}

// ClipToWidth truncates an already-styled s to at most width display
// columns, ending in an ellipsis when it cuts; ANSI-aware.
func ClipToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	return ansi.Truncate(s, width, "…")
}
