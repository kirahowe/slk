package thread

import (
	"bytes"
	"fmt"
	stdimage "image"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	emojiutil "github.com/gammons/slk/internal/emoji"
	imgpkg "github.com/gammons/slk/internal/image"
	"github.com/gammons/slk/internal/ui/messages"
	"github.com/gammons/slk/internal/ui/peerstatus"
)

// The thread parent is drawn outside the reply cache, so its image emoji
// (its author's status among them) must be uploaded by the frame that
// draws it, with or without replies, and only once while the frame does
// not change.
func TestThreadView_ParentStatusImageUploads(t *testing.T) {
	t.Setenv("TMUX", "")
	emojiutil.SetImageMode(true, 2)
	t.Cleanup(func() { emojiutil.SetImageMode(false, 2) })
	for _, tc := range []struct {
		name, replyUser string
	}{
		{"no replies", ""},
		{"unrelated reply", "U_OTHER"},
		{"same status in reply", "U_PARENT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const imageURL = "https://example.test/parent-status.png"
			key := emojiutil.EmojiCacheKey(imageURL)
			kitty := imgpkg.NewKittyRenderer(imgpkg.NewRegistry())
			kitty.SetSource(key, stdimage.NewRGBA(stdimage.Rect(0, 0, 2, 2)))
			placed := kitty.RenderKey(key, stdimage.Pt(2, 1))
			fetcher := newFakePlaceFetcher()
			fetcher.setPrerendered(key, stdimage.Pt(2, 1), placed)
			parent := messages.MessageItem{
				TS: "1700000001.000000", UserID: "U_PARENT",
				UserName: "parent-author", Text: "Parent body",
			}
			var replies []messages.MessageItem
			if tc.replyUser != "" {
				replies = []messages.MessageItem{{
					TS: "1700000002.000000", UserID: tc.replyUser,
					UserName: "reply-author", Text: "Plain reply",
				}}
			}
			m := New()
			m.SetThread(parent, replies, "C_FIXTURE", parent.TS)
			m.SetUserStatuses(map[string]peerstatus.Status{
				parent.UserID: {Emoji: ":pager:", Text: "On call"},
			})
			m.SetEmojiContext(messages.EmojiContext{
				PlaceCtx: emojiutil.PlaceContext{Fetcher: fetcher}, Cells: 2,
				Customs: map[string]string{"pager": imageURL},
			})
			var uploads bytes.Buffer
			oldOutput := imgpkg.KittyOutput
			imgpkg.KittyOutput = imgpkg.SerializeOutput(&uploads)
			t.Cleanup(func() { imgpkg.KittyOutput = oldOutput })
			const width, height = 80, 24
			frame := m.View(height, width)
			canvas := lipgloss.NewCanvas(width, height)
			canvas.Compose(lipgloss.NewLayer(frame))
			parentCells := 0
			for y, line := range strings.Split(frame, "\n") {
				if !strings.Contains(ansi.Strip(line), parent.UserName) {
					continue
				}
				for x := range width {
					cell := canvas.CellAt(x, y)
					if cell == nil || !strings.HasPrefix(cell.Content, string(imgpkg.PlaceholderRune)) {
						continue
					}
					parentCells++
					if cell.Style.Fg == nil {
						t.Fatal("parent status placement lost its image ID")
					}
					r, g, b, _ := cell.Style.Fg.RGBA()
					if got := (r>>8)<<16 | (g>>8)<<8 | b>>8; got != placed.ID {
						t.Fatalf("parent status references image %d, want %d", got, placed.ID)
					}
				}
			}
			if parentCells != 2 {
				t.Fatalf("parent status has %d image cells, want both cells", parentCells)
			}
			marker := fmt.Sprintf("\x1b_Ga=T,f=100,t=d,i=%d,", placed.ID)
			if count := strings.Count(uploads.String(), marker); count != 1 {
				t.Errorf("parent status image %d uploaded %d times, want once before drawing", placed.ID, count)
			}
			uploads.Reset()
			_ = m.View(height, width)
			if uploads.Len() != 0 {
				t.Errorf("unchanged frame retransmitted an already uploaded status: %q", uploads.String())
			}
		})
	}
}
