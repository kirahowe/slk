package messages

import (
	"context"
	goimage "image"
	"strings"
	"testing"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	emojiutil "github.com/gammons/slk/internal/emoji"
	imgpkg "github.com/gammons/slk/internal/image"
	"github.com/gammons/slk/internal/mention"
	"github.com/gammons/slk/internal/proptest"
	"github.com/gammons/slk/internal/ui/peerstatus"
	"github.com/gammons/slk/internal/ui/styles"
	"pgregory.net/rapid"
)

// warmPlaceFetcher has every emoji image already rendered (the same
// placement for any key), so image mode never starts a fetch.
type warmPlaceFetcher struct{ placed imgpkg.Render }

func (f warmPlaceFetcher) Prerendered(string, goimage.Point, imgpkg.Protocol) (imgpkg.Render, bool) {
	return f.placed, true
}

func (f warmPlaceFetcher) Fetch(context.Context, imgpkg.FetchRequest) (imgpkg.FetchResult, error) {
	return imgpkg.FetchResult{}, nil
}

// propStatus is an author's status: none, a huddle, or a custom status
// emoji with a glyph (base+VS16 among them), a skin tone, a workspace
// emoji, or one with no glyph.
func propStatus() *rapid.Generator[peerstatus.Status] {
	return rapid.Custom(func(t *rapid.T) peerstatus.Status {
		switch rapid.IntRange(0, 2).Draw(t, "statusKind") {
		case 0:
			return peerstatus.Status{}
		case 1:
			return peerstatus.Status{Huddle: peerstatus.HuddleActive}
		}
		return peerstatus.Status{
			Emoji: rapid.SampledFrom([]string{":pager:", ":heart:", ":wave::skin-tone-3:", ":team_fixture:", ":man-woman-girl:"}).Draw(t, "statusEmoji"),
			Text:  "busy",
		}
	})
}

// propUserName is a display name: a short one, or words of plain text
// (wide and combining runes included) long enough to wrap the header.
func propUserName() *rapid.Generator[string] {
	long := rapid.Custom(func(t *rapid.T) string {
		words := rapid.SliceOfN(proptest.Word(), 1, 8).Draw(t, "words")
		return proptest.NoVS16(strings.Join(words, " "))
	})
	return rapid.OneOf(rapid.SampledFrom([]string{"kira", "dave", "a-much-longer-display-name"}), long)
}

// propMessage is a message from one of three users, about any Slack
// text, with optional reactions, replies and edit mark.
func propMessage(self string) *rapid.Generator[MessageItem] {
	users := []string{self, "U0OTHER001", "U0OTHER002"}
	return rapid.Custom(func(t *rapid.T) MessageItem {
		author := rapid.SampledFrom(users).Draw(t, "author")
		msg := MessageItem{
			TS:       proptest.TS().Draw(t, "ts"),
			UserID:   author,
			UserName: propUserName().Draw(t, "name"),
			// KNOWN: see wrapInput; an ASCII byte before VS16 wraps a
			// cell too wide.
			Text:       stripASCIIVS16(proptest.SlackText(users, []string{"C0GENERAL1"}).Draw(t, "text")),
			Timestamp:  rapid.SampledFrom([]string{"3:04 PM", "15:04"}).Draw(t, "stamp"),
			ReplyCount: rapid.IntRange(0, 3).Draw(t, "replies"),
			IsEdited:   rapid.Bool().Draw(t, "edited"),
		}
		for range rapid.IntRange(0, 3).Draw(t, "reactions") {
			ApplyReaction(&msg, rapid.SampledFrom(propEmojis).Draw(t, "emoji"), rapid.SampledFrom(users).Draw(t, "reactor"), self, false)
		}
		return msg
	})
}

