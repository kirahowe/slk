package messages

import (
	"bytes"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	emojiutil "github.com/gammons/slk/internal/emoji"
	imgpkg "github.com/gammons/slk/internal/image"
	"github.com/gammons/slk/internal/ui/peerstatus"
)

// In image mode an author's status emoji is drawn as the image a message
// body would use for it (a workspace emoji of that name included), and
// its upload goes out with the body's. A huddle, and a status emoji with
// no image of its own, draw their glyph's image.
func TestAuthorStatusEmojiUsesMessageImage(t *testing.T) {
	t.Setenv("TMUX", "")
	emojiutil.SetImageMode(true, 2)
	t.Cleanup(func() { emojiutil.SetImageMode(false, 2) })
	for _, tc := range []struct {
		name, body, url string
		status          peerstatus.Status
		customs         map[string]string
	}{
		{"same emoji in body", "📟", emojiutil.CDNBaseURL + "1f4df.png", peerstatus.Status{Emoji: ":pager:", Text: "On call"}, nil},
		{"status only", "On call", emojiutil.CDNBaseURL + "1f4df.png", peerstatus.Status{Emoji: ":pager:", Text: "On call"}, nil},
		{"custom status", "On call", "https://example.test/custom-pager.png", peerstatus.Status{Emoji: ":pager:", Text: "On call"}, map[string]string{"pager": "https://example.test/custom-pager.png"}},
		{"unknown status fallback", "On call", emojiutil.CDNBaseURL + "1f4ac.png", peerstatus.Status{Emoji: ":unknown_status_fixture:", Text: "On call"}, nil},
		{"huddle takes priority", "On call", emojiutil.CDNBaseURL + "1f3a7.png", peerstatus.Status{Emoji: ":pager:", Huddle: peerstatus.HuddleActive}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fetcher, placed := fixtureEmoji(t, tc.url)
			msg := MessageItem{TS: "1.0", UserID: "U_FIXTURE", UserName: "fixture", Text: tc.body}
			m := New([]MessageItem{msg}, "fixture")
			m.SetUserStatuses(map[string]peerstatus.Status{"U_FIXTURE": tc.status})
			m.SetEmojiContext(EmojiContext{PlaceCtx: emojiutil.PlaceContext{Fetcher: fetcher}, Cells: 2, Customs: tc.customs})
			rendered := RenderMessage(msg, 80, m.renderContext(), RenderOptions{})
			if !strings.ContainsRune(rendered.Header, imgpkg.PlaceholderRune) || strings.Contains(rendered.Header, "📟") {
				t.Fatalf("status emoji did not use the message image: %q", rendered.Header)
			}
			for _, id := range imageCellIDs(t, rendered.Content, 80, lipgloss.Height(rendered.Content)) {
				if id != placed.ID {
					t.Fatalf("status emoji references image %d, want %d", id, placed.ID)
				}
			}
			var uploads bytes.Buffer
			for _, flush := range rendered.BodyFlushes {
				if err := flush(&uploads); err != nil {
					t.Fatal(err)
				}
			}
			if want := fmt.Sprintf("\x1b_Ga=T,f=100,t=d,i=%d,", placed.ID); !strings.Contains(uploads.String(), want) {
				t.Fatalf("status emoji was not uploaded for its placement: %q", uploads.String())
			}
		})
	}
}

// When the status emoji's image cannot be fetched, the header keeps the
// glyph it drew before, on the render after the failure and later ones.
func TestAuthorStatusEmojiFetchFailurePreservesGlyph(t *testing.T) {
	t.Setenv("TMUX", "")
	emojiutil.SetImageMode(true, 2)
	t.Cleanup(func() { emojiutil.SetImageMode(false, 2) })
	for _, tc := range []struct {
		name, shortcode string
	}{
		{"standard glyph", "pager"},
		{"custom status fallback", "team_status_fixture"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Each run needs its own failed URL: Place's negative cache is
			// process-wide, and must not poison other image tests.
			imageURL := (&url.URL{Scheme: "https", Host: "example.test", Path: t.TempDir() + "/status.png"}).String()
			fetcher := newFakePlaceFetcher()
			fetcher.err = errors.New("synthetic emoji download failure")
			ready := make(chan any, 1)
			status := peerstatus.Status{Emoji: ":" + tc.shortcode + ":", Text: "On call"}
			msg := MessageItem{TS: "1.0", UserID: "U_FIXTURE", UserName: "fixture", Text: "Message"}
			m := New([]MessageItem{msg}, "fixture")
			m.SetUserStatuses(map[string]peerstatus.Status{"U_FIXTURE": status})
			m.SetEmojiContext(EmojiContext{
				PlaceCtx: emojiutil.PlaceContext{
					Fetcher: fetcher, SendMsg: func(msg any) { ready <- msg },
				},
				Cells: 2, Customs: map[string]string{tc.shortcode: imageURL},
			})
			// The cold render starts the fetch; its failure notice says
			// Place has recorded it.
			RenderMessage(msg, 80, m.renderContext(), RenderOptions{})
			<-ready
			want := "fixture " + status.Glyph(time.Now())
			for _, phase := range []string{"after failure", "subsequent render"} {
				rendered := RenderMessage(msg, 80, m.renderContext(), RenderOptions{})
				if got := strings.TrimSpace(ansi.Strip(rendered.Header)); got != want {
					t.Errorf("%s: status header = %q, want existing glyph %q", phase, got, want)
				}
			}
		})
	}
}

// Without render options the suffix is the glyph, image mode or not.
func TestAuthorStatusSuffixNilOptionsPreservesGlyph(t *testing.T) {
	t.Setenv("TMUX", "")
	emojiutil.SetImageMode(true, 2)
	t.Cleanup(func() { emojiutil.SetImageMode(false, 2) })
	statuses := map[string]peerstatus.Status{
		"U_FIXTURE": {Emoji: ":pager:", Text: "On call"},
	}
	got := AuthorStatusSuffix(statuses, "U_FIXTURE", time.Now(), nil)
	if plain := ansi.Strip(got); plain != " 📟" {
		t.Fatalf("nil options status suffix = %q, want the existing pager glyph", plain)
	}
}
