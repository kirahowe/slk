package canvasview

import (
	"fmt"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"

	"github.com/gammons/slk/internal/bubbles/chrome"
	"github.com/gammons/slk/internal/bubbles/scrollbar"
	"github.com/gammons/slk/internal/core"
	"github.com/gammons/slk/internal/ui/messages"
	"github.com/gammons/slk/internal/ui/overlay"
	"github.com/gammons/slk/internal/ui/styles"
)

var _ chrome.Dialog = (*Model)(nil)

// maxTextWidth caps the text column, so a canvas on a wide terminal
// reads as a page rather than a few very long lines.
const maxTextWidth = 100

type state int

const (
	// stateListing: waiting for the conversation's canvas list.
	stateListing state = iota
	stateLoading
	stateLoaded
	stateFailed
)

// Model is the canvas viewer overlay. It does no I/O: the App fetches,
// and each request carries the Req the model handed out, so a result
// for a canvas the user has since moved away from is dropped.
type Model struct {
	visible bool
	teamID  string
	files   []string
	idx     int
	req     int
	state   state
	canvas  core.Canvas
	err     error
	offset  int
	// rows is how many body rows the last frame showed, for paging.
	rows int
}

// New returns a hidden viewer.
func New() *Model { return &Model{} }

// IsVisible reports whether the viewer is showing.
func (m *Model) IsVisible() bool { return m.visible }

// Close hides the viewer and clears its state. The request counter is
// kept, so a fetch started before Close can never match a later Open.
func (m *Model) Close() { *m = Model{req: m.req} }

// TeamID is the workspace the open canvas belongs to.
func (m *Model) TeamID() string { return m.teamID }

// Permalink is the open canvas's Slack link, "" until it is known.
func (m *Model) Permalink() string { return m.canvas.Permalink }

// OpenConversation shows the viewer while the App lists a
// conversation's canvases. It returns the request to answer with
// SetFiles.
func (m *Model) OpenConversation(teamID string) int {
	m.Close()
	m.visible, m.teamID, m.state = true, teamID, stateListing
	m.req++
	return m.req
}

// OpenFile shows one canvas. It returns the request to answer with
// SetCanvas.
func (m *Model) OpenFile(teamID, fileID string) int {
	m.Close()
	m.visible, m.teamID, m.files = true, teamID, []string{fileID}
	return m.load()
}

// SetFiles applies a conversation's canvas list. It returns the canvas
// to fetch and its request, or ok false when the result is stale, failed
// (the viewer shows the error) or empty (the caller closes the viewer).
func (m *Model) SetFiles(req int, ids []string, err error) (fileID string, next int, ok bool) {
	if !m.visible || req != m.req || m.state != stateListing {
		return "", 0, false
	}
	if err != nil {
		m.state, m.err = stateFailed, err
		return "", 0, false
	}
	if len(ids) == 0 {
		return "", 0, false
	}
	m.files, m.idx = ids, 0
	return m.files[0], m.load(), true
}

// Empty reports whether a conversation's listing came back with no
// canvases: the viewer is still listing but has nothing to show.
func (m *Model) Empty(req int, ids []string, err error) bool {
	return m.visible && req == m.req && m.state == stateListing && err == nil && len(ids) == 0
}

// SetCanvas applies a fetched canvas. It returns false for a stale
// result.
func (m *Model) SetCanvas(req int, c core.Canvas, err error) bool {
	if !m.visible || req != m.req || m.state != stateLoading {
		return false
	}
	m.canvas = c
	if err != nil {
		m.state, m.err = stateFailed, err
		return true
	}
	m.state, m.err, m.offset = stateLoaded, nil, 0
	return true
}

// Cycle moves to the next (delta 1) or previous (-1) canvas of the
// conversation, returning the canvas to fetch and its request.
func (m *Model) Cycle(delta int) (fileID string, req int, ok bool) {
	if len(m.files) < 2 {
		return "", 0, false
	}
	m.idx = (m.idx + delta + len(m.files)) % len(m.files)
	m.canvas = core.Canvas{}
	return m.files[m.idx], m.load(), true
}

