package thread

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"pgregory.net/rapid"

	"github.com/gammons/slk/internal/proptest"
	"github.com/gammons/slk/internal/ui/messages"
)

// The click target renderBreadcrumb reports is the channel label as
// drawn: its span lies inside the row, the cells it covers are the glyph
// and name (or their "…" truncation) and nothing else, and it is empty
// when the row shows no channel.
func TestRenderBreadcrumbLabelSpanIsTheDrawnLabel(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		width := rapid.IntRange(0, 80).Draw(t, "width")
		channel := proptest.NoVS16(rapid.OneOf(rapid.Just(""), proptest.Word()).Draw(t, "channel"))
		chType := rapid.SampledFrom([]string{"channel", "private", "dm"}).Draw(t, "chType")
		author := proptest.NoVS16(rapid.OneOf(rapid.Just(""), proptest.Word()).Draw(t, "author"))
		replies := rapid.IntRange(0, 120).Draw(t, "replies")

		row, from, to := renderBreadcrumb(width, channel, chType, author, replies)
		if from < 0 || from > to || to > width {
			t.Fatalf("span [%d, %d) outside a %d-cell row", from, to, width)
		}
		if from == to {
			return
		}
		if channel == "" {
			t.Fatalf("span [%d, %d) with no channel", from, to)
		}
		label := messages.ChannelGlyph(chType) + " " + channel
		got := ansi.Cut(ansi.Strip(row), from, to)
		if got != label && (!strings.HasSuffix(got, "…") || !strings.HasPrefix(label, strings.TrimSuffix(got, "…"))) {
			t.Fatalf("span covers %q, want the label %q or its truncation", got, label)
		}
	})
}
