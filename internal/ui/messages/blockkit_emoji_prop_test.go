package messages

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"pgregory.net/rapid"

	"github.com/gammons/slk/internal/proptest"
	"github.com/gammons/slk/internal/ui/imgrender"
	"github.com/gammons/slk/internal/ui/messages/blockkit"
)

// labelWithShortcodes is plain text from proptest (newlines made spaces,
// raw VS16 taken out, see proptest.NoVS16) with emoji shortcodes mixed
// in: plain, base+VS16 (:heart:), ZWJ and flag (left as shortcodes),
// skin-toned and unknown.
func labelWithShortcodes() *rapid.Generator[string] {
	return rapid.Custom(func(t *rapid.T) string {
		var b strings.Builder
		for range rapid.IntRange(1, 4).Draw(t, "parts") {
			b.WriteString(proptest.NoVS16(strings.ReplaceAll(proptest.PlainText().Draw(t, "text"), "\n", " ")))
			b.WriteString(rapid.SampledFrom([]string{
				":white_check_mark:", ":heart:", ":mountain:", ":+1:", ":wave::skin-tone-3:",
				":man-woman-girl:", ":flag-us:", ":keycap_ten:", ":not_an_emoji:", ":rocket:",
			}).Draw(t, "shortcode"))
		}
		return b.String()
	})
}

// A Block Kit header, and a row of buttons, is never wider than the width
// it is rendered at, for any label: the plain-text emoji path draws
// shortcodes through the VS16 gate, so the width lipgloss measures for a
// truncation or a wrap is the width drawn.
func TestBlockKitLabelRowsFitTheirWidth(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		width := rapid.IntRange(1, 60).Draw(t, "width")
		header := labelWithShortcodes().Draw(t, "header")
		var buttons []blockkit.ActionElement
		for range rapid.IntRange(1, 4).Draw(t, "buttons") {
			buttons = append(buttons, blockkit.ActionElement{Kind: "button", Label: labelWithShortcodes().Draw(t, "label")})
		}
		msg := MessageItem{TS: "1.0", Blocks: []blockkit.Block{
			blockkit.HeaderBlock{Text: header},
			blockkit.ActionsBlock{Elements: buttons},
		}}
		rc := &RenderContext{Images: imgrender.NewRenderer()}
		res := blockkit.Render(msg.Blocks, BlockkitContext(msg, rc, nil), width)
		for i, line := range res.Lines {
			if w := lipgloss.Width(line); w > width {
				t.Fatalf("row %d is %d wide, over %d: %q", i, w, width, line)
			}
			if strings.Contains(line, "❤️") || strings.Contains(line, "⛰️") {
				t.Fatalf("row %d draws a base+VS16 pair: %q", i, line)
			}
		}
	})
}
