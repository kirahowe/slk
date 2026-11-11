package messages

import (
	"bytes"
	"fmt"
	goimage "image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	emojiutil "github.com/gammons/slk/internal/emoji"
	imgpkg "github.com/gammons/slk/internal/image"
	"github.com/gammons/slk/internal/mention"
	"github.com/gammons/slk/internal/ui/messages/blockkit"
)

// A button label is plain text: its shortcodes become emoji, but its
// asterisks and mention syntax are shown as typed.
func TestRenderMessageActionLabelRendersEmoji(t *testing.T) {
	const label = ":white_check_mark: *Resolve* <@U1>"
	for _, tc := range []struct {
		name string
		msg  MessageItem
	}{
		{"actions", MessageItem{Blocks: []blockkit.Block{
			blockkit.ActionsBlock{Elements: []blockkit.ActionElement{{Kind: "button", Label: label}}},
		}}},
		{"accessory", MessageItem{Blocks: []blockkit.Block{
			blockkit.SectionBlock{Text: "Incident", Accessory: blockkit.LabelAccessory{Kind: "button", Label: label}},
		}}},
		{"legacy", MessageItem{LegacyAttachments: []blockkit.LegacyAttachment{{
			Actions: []blockkit.LegacyAction{{Type: "button", Text: label}},
		}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.msg.TS, tc.msg.UserName = "1.0", "alert-bot"
			plain := renderedFor(t, tc.msg, 100)
			if !strings.Contains(plain, "✅") || strings.Contains(plain, ":white_check_mark:") {
				t.Fatalf("button emoji did not render: %q", plain)
			}
			if !strings.Contains(plain, "*Resolve* <@U1>") {
				t.Fatalf("plain-text label was interpreted as mrkdwn: %q", plain)
			}
		})
	}
}

// A header is plain text too, and its emoji draw as the body's do.
func TestRenderMessageHeaderEmojiRendersLikeBody(t *testing.T) {
	msg := MessageItem{
		TS:       "1.0",
		UserName: "fixture-bot",
		Blocks: []blockkit.Block{
			blockkit.HeaderBlock{Text: ":white_check_mark: *Ready*"},
			blockkit.SectionBlock{Text: ":white_check_mark: Ready"},
		},
	}
	plain := renderedFor(t, msg, 100)
	if strings.Contains(plain, ":white_check_mark:") || strings.Count(plain, "✅") != 2 {
		t.Fatalf("header and body did not both expand the checkmark: %q", plain)
	}
	if !strings.Contains(plain, "*Ready*") {
		t.Fatalf("plain header text was interpreted as mrkdwn: %q", plain)
	}
}

// A base+VS16 emoji in a label is drawn as its bare base, as on every
// width-sensitive row (#301), so the row is as wide as lipgloss thinks.
func TestRenderMessageLabelEmojiUsesTheVS16Gate(t *testing.T) {
	msg := MessageItem{TS: "1.0", UserName: "bot", Blocks: []blockkit.Block{
		blockkit.HeaderBlock{Text: ":heart: header"},
		blockkit.ActionsBlock{Elements: []blockkit.ActionElement{{Kind: "button", Label: ":heart: button"}}},
	}}
	plain := renderedFor(t, msg, 100)
	if strings.Contains(plain, "❤️") || strings.Count(plain, "❤") != 2 {
		t.Fatalf("labels did not draw :heart: as the bare base: %q", plain)
	}
}

// fixtureEmoji puts a 2x1 kitty placement for url in a fake fetcher, so
// image-mode rendering places it without the network.
func fixtureEmoji(t *testing.T, url string) (*fakePlaceFetcher, imgpkg.Render) {
	t.Helper()
	key := emojiutil.EmojiCacheKey(url)
	src, err := png.Decode(bytes.NewReader(makeTestPNG(2, 2)))
	if err != nil {
		t.Fatal(err)
	}
	kitty := imgpkg.NewKittyRenderer(imgpkg.NewRegistry())
	kitty.SetSource(key, src)
	placed := kitty.RenderKey(key, goimage.Pt(2, 1))
	fetcher := newFakePlaceFetcher()
	fetcher.setPrerendered(key, goimage.Pt(2, 1), placed)
	return fetcher, placed
}

// imageCellIDs are the kitty image IDs of the placeholder cells in the
// first height rows of content, width columns wide.
func imageCellIDs(t *testing.T, content string, width, height int) []uint32 {
	t.Helper()
	canvas := lipgloss.NewCanvas(width, height)
	canvas.Compose(lipgloss.NewLayer(content))
	var ids []uint32
	for y := range height {
		for x := range width {
			cell := canvas.CellAt(x, y)
			if cell == nil || !strings.HasPrefix(cell.Content, string(imgpkg.PlaceholderRune)) {
				continue
			}
			if cell.Style.Fg == nil {
				t.Fatal("emoji placement lost its image ID")
			}
			r, g, b, _ := cell.Style.Fg.RGBA()
			ids = append(ids, (r>>8)<<16|(g>>8)<<8|b>>8)
		}
	}
	return ids
}

// A header cut to the pane keeps its emoji image whole: truncation counts
// display columns and keeps the placement's escapes.
func TestBlockkitHeaderImageTruncationPreservesTextAndImageID(t *testing.T) {
	t.Setenv("TMUX", "")
	emojiutil.SetImageMode(true, 2)
	t.Cleanup(func() { emojiutil.SetImageMode(false, 2) })
	const url = "https://example.test/header-truncation.png"
	fetcher, placed := fixtureEmoji(t, url)
	msg := MessageItem{TS: "1.0", UserName: "fixture", Blocks: []blockkit.Block{
		blockkit.HeaderBlock{Text: ":header_fixture: abcdefghijklmnopqrstuvwxyz"},
	}}
	m := New([]MessageItem{msg}, "fixture")
	m.SetEmojiContext(EmojiContext{
		PlaceCtx: emojiutil.PlaceContext{Fetcher: fetcher}, Cells: 2,
		Customs: map[string]string{"header_fixture": url},
	})
	const width = 12
	rendered := blockkit.Render(msg.Blocks, BlockkitContext(msg, m.renderContext(), nil), width)
	header := rendered.Lines[0]
	if plain := ansi.Strip(header); !strings.HasSuffix(plain, " abcdefgh…") {
		t.Errorf("truncated header lost visible text: %q", plain)
	}
	if got := lipgloss.Width(header); got > width {
		t.Errorf("header width = %d, exceeds %d", got, width)
	}
	ids := imageCellIDs(t, header, width, 1)
	if len(ids) != 2 {
		t.Fatalf("truncated header has %d emoji cells, want both cells", len(ids))
	}
	for _, id := range ids {
		if id != placed.ID {
			t.Errorf("truncated header references image %d, want %d", id, placed.ID)
		}
	}
}

// An image emoji in any Block Kit text, a header or button label among
// them, is uploaded with the message's block uploads, so its placeholder
// cells resolve.
func TestRenderMessageBlockEmojiUploadsReferencedImage(t *testing.T) {
	t.Setenv("TMUX", "")
	emojiutil.SetImageMode(true, 2)
	t.Cleanup(func() { emojiutil.SetImageMode(false, 2) })
	const url = "https://example.test/custom-emoji.png"
	for _, tc := range []struct {
		name string
		msg  MessageItem
	}{
		{"header", MessageItem{Blocks: []blockkit.Block{
			blockkit.HeaderBlock{Text: ":custom_fixture:"},
		}}},
		{"section", MessageItem{Blocks: []blockkit.Block{
			blockkit.SectionBlock{Text: ":custom_fixture:"},
		}}},
		{"accessory", MessageItem{Blocks: []blockkit.Block{
			blockkit.SectionBlock{Text: "Incident", Accessory: blockkit.LabelAccessory{Kind: "button", Label: ":custom_fixture: Resolve"}},
		}}},
		{"actions", MessageItem{Blocks: []blockkit.Block{
			blockkit.ActionsBlock{Elements: []blockkit.ActionElement{{Kind: "button", Label: ":custom_fixture: Resolve"}}},
		}}},
		{"legacy", MessageItem{LegacyAttachments: []blockkit.LegacyAttachment{{
			Actions: []blockkit.LegacyAction{{Type: "button", Text: ":custom_fixture: Resolve"}},
		}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fetcher, placed := fixtureEmoji(t, url)
			tc.msg.TS, tc.msg.UserName = "1.0", "alert-bot"
			m := New([]MessageItem{tc.msg}, "general")
			m.SetEmojiContext(EmojiContext{
				PlaceCtx: emojiutil.PlaceContext{Fetcher: fetcher},
				Cells:    2,
				Customs:  map[string]string{"custom_fixture": url},
			})
			rendered := RenderMessage(tc.msg, 100, m.renderContext(), RenderOptions{})
			ids := imageCellIDs(t, rendered.Content, 100, lipgloss.Height(rendered.Content))
			if len(ids) == 0 {
				t.Fatalf("emoji did not create an image placement: %q", rendered.Content)
			}
			for _, id := range ids {
				if id != placed.ID {
					t.Fatalf("emoji references image %d, want %d", id, placed.ID)
				}
			}
			var uploads bytes.Buffer
			for _, flush := range rendered.BlockFlushes {
				if err := flush(&uploads); err != nil {
					t.Fatal(err)
				}
			}
			if want := fmt.Sprintf("\x1b_Ga=T,f=100,t=d,i=%d,", placed.ID); !strings.Contains(uploads.String(), want) {
				t.Fatalf("block references image %d without uploading it: %q", placed.ID, uploads.String())
			}
		})
	}
}

// Text after an image emoji in a header or a control label keeps the
// label's colour. A kitty placement ends in a foreground reset, so
// without it the rest of the label takes the terminal's default
// foreground. Checked on every row a pane draws: unselected, selected
// and tinted for a direct mention.
func TestBlockkitPlainTextAfterImageEmojiKeepsForeground(t *testing.T) {
	t.Setenv("TMUX", "")
	emojiutil.SetImageMode(true, 2)
	t.Cleanup(func() { emojiutil.SetImageMode(false, 2) })
	const url = "https://example.test/plain-text-foreground.png"
	const label = "Ab :zz: Ready"
	action := func(kind string) []blockkit.Block {
		return []blockkit.Block{blockkit.ActionsBlock{Elements: []blockkit.ActionElement{{Kind: kind, Label: label}}}}
	}
	cases := []struct {
		name string
		msg  MessageItem
	}{
		{"header", MessageItem{Blocks: []blockkit.Block{blockkit.HeaderBlock{Text: label}}}},
		{"button", MessageItem{Blocks: action("button")}},
		{"workflow_button", MessageItem{Blocks: action("workflow_button")}},
		{"static_select", MessageItem{Blocks: action("static_select")}},
		{"multi_select", MessageItem{Blocks: action("multi_select")}},
		{"datepicker", MessageItem{Blocks: action("datepicker")}},
		{"timepicker", MessageItem{Blocks: action("timepicker")}},
		{"accessory", MessageItem{Blocks: []blockkit.Block{
			blockkit.SectionBlock{Text: "Incident", Accessory: blockkit.LabelAccessory{Kind: "button", Label: label}},
		}}},
		{"legacy button", MessageItem{LegacyAttachments: []blockkit.LegacyAttachment{{
			Actions: []blockkit.LegacyAction{{Type: "button", Text: label}},
		}}}},
		{"legacy select", MessageItem{LegacyAttachments: []blockkit.LegacyAttachment{{
			Actions: []blockkit.LegacyAction{{Type: "select", Text: label}},
		}}}},
	}
	const width = 60
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fetcher, _ := fixtureEmoji(t, url)
			tc.msg.TS, tc.msg.UserName = "1.0", "alert-bot"
			m := New([]MessageItem{tc.msg}, "general")
			m.SetEmojiContext(EmojiContext{
				PlaceCtx: emojiutil.PlaceContext{Fetcher: fetcher},
				Cells:    2,
				Customs:  map[string]string{"zz": url},
			})
			rendered := RenderMessage(tc.msg, width, m.renderContext(), RenderOptions{})
			s := NewWrapStyles(true)
			wrapped := WrapRows(tc.msg, rendered, width, s)
			rendered.Mention = mention.Direct
			tinted := WrapRows(tc.msg, rendered, width, s)
			for _, v := range []struct {
				name  string
				lines []string
			}{
				{"content", strings.Split(rendered.Content, "\n")},
				{"unselected", wrapped.Lines},
				{"selected", wrapped.Selected},
				{"mention", tinted.Lines},
			} {
				assertForegroundAcrossEmoji(t, v.name, strings.Join(v.lines, "\n"), width+1)
			}
		})
	}
}