// RenderMessage then WrapRows, with the theme pinned, gives three
// parallel variants of one message: every unselected and selected row
// is exactly the pane width, the variants and the plain mirror have the
// same height, the plain mirror never holds the border glyph, and the
// author header, however long the name, is wrapped so that each of its
// lines fits HeaderBudget and an avatar's two rows stay the first two
// rows, one above the other. The
// mention bar is on the border exactly when the message mentions the
// current user (Mention, which must agree with mention.Classify, and is
// never set on the user's own message).
func TestRenderAndWrapRows(t *testing.T) {
	applyDarkTheme(t)
	t.Setenv("TMUX", "")
	t.Cleanup(func() { emojiutil.SetImageMode(false, 2) })
	_, placed := fixtureEmoji(t, "https://emoji.example/any.png")
	warm := warmPlaceFetcher{placed: placed}
	const self = "U0SELF0001"
	bar := strings.TrimSuffix(strings.TrimPrefix(fgANSIFor(styles.MentionBar), "\x1b["), "m")
	rapid.Check(t, func(t *rapid.T) {
		msg := propMessage(self).Draw(t, "msg")
		avatar := rapid.SampledFrom([]string{"", "TOP!\nLOW!"}).Draw(t, "avatar")
		// RenderMessage floors the content width at 20 (render_message.go),
		// so below 24 columns, or 29 with the avatar's five, rows are
		// wider than the pane by design.
		minWidth := 24
		if avatar != "" {
			minWidth = 29
		}
		width := rapid.IntRange(minWidth, 100).Draw(t, "width")
		m := New([]MessageItem{msg}, "general")
		m.SetCurrentUser(self)
		m.SetUserNames(map[string]string{self: "kira", "U0OTHER001": "dave"})
		// The author's status emoji is in the header the budget bounds,
		// drawn as an image in image mode.
		status := propStatus().Draw(t, "status")
		m.SetUserStatuses(map[string]peerstatus.Status{msg.UserID: status})
		imageMode := rapid.Bool().Draw(t, "imageMode")
		emojiutil.SetImageMode(imageMode, 2)
		if imageMode {
			m.SetEmojiContext(EmojiContext{
				PlaceCtx: emojiutil.PlaceContext{Fetcher: warm}, Cells: 2,
				Customs: map[string]string{"team_fixture": "https://emoji.example/team.png"},
			})
		}
		opt := RenderOptions{
			ReplyAffordance: rapid.Bool().Draw(t, "affordance"),
			EditedMark:      rapid.Bool().Draw(t, "editedMark"),
			BroadcastLabel:  rapid.Bool().Draw(t, "broadcastLabel"),
			IsSelected:      rapid.Bool().Draw(t, "selected"),
			Avatar:          avatar,
		}
		r := RenderMessage(msg, width, m.renderContext(), opt)
		w := WrapRows(msg, r, width, NewWrapStyles(rapid.Bool().Draw(t, "focused")))

		wantKind := mention.None
		if msg.UserID != self {
			wantKind = mention.Classify(msg.Text, self)
		}
		if r.Mention != wantKind {
			t.Fatalf("Mention = %v, want %v for %q by %s", r.Mention, wantKind, msg.Text, msg.UserID)
		}
		for i, line := range strings.Split(r.Header, "\n") {
			if got := lipgloss.Width(line); got > r.HeaderBudget {
				t.Fatalf("header line %d is %d wide, over the budget %d: %q", i, got, r.HeaderBudget, line)
			}
			// Every placement is two cells; a wrap must not split one.
			if n := strings.Count(line, string(imgpkg.PlaceholderRune)); n%2 != 0 {
				t.Fatalf("header line %d holds %d image cells, splitting an image: %q", i, n, line)
			}
		}
		if imageMode && status.Glyph(time.Now()) != "" && !strings.ContainsRune(r.Header, imgpkg.PlaceholderRune) {
			t.Fatalf("image mode, yet the status emoji is not an image: %q", r.Header)
		}
		if opt.Avatar != "" {
			top, low, _ := strings.Cut(opt.Avatar, "\n")
			for _, rows := range [][]string{w.Lines, w.Selected} {
				if len(rows) < 2 {
					t.Fatalf("avatar needs two rows, message has %d", len(rows))
				}
				at := strings.Index(ansi.Strip(rows[0]), top)
				if at < 0 || strings.Index(ansi.Strip(rows[1]), low) != at {
					t.Fatalf("avatar rows split or shifted:\n%q\n%q", rows[0], rows[1])
				}
			}
		}
		if len(w.Lines) != len(w.Selected) || len(w.Lines) != len(w.Plain) {
			t.Fatalf("heights differ: lines %d, selected %d, plain %d", len(w.Lines), len(w.Selected), len(w.Plain))
		}
		for i := range w.Lines {
			if got := lipgloss.Width(w.Lines[i]); got != width {
				t.Fatalf("row %d is %d wide, want %d: %q", i, got, width, w.Lines[i])
			}
			if got := lipgloss.Width(w.Selected[i]); got != width {
				t.Fatalf("selected row %d is %d wide, want %d: %q", i, got, width, w.Selected[i])
			}
			if strings.Contains(w.Plain[i].Text, "▌") {
				t.Fatalf("plain row %d holds the border: %q", i, w.Plain[i].Text)
			}
			border, _, _ := strings.Cut(w.Lines[i], "▌")
			if got := strings.Contains(border, bar); got != (r.Mention != mention.None) {
				t.Fatalf("row %d: mention bar %v for Mention %v: %q", i, got, r.Mention, w.Lines[i])
			}
		}
	})
}
