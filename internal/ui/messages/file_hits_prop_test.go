package messages

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"pgregory.net/rapid"

	"github.com/gammons/slk/internal/bubbles/entrylist"
	"github.com/gammons/slk/internal/proptest"
)

// RenderMessage records one HitFile rect per downloadable attachment and
// none for any other: each names its attachment by index, and on the
// rows WrapRows draws, the rect covers that attachment's
// "[File] <name>" line (cut with "…" when it is wider than the pane) and
// stays inside the row, so a click there downloads the file it shows.
func TestRenderMessageFileHitsCoverTheirLines(t *testing.T) {
	applyDarkTheme(t)
	const self = "U0SELF0001"
	rapid.Check(t, func(t *rapid.T) {
		msg := propMessage(self).Draw(t, "msg")
		for i := range rapid.IntRange(0, 4).Draw(t, "attachments") {
			att := Attachment{
				Kind: rapid.SampledFrom([]string{"file", "canvas", "image"}).Draw(t, "kind"),
				// Up to 96 runes: names wider than the pane must still
				// take one row each.
				Name: proptest.NoVS16(strings.Repeat(proptest.Word().Draw(t, "name"), rapid.IntRange(0, 12).Draw(t, "repeat"))),
				URL:  "https://acme.slack.com/files/U1/F1/x",
			}
			if rapid.Bool().Draw(t, "downloadable") {
				att.DownloadURL = "https://files.slack.com/files-pri/T1-F1/x"
			}
			if att.Kind == "canvas" {
				att.FileID = "F0CANVAS" + string(rune('0'+i))
			}
			msg.Attachments = append(msg.Attachments, att)
		}
		width := rapid.IntRange(40, 100).Draw(t, "width")
		m := New([]MessageItem{msg}, "general")
		m.SetCurrentUser(self)
		r := RenderMessage(msg, width, m.renderContext(), RenderOptions{ImageHits: true})
		w := WrapRows(msg, r, width, NewWrapStyles(false))

		var want []int
		for i, att := range msg.Attachments {
			if att.Downloadable() {
				want = append(want, i)
			}
		}
		var got []int
		for _, h := range r.Hits {
			if h.Kind != entrylist.HitFile {
				continue
			}
			got = append(got, h.Aux)
			if h.RowStart < 0 || h.RowEnd <= h.RowStart || h.RowEnd > len(w.Lines) {
				t.Fatalf("hit rows [%d, %d) outside the %d rows drawn", h.RowStart, h.RowEnd, len(w.Lines))
			}
			if h.ColEnd > width {
				t.Fatalf("hit columns [%d, %d) past the %d-column row", h.ColStart, h.ColEnd, width)
			}
			att := msg.Attachments[h.Aux]
			label := att.Name
			if label == "" {
				label = att.URL
			}
			full := "[File] " + label
			line := strings.TrimRight(ansi.Cut(ansi.Strip(w.Lines[h.RowStart]), h.ColStart, h.ColEnd), " ")
			if line != full && (!strings.HasSuffix(line, "…") || !strings.HasPrefix(full, strings.TrimSuffix(line, "…"))) {
				t.Fatalf("hit for attachment %d covers %q, want %q or its truncation; rows %+q", h.Aux, line, full, ansi.Strip(strings.Join(w.Lines, "\n")))
			}
		}
		if len(got) != len(want) {
			t.Fatalf("file hits for attachments %v, want %v", got, want)
		}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("file hits for attachments %v, want %v", got, want)
			}
		}
	})
}