// assertForegroundAcrossEmoji finds the row of content holding emoji
// placeholder cells and checks that every non-blank cell after them on
// that row has the foreground of the last non-blank cell before them.
func assertForegroundAcrossEmoji(t *testing.T, variant, content string, width int) {
	t.Helper()
	height := lipgloss.Height(content)
	canvas := lipgloss.NewCanvas(width, height)
	canvas.Compose(lipgloss.NewLayer(content))
	// cell is the content and foreground of the cell at x, y; a cell
	// with no content reads as blank.
	cell := func(x, y int) (string, color.Color) {
		c := canvas.CellAt(x, y)
		if c == nil {
			return "", nil
		}
		return c.Content, c.Style.Fg
	}
	isImage := func(content string) bool {
		return strings.HasPrefix(content, string(imgpkg.PlaceholderRune))
	}
	blank := func(content string) bool { return content == "" || content == " " }
	for y := range height {
		first, last := -1, -1
		for x := range width {
			if content, _ := cell(x, y); isImage(content) {
				if first < 0 {
					first = x
				}
				last = x
			}
		}
		if first < 0 {
			continue
		}
		var want color.Color
		for x := first - 1; x >= 0; x-- {
			if content, fg := cell(x, y); !blank(content) {
				want = fg
				break
			}
		}
		if want == nil {
			t.Fatalf("%s: no styled text before the emoji on row %d", variant, y)
		}
		checked := 0
		for x := last + 1; x < width; x++ {
			content, fg := cell(x, y)
			if blank(content) {
				continue
			}
			checked++
			if !sameColor(fg, want) {
				t.Errorf("%s: cell %d %q has foreground %v, want %v (the label's)", variant, x, content, fg, want)
			}
		}
		if checked == 0 {
			t.Fatalf("%s: no text after the emoji on row %d", variant, y)
		}
		return
	}
	t.Fatalf("%s: no emoji placeholder cells in %q", variant, content)
}

// sameColor reports whether a and b are the same colour; nil (the
// terminal's default) equals only nil.
func sameColor(a, b color.Color) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	ar, ag, ab, aa := a.RGBA()
	br, bg, bb, ba := b.RGBA()
	return ar == br && ag == bg && ab == bb && aa == ba
}
