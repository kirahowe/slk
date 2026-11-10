// internal/ui/draft_preview.go
//
// The draft preview: the draft in a compose box drawn as its message
// will look once sent, in a few rows above the box, inside the same
// pane border (ctrl+r; docs/superpowers/specs/
// 2026-11-10-compose-draft-preview-design.md). Its on/off state and the
// draft's wire text live on compose.Model (PreviewOn, PreviewWire); the
// pane draws the rows (contentPane.RenderDraft); this file decides what
// the rows say and caches them, and the two bottom-region renderers
// (view_messages.go, view_thread.go) size and stack them.
package ui

import (
	"bytes"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	imgpkg "github.com/gammons/slk/internal/image"
	"github.com/gammons/slk/internal/ui/compose"
	"github.com/gammons/slk/internal/ui/messages"
	"github.com/gammons/slk/internal/ui/styles"
)

// draftPreviewRows is how many rows the draft preview may take, its
// title row included: a third of the pane's inner height (at least 2),
// and no more than room, what is left above the message list's 3-row
// minimum. Under 2 there is no room for the title and a line, so none.
func draftPreviewRows(inner, room int) int {
	n := min(max(2, inner/3), room)
	if n < 2 {
		return 0
	}
	return n
}

// draftPreviewBox is one compose box's draft preview: the box, the pane
// it sits in (which draws the draft), which panel that is, the thread
// a reply goes to ("" for the channel box), and the box's render cache.
type draftPreviewBox struct {
	compose  *compose.Model
	pane     contentPane
	panel    Panel
	threadTS string
	cache    *draftPreviewCache
	// window tells apart the channel panes of different windows, whose
	// versions are counted independently.
	window int64
}

// draftPreviewCache holds a box's rendered draft for everything the
// render reads besides the pane's state, which pane.Version covers.
type draftPreviewCache struct {
	key  draftPreviewKey
	body messages.Draft
	ok   bool
}

type draftPreviewKey struct {
	wire        string
	width       int
	paneVersion int64
	window      int64
	theme       int64
	// The user's own avatar can arrive without changing the pane's
	// version, and the body wraps narrower beside one.
	selfID, avatar string
}

// draftPreview returns the draft preview's rows for box b, each width
// cells, or "" when it is off or maxRows is under 2: a title rule, then
// one dim note, or the draft drawn as the pane will draw its message
// once sent. Past maxRows the body keeps its end, under a dim "…" row
// when there is room for one besides a line. The emoji images in the
// rows are uploaded as the frame is drawn, as a pane's rows are.
func (a *App) draftPreview(b draftPreviewBox, width, maxRows int) string {
	if !b.compose.PreviewOn() || maxRows < 2 || width < 1 {
		*b.cache = draftPreviewCache{}
		return ""
	}
	rows := []string{draftPreviewLine("── draft preview "+strings.Repeat("─", width), width)}
	if note := a.draftPreviewNote(b.compose, b.panel); note != "" {
		return strings.Join(append(rows, draftPreviewLine("  "+note, width)), "\n")
	}
	body := a.draftBody(b, width)
	uploadDraft(body.Uploads)
	lines := body.Rows
	switch {
	case len(lines) <= maxRows-1:
	case maxRows == 2:
		lines = lines[len(lines)-1:]
	default:
		lines = append([]string{draftPreviewLine("  …", width)}, lines[len(lines)-(maxRows-2):]...)
	}
	return strings.Join(append(rows, lines...), "\n")
}

// draftBody is b's draft as the pane draws it: the placeholder row a
// send makes for it (localRow of messages.TypedBody of the wire text),
// drawn by the pane without its header. Cached until the draft, the
// pane, the width, the theme or the user's avatar changes.
func (a *App) draftBody(b draftPreviewBox, width int) messages.Draft {
	key := draftPreviewKey{
		wire: b.compose.PreviewWire(), width: width,
		paneVersion: b.pane.Version(), window: b.window, theme: styles.Version(),
		selfID: a.currentUserID,
	}
	if a.avatarFn != nil {
		key.avatar = a.avatarFn(a.currentUserID)
	}
	if c := b.cache; c.ok && c.key == key {
		return c.body
	}
	row := a.localRow("", messages.TypedBody(key.wire), b.threadTS, "")
	body := b.pane.RenderDraft(row, width)
	*b.cache = draftPreviewCache{key: key, body: body, ok: true}
	return body
}

// draftPreviewNote is the one dim line the preview shows in place of a
// body, or "" for a draft it draws: a slash command (Enter runs it; an
// edit is saved as written, so not while editing in this box), a
// caption sent with files (posted as typed, not converted), or nothing
// to show.
func (a *App) draftPreviewNote(c *compose.Model, panel Panel) string {
	text := c.Value()
	editing := a.editing.IsActive() && a.editing.Panel() == panel
	if _, _, ok := parseSlashCommand(text); ok && !editing {
		return "slash command, not a message"
	}
	if len(c.Attachments()) > 0 {
		return "caption is sent as typed"
	}
	if strings.TrimSpace(text) == "" {
		return "nothing to preview"
	}
	return ""
}

// draftPreviewLine is text as one muted row, cut and padded to width.
func draftPreviewLine(text string, width int) string {
	return lipgloss.NewStyle().Background(styles.Background).Foreground(styles.TextMuted).
		Width(width).Render(ansi.Truncate(text, width, ""))
}

// uploadDraft writes the draft's kitty uploads straight to the
// terminal, as the panes write theirs: APC sequences inside line
// content get mangled by the renderer. Each upload sends its image
// once, so a repeat frame writes nothing.
func uploadDraft(uploads []func(io.Writer) error) {
	var buf bytes.Buffer
	for _, fl := range uploads {
		if fl != nil {
			_ = fl(&buf)
		}
	}
	if buf.Len() > 0 {
		_, _ = imgpkg.KittyOutput.Write(buf.Bytes())
	}
}