// Reload fetches the current canvas again.
func (m *Model) Reload() (fileID string, req int, ok bool) {
	if len(m.files) == 0 || m.state == stateLoading {
		return "", 0, false
	}
	permalink := m.canvas.Permalink
	req = m.load()
	m.canvas.Permalink = permalink
	return m.files[m.idx], req, true
}

func (m *Model) load() int {
	m.state, m.err, m.offset = stateLoading, nil, 0
	m.req++
	return m.req
}

// Scroll moves the view by n lines; the next frame clamps it.
func (m *Model) Scroll(n int) {
	m.offset = max(0, m.offset+n)
}

// HalfPage and Page are the scroll distances for the last frame's
// height.
func (m *Model) HalfPage() int { return max(1, m.rows/2) }
func (m *Model) Page() int     { return max(1, m.rows-1) }

// Top and Bottom jump to either end.
func (m *Model) Top()    { m.offset = 0 }
func (m *Model) Bottom() { m.offset = 1 << 30 }

// ViewOverlay draws the viewer over background, which it dims.
func (m *Model) ViewOverlay(termW, termH int, background string, names Names) string {
	if !m.visible {
		return background
	}
	box := m.renderBox(termW, termH, names)
	out := overlay.DimmedOverlay(termW, termH, background, box, 0.5)
	lines := strings.Split(out, "\n")
	if len(lines) > termH {
		lines = lines[:termH]
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderBox(termW, termH int, names Names) string {
	s := messages.ChromeStyles()
	boxW := max(20, termW-4)
	boxH := max(10, termH-2)
	frameW := s.Box.GetHorizontalFrameSize()
	innerW := boxW - frameW
	// Title, blank, blank, footer, plus the box's own frame.
	m.rows = max(1, boxH-s.Box.GetVerticalFrameSize()-4)

	textW := min(innerW-2, maxTextWidth)
	var all []string
	switch m.state {
	case stateLoaded:
		all = Render(m.canvas.Doc, textW, names)
		if len(all) == 0 {
			all = []string{s.Empty.Render("This canvas is empty.")}
		}
	case stateFailed:
		all = []string{
			lipgloss.NewStyle().Background(styles.Background).Foreground(styles.Error).
				Render(xansi.Truncate("Couldn't load the canvas: "+errText(m.err), textW, "…")),
			"",
			s.Empty.Render("r to retry"),
		}
	default:
		all = []string{s.Empty.Render("Loading canvas…")}
	}

	maxOff := max(0, len(all)-m.rows)
	m.offset = min(m.offset, maxOff)
	end := min(len(all), m.offset+m.rows)

	left := (innerW - textW) / 2
	pad := func(n int) string {
		return lipgloss.NewStyle().Background(styles.Background).Render(strings.Repeat(" ", max(0, n)))
	}
	body := make([]string, 0, m.rows)
	for _, l := range all[m.offset:end] {
		body = append(body, pad(left)+l+pad(innerW-left-lipgloss.Width(l)))
	}
	for len(body) < m.rows {
		body = append(body, pad(innerW))
	}
	body = scrollbar.Overlay(body, innerW, len(all), m.offset, m.rows, styles.Background, styles.Border, styles.Primary)

	return chrome.Frame{
		Title:  xansi.Truncate(m.title(), innerW, "…"),
		Body:   body,
		Footer: s.Footer.Render(xansi.Truncate(m.footer(), innerW, "…")),
	}.Render(s, boxW)
}

func (m *Model) title() string {
	t := printable(m.canvas.Title)
	if t == "" {
		t = "Canvas"
	}
	if len(m.files) > 1 {
		t += fmt.Sprintf("  (%d of %d)", m.idx+1, len(m.files))
	}
	return t
}

func (m *Model) footer() string {
	keys := "j/k scroll  "
	if len(m.files) > 1 {
		keys += "tab next canvas  "
	}
	return keys + "o open in Slack  r reload  esc close"
}

func errText(err error) string {
	if err == nil {
		return "unknown error"
	}
	return printable(err.Error())
}

// printable drops control characters from text that came from Slack
// (a title, an error carrying a response body) before it is drawn.
func printable(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}
