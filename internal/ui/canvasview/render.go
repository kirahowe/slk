// Package canvasview draws a Slack canvas (a parsed canvas.Document) as
// a read-only, scrollable overlay.
package canvasview

import (
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"

	"github.com/gammons/slk/internal/bubbles/ansi"
	"github.com/gammons/slk/internal/canvas"
	"github.com/gammons/slk/internal/ui/messages"
	"github.com/gammons/slk/internal/ui/styles"
)

// Names resolves a user ID to a display name for a mention chip.
type Names func(userID string) (string, bool)

// Render draws doc as lines exactly width cells wide on the theme
// background. It reads the theme on every call, so a theme change
// applies on the next draw.
func Render(doc canvas.Document, width int, names Names) []string {
	if width < 1 {
		return nil
	}
	var out []string
	for i, b := range doc.Blocks {
		if b.Kind == canvas.Heading && i > 0 && len(out) > 0 && !isBlankLine(out[len(out)-1]) {
			out = append(out, fill("", width))
		}
		first, rest := prefixes(b)
		body := wrapBlock(b, width-lipgloss.Width(first), names)
		for j, line := range body {
			p := rest
			if j == 0 {
				p = first
			}
			out = append(out, fill(prefixStyle(b).Render(p)+line, width))
		}
	}
	return out
}

// wrapBlock is a block's text, styled and wrapped to width, without its
// marker or indent.
func wrapBlock(b canvas.Block, width int, names Names) []string {
	if width < 1 {
		width = 1
	}
	if b.Kind == canvas.Code {
		var plain strings.Builder
		for _, s := range b.Spans {
			plain.WriteString(s.Text)
		}
		cs := codeBlockStyle()
		wrapped := xansi.Hardwrap(plain.String(), width, true)
		lines := strings.Split(wrapped, "\n")
		for i, l := range lines {
			lines[i] = cs.Render(l + strings.Repeat(" ", max(0, width-lipgloss.Width(l))))
		}
		return lines
	}
	text := styledSpans(b, names)
	if text == "" {
		return []string{""}
	}
	return strings.Split(messages.WordWrap(text, width), "\n")
}

// styledSpans renders a block's spans. Each word is styled on its own,
// so wrapping between words never splits an escape sequence or an OSC 8
// hyperlink across lines.
func styledSpans(b canvas.Block, names Names) string {
	var parts []string
	for _, s := range b.Spans {
		text, st := s.Text, spanStyle(b, s)
		switch s.Chip {
		case canvas.UserChip:
			if name, ok := names(s.ID); ok && name != "" {
				text = "@" + name
			}
		case canvas.DateChip:
			text = "📅 " + text
		}
		parts = append(parts, styleWords(text, st, s.Link))
	}
	return strings.Join(parts, "")
}

func styleWords(text string, st lipgloss.Style, link string) string {
	var b strings.Builder
	start := -1
	emit := func(end int) {
		w := st.Render(text[start:end])
		if link != "" {
			w = "\x1b]8;;" + link + "\x1b\\" + w + "\x1b]8;;\x1b\\"
		}
		b.WriteString(w)
		start = -1
	}
	for i, r := range text {
		if r == ' ' {
			if start >= 0 {
				emit(i)
			}
			b.WriteByte(' ')
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		emit(len(text))
	}
	return b.String()
}

// prefixes returns a block's first-line prefix (indent and marker) and
// the prefix of its wrapped lines, which lines them up under the text.
func prefixes(b canvas.Block) (string, string) {
	switch b.Kind {
	case canvas.Quote:
		return "│ ", "│ "
	case canvas.ListItem:
		indent := strings.Repeat("  ", b.Depth)
		marker := bullets[b.Depth%len(bullets)] + " "
		if b.List == canvas.Numbered {
			marker = strconv.Itoa(b.Number) + ". "
		}
		hang := indent + strings.Repeat(" ", lipgloss.Width(marker))
		if b.Continued {
			return hang, hang
		}
		return indent + marker, hang
	}
	return "", ""
}

var bullets = []string{"•", "◦", "▪"}

func spanStyle(b canvas.Block, s canvas.Span) lipgloss.Style {
	st := lipgloss.NewStyle().Background(styles.Background).Foreground(styles.TextPrimary)
	switch b.Kind {
	case canvas.Heading:
		st = st.Bold(true)
		switch b.Level {
		case 1:
			st = st.Foreground(styles.Primary)
		case 2:
			st = st.Foreground(styles.Accent)
		}
	case canvas.Quote:
		st = st.Foreground(styles.TextMuted)
	}
	if s.Bold {
		st = st.Bold(true)
	}
	if s.Italic {
		st = st.Italic(true)
	}
	if s.Code {
		st = st.Foreground(styles.Warning).Background(styles.Surface)
	}
	if s.Link != "" {
		st = st.Foreground(styles.Primary).Underline(true)
	}
	if s.Chip == canvas.UserChip {
		st = st.Foreground(styles.Primary).Bold(true)
	}
	return st
}

func prefixStyle(b canvas.Block) lipgloss.Style {
	st := lipgloss.NewStyle().Background(styles.Background).Foreground(styles.TextMuted)
	if b.Kind == canvas.ListItem {
		st = st.Foreground(styles.Primary)
	}
	return st
}

func codeBlockStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(styles.Warning).Background(styles.Surface)
}

// fill pads line to width on the theme background, restoring the
// background after every reset inside it.
func fill(line string, width int) string {
	line = ansi.ReapplyAfterResets(line, messages.BgANSI()+messages.FgANSI())
	pad := width - lipgloss.Width(line)
	if pad > 0 {
		line += lipgloss.NewStyle().Background(styles.Background).Render(strings.Repeat(" ", pad))
	}
	return line
}

func isBlankLine(line string) bool {
	return strings.TrimSpace(xansi.Strip(line)) == ""
}
