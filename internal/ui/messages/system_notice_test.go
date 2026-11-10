package messages

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/gammons/slk/internal/bubbles/entrylist"
	"github.com/gammons/slk/internal/config"
	"github.com/gammons/slk/internal/ui/styles"
)

func joinNotice() MessageItem {
	return MessageItem{
		TS: "1.000000", UserID: "U2", UserName: "alice",
		Text: "<@U2> has joined the channel", Timestamp: "10:30 AM", Subtype: "channel_join",
	}
}

// A reaction on a join (people wave at someone joining) is drawn as a
// pill under the notice, and clicking it toggles it like any other.
func TestNoticeRendersReactionPill(t *testing.T) {
	msg := joinNotice()
	msg.Reactions = []ReactionItem{{Emoji: "wave", Count: 3}}
	res := RenderMessage(msg, 60, &RenderContext{UserNames: map[string]string{"U2": "alice"}}, RenderOptions{})

	lines := strings.Split(ansi.Strip(res.Content), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "alice has joined the channel") || !strings.Contains(lines[1], "3") {
		t.Fatalf("want the notice and then its pill, got %q", lines)
	}
	if len(res.Hits) != 1 {
		t.Fatalf("hits = %+v, want one reaction hit", res.Hits)
	}
	h := res.Hits[0]
	if h.Kind != entrylist.HitReaction || h.Key != "wave" || h.RowStart != 1 || h.RowEnd != 2 || h.ColStart != 1 {
		t.Errorf("hit = %+v, want the wave pill on row 1 from column 1", h)
	}
}

// A search term in a notice is highlighted, and the text after the
// match goes back to the notice's muted style, not the body's colours.
func TestNoticeHighlightsSearchMatchAndStaysMuted(t *testing.T) {
	styles.Apply("dark", config.Theme{})
	t.Cleanup(func() { styles.Apply("dark", config.Theme{}) })
	hlStart, _, ok := SearchHighlightSGR()
	if !ok {
		t.Fatal("no highlight sequence")
	}
	rc := &RenderContext{UserNames: map[string]string{"U2": "alice"}, SearchTerms: []string{"joined"}}
	content := RenderMessage(joinNotice(), 60, rc, RenderOptions{}).Content

	if !strings.Contains(content, hlStart+"joined") {
		t.Fatalf("match not highlighted: %q", content)
	}
	muted := strings.SplitN(styles.Timestamp.Render("\x00"), "\x00", 2)[0]
	after := content[strings.Index(content, hlStart+"joined")+len(hlStart+"joined"):]
	if !strings.Contains(after, muted+" the channel") {
		t.Errorf("text after the match is not muted again: %q", after)
	}
	if strings.Contains(after, FgANSI()) {
		t.Errorf("text after the match restores the body foreground: %q", after)
	}
}
