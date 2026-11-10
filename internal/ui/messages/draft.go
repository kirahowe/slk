package messages

import (
	"io"
	"slices"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/gammons/slk/internal/ui/styles"
)

// Draft is a message the user has not sent yet, drawn as a content pane
// would draw its row once it is sent, without the author header: the
// draft preview above a compose box.
type Draft struct {
	// Rows are the row's lines below its header, each exactly the
	// pane's width: the invisible left border, the body and the fill.
	Rows []string
	// Uploads are the kitty uploads of the emoji images in Rows. Each
	// sends its image once, so writing them every frame costs nothing
	// after the first.
	Uploads []func(io.Writer) error
}

// RenderDraft draws msg as a pane's unselected row for it, rendered with
// the pane's rc and opt (RenderMessage) and bordered and filled as
// WrapRows does, but with RenderOptions.BodyOnly: no label rows and no
// author header. What is not part of how the sent message looks is off:
// search highlighting, the reaction cursor, hit rects and sixel rows
// (a sixel image is drawn as its halfblock fallback). Every row is cut
// and padded to exactly width cells, since RenderMessage never wraps
// narrower than 20.
func RenderDraft(msg MessageItem, width int, rc *RenderContext, opt RenderOptions) Draft {
	ctx := *rc
	ctx.SearchTerms = nil
	ctx.Nav = ReactionNav{}
	opt.BodyOnly = true
	opt.IsSelected, opt.ImageHits, opt.SixelRows = false, false, false
	opt.stats = nil
	r := RenderMessage(msg, width, &ctx, opt)
	rows := NewWrapStyles(false).Unmarked(r.Content, width)
	pad := lipgloss.NewStyle().Background(styles.Background)
	for i, row := range rows {
		// A wide character cut by the edge leaves the row a cell short.
		row = ansi.Truncate(row, width, "")
		if short := width - ansi.StringWidth(row); short > 0 {
			row += pad.Render(strings.Repeat(" ", short))
		}
		rows[i] = row
	}
	return Draft{
		Rows:    rows,
		Uploads: slices.Concat(r.BlockFlushes, r.AttachmentFlushes, r.BodyFlushes, r.ReactionFlushes),
	}
}
