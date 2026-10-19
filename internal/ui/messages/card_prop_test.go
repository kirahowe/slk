package messages

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"pgregory.net/rapid"

	"github.com/gammons/slk/internal/config"
	"github.com/gammons/slk/internal/proptest"
	"github.com/gammons/slk/internal/ui/styles"
)

// fgSGR is an SGR that sets no background: a reset (both forms), an
// attribute or a foreground colour, as the spans on a card line are.
func fgSGR() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		switch rapid.IntRange(0, 3).Draw(t, "kind") {
		case 0:
			return rapid.SampledFrom([]string{"\x1b[m", "\x1b[0m"}).Draw(t, "reset")
		case 1:
			return rapid.SampledFrom([]string{"\x1b[1m", "\x1b[3m", "\x1b[22m"}).Draw(t, "attr")
		case 2:
			return fmt.Sprintf("\x1b[%dm", rapid.SampledFrom([]int{31, 34, 90, 97}).Draw(t, "fg16"))
		default:
			return fmt.Sprintf("\x1b[38;2;%d;%d;%dm", rapid.IntRange(0, 255).Draw(t, "r"),
				rapid.IntRange(0, 255).Draw(t, "g"), rapid.IntRange(0, 255).Draw(t, "b"))
		}
	})
}

// cellBackgrounds returns, for each printable rune in line, the background
// SGR parameters in effect ("" for the terminal default). OSC sequences
// (hyperlinks) are skipped.
func cellBackgrounds(line string) []string {
	var out []string
	bg := ""
	for i := 0; i < len(line); {
		switch {
		case strings.HasPrefix(line[i:], "\x1b]"):
			end := strings.Index(line[i:], "\x1b\\")
			if end < 0 {
				return out
			}
			i += end + 2
		case strings.HasPrefix(line[i:], "\x1b["):
			end := strings.IndexByte(line[i:], 'm')
			if end < 0 {
				return out
			}
			ps := strings.Split(line[i+2:i+end], ";")
			for j := 0; j < len(ps); j++ {
				switch p := ps[j]; {
				case p == "" || p == "0":
					bg = ""
				case (p == "38" || p == "48") && j+1 < len(ps) && ps[j+1] == "2" && j+4 < len(ps):
					if p == "48" {
						bg = strings.Join(ps[j:j+5], ";")
					}
					j += 4
				case (p == "38" || p == "48") && j+1 < len(ps) && ps[j+1] == "5" && j+2 < len(ps):
					if p == "48" {
						bg = strings.Join(ps[j:j+3], ";")
					}
					j += 2
				case len(p) == 2 && p[0] == '4' && p[1] <= '7', len(p) == 3 && p[:2] == "10" && p[2] <= '7':
					bg = p
				}
			}
			i += end + 1
		default:
			_, n := utf8.DecodeRuneInString(line[i:])
			out = append(out, bg)
			i += n
		}
	}
	return out
}

// Every cell of a card line, the selection bar and the padding included,
// is drawn on the row's background: the selection tint on the selected
// card, the theme background otherwise. No styled span in the line can
// leave a cell on the terminal default.
func TestCardLineFillsEveryCell(t *testing.T) {
	styles.Apply("dark", config.Theme{})
	t.Cleanup(func() { styles.Apply("dark", config.Theme{}) })
	bgOf := func(c interface{ RGBA() (r, g, b, a uint32) }) string {
		r, g, b, _ := c.RGBA()
		return fmt.Sprintf("48;2;%d;%d;%d", r>>8, g>>8, b>>8)
	}
	rapid.Check(t, func(t *rapid.T) {
		// One line: both callers flatten newlines before drawing. No
		// base+VS16, whose width terminals and lipgloss disagree on (#301).
		line := proptest.NoVS16(strings.ReplaceAll(proptest.StyledTextWith(fgSGR()).Draw(t, "line"), "\n", " "))
		width := rapid.IntRange(1, 60).Draw(t, "width")
		selected := rapid.Bool().Draw(t, "selected")
		focused := rapid.Bool().Draw(t, "focused")

		want := bgOf(styles.Background)
		if selected {
			want = bgOf(styles.SelectionTintColor(focused))
		}
		got := CardLine(line, width, selected, focused)
		cells := cellBackgrounds(got)
		if w := lipgloss.Width(got); w != width+1 {
			t.Fatalf("drawn %d wide, want %d: %q", w, width+1, got)
		}
		for i, bg := range cells {
			if bg != want {
				t.Fatalf("cell %d background %q, want %q:\n%q", i, bg, want, got)
			}
		}
	})
}
