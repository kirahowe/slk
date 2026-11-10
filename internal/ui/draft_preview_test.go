package ui

import (
	"bytes"
	"context"
	"fmt"
	stdimage "image"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"pgregory.net/rapid"

	"github.com/gammons/slk/internal/core"
	emojiutil "github.com/gammons/slk/internal/emoji"
	imgpkg "github.com/gammons/slk/internal/image"
	"github.com/gammons/slk/internal/proptest"
	"github.com/gammons/slk/internal/ui/mentionpicker"
	"github.com/gammons/slk/internal/ui/messages"
	"github.com/gammons/slk/internal/ui/wintree"
)

// richDraft is a draft with the formatting the preview exists for: a
// list with bold and a labelled link.
const richDraft = "Release plan:\n- ship **v0.28** on Tuesday\n- read [the notes](https://example.com/notes)"

// draftApp is a w x h App in T1 on C1 (three messages from alice, the
// current user U1), in insert mode in the channel box holding draft,
// its draft preview on.
func draftApp(t testing.TB, w, h int, draft string) *App {
	t.Helper()
	a := newTestApp(t, append(insertOpts(), withSize(w, h), withActiveTeam("T1"))...)
	a.SetCurrentUserID("U1")
	a.compose.SetDraftContext("T1", "C1", "")
	_ = a.beginInsertMode(false)
	a.compose.SetValue(draft)
	a.compose.SetPreview(true)
	return a
}

// frameRows is the frame's rows, escapes stripped.
func frameRows(a *App) []string {
	return strings.Split(ansi.Strip(a.View().Content), "\n")
}

// previewRows is the draft preview's rows in the frame, between the pane
// borders and trimmed: the title row and what follows it down to the
// blank spacer row above the compose box (or picker). Nil when the frame
// has no preview.
func previewRows(t *testing.T, a *App) []string {
	t.Helper()
	var out []string
	rows := frameRows(a)
	for i, row := range rows {
		if !strings.Contains(row, "── draft preview ") {
			continue
		}
		col := strings.Index(row, "── draft preview ")
		for _, r := range rows[i:] {
			// The body rows' invisible left border strips to "▌", so
			// a blank body row is not the blank spacer row.
			raw := strings.TrimRight(r[col:], "│┃ ")
			if strings.TrimSpace(raw) == "" {
				return out
			}
			out = append(out, strings.TrimSpace(strings.TrimPrefix(raw, "▌")))
		}
	}
	return out
}

func TestDraftPreviewShowsTheDraftAsSent(t *testing.T) {
	a := draftApp(t, 120, 40, richDraft)
	rows := strings.Join(previewRows(t, a), "\n")
	for _, want := range []string{"Release plan:", "• ship v0.28 on Tuesday", "• read the notes"} {
		if !strings.Contains(rows, want) {
			t.Errorf("preview lacks %q:\n%s", want, rows)
		}
	}
	for _, bad := range []string{"**", "](", "alice", "1:00 PM"} {
		if strings.Contains(rows, bad) {
			t.Errorf("preview has %q:\n%s", bad, rows)
		}
	}
}

// A line after a list stays on a row of its own, under the list.
func TestDraftPreviewLineAfterAList(t *testing.T) {
	draft := strings.Join([]string{"bug with rendering lists:", "", "- one", "- two", "- three", "", "and another line"}, "\n")
	rows := previewRows(t, draftApp(t, 120, 40, draft))
	three, after := -1, -1
	for i, r := range rows {
		switch r {
		case "• three":
			three = i
		case "and another line":
			after = i
		}
	}
	if three < 0 || after <= three {
		t.Fatalf("want \"• three\" and then \"and another line\" on rows of their own:\n%s", strings.Join(rows, "\n"))
	}
}

func TestDraftPreviewNotes(t *testing.T) {
	for _, tc := range []struct {
		name, draft, want string
		attach, edit      bool
	}{
		{name: "empty", draft: "", want: "nothing to preview"},
		{name: "whitespace", draft: "  \n\n ", want: "nothing to preview"},
		{name: "slash command", draft: "/remind me", want: "slash command, not a message"},
		{name: "caption", draft: "**x**", attach: true, want: "caption is sent as typed"},
		{name: "edit starting with a slash", draft: "/not a command", edit: true, want: "/not a command"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := draftApp(t, 120, 40, "")
			if tc.edit {
				_ = a.enterEdit("C1", "1.0", PanelMessages, tc.draft)
				a.compose.SetPreview(true)
			}
			a.compose.SetValue(tc.draft)
			if tc.attach {
				a.compose.AddAttachment(core.PendingAttachment{Filename: "notes.txt", Path: "/tmp/notes.txt", Size: 5})
			}
			rows := previewRows(t, a)
			if len(rows) != 2 || rows[1] != tc.want {
				t.Fatalf("preview rows %q, want the title and %q", rows, tc.want)
			}
		})
	}
}

// A mention is drawn with the pane's name for the user, and redrawn
// when the name arrives while the draft sits unchanged.
func TestDraftPreviewMentionsUseThePanesNames(t *testing.T) {
	a := draftApp(t, 120, 40, "hi @bob")
	a.compose.SetUsers([]mentionpicker.User{{ID: "U2", DisplayName: "bob"}})
	before := strings.Join(previewRows(t, a), "\n")
	if strings.Contains(before, "Bob Smith") {
		t.Fatalf("precondition: the pane already names U2:\n%s", before)
	}
	a.messagepane.SetUserNames(map[string]string{"U2": "Bob Smith"})
	if after := strings.Join(previewRows(t, a), "\n"); !strings.Contains(after, "@Bob Smith") {
		t.Fatalf("preview after the name arrived:\n%s\nwant @Bob Smith", after)
	}
}

// numberedDraft is n lines, "line 1" to "line n".
func numberedDraft(n int) string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %d", i+1)
	}
	return strings.Join(lines, "\n")
}

// Past its cap the preview shows the draft's end under a "…" row, and
// the cap is a third of the pane's inner height.
func TestDraftPreviewCapShowsTheEnd(t *testing.T) {
	a := draftApp(t, 120, 80, numberedDraft(40))
	rows := previewRows(t, a)
	inner := a.computeFrame().ContentHeight - 2
	if want := inner / 3; len(rows) != want {
		t.Fatalf("preview has %d rows, want a third of the inner height %d: %d", len(rows), inner, want)
	}
	if rows[1] != "…" || rows[len(rows)-1] != "line 40" {
		t.Fatalf("preview rows %q, want … then the draft's end", rows)
	}
}

// On a short terminal the preview takes only what the message list can
// spare above its 3-row minimum, or nothing, and never a title alone.
func TestDraftPreviewYieldsToTheMessageList(t *testing.T) {
	for _, h := range []int{10, 12, 14, 16, 20, 24} {
		for _, lines := range []int{1, 3, 40} {
			a := draftApp(t, 80, h, numberedDraft(lines))
			rows := previewRows(t, a)
			if len(rows) == 0 {
				continue
			}
			if len(rows) == 1 {
				t.Fatalf("h=%d lines=%d: preview is a title with nothing under it", h, lines)
			}
			if got := a.layout.PageHeight(PanelMessages); got < 3 {
				t.Fatalf("h=%d lines=%d: the preview left the message list %d rows, want at least 3", h, lines, got)
			}
		}
	}
	if rows := previewRows(t, draftApp(t, 80, 16, "hi")); len(rows) != 2 {
		t.Fatalf("80x16 one-line draft: preview rows %q, want the title and the line", rows)
	}
}

// The window at the bottom of the list stays there as the preview
// opens and closes: the newest message stays on screen.
func TestDraftPreviewKeepsTheListAtTheBottom(t *testing.T) {
	a := newTestApp(t, withSize(120, 30), withActiveChannel("C1"), withActiveTeam("T1"), withMessages(testMessageItems(40)...))
	a.compose.SetDraftContext("T1", "C1", "")
	_ = a.beginInsertMode(false)
	a.compose.SetValue(numberedDraft(10))
	newest := func() bool { return strings.Contains(strings.Join(frameRows(a), "\n"), "msg-40") }
	if !newest() {
		t.Fatal("precondition: the newest message is not on screen")
	}
	send(a, ctrlR)
	if !newest() {
		t.Fatal("opening the preview scrolled the newest message off screen")
	}
	send(a, ctrlR)
	if !newest() {
		t.Fatal("closing the preview scrolled the newest message off screen")
	}
}

func TestDraftPreviewFollowsTheTheme(t *testing.T) {
	a := draftApp(t, 120, 40, richDraft)
	before := a.View().Content
	a.applyTheme("dracula")
	t.Cleanup(func() { a.applyTheme("dark") })
	after := a.View().Content
	pick := func(frame string) string {
		for _, row := range strings.Split(frame, "\n") {
			if strings.Contains(ansi.Strip(row), "ship v0.28") {
				return row
			}
		}
		return ""
	}
	if pick(before) == "" || pick(before) == pick(after) {
		t.Fatal("the preview row did not change with the theme")
	}
}

// A preview whose box is not drawn (another view) draws nothing, and a
// closed preview keeps nothing cached.
func TestClosedDraftPreviewHoldsNothing(t *testing.T) {
	a := draftApp(t, 120, 40, richDraft)
	_ = a.View()
	if !a.renderCache.draftMsg.ok {
		t.Fatal("precondition: the drawn preview is not cached")
	}
	a.compose.SetPreview(false)
	_ = a.View()
	if c := a.renderCache.draftMsg; c.ok || c.body.Rows != nil || c.body.Uploads != nil {
		t.Fatalf("a closed preview kept its render: %+v", c.key)
	}
}

func TestDraftPreviewAboveThePicker(t *testing.T) {
	a := draftApp(t, 120, 40, "")
	a.compose.SetUsers([]mentionpicker.User{{ID: "U2", DisplayName: "bob"}})
	send(a, keyPress('@'))
	if !a.compose.IsMentionActive() {
		t.Fatal("precondition: @ did not open the mention picker")
	}
	rows := frameRows(a)
	title, picker := -1, -1
	for i, row := range rows {
		if strings.Contains(row, "── draft preview ") {
			title = i
		}
		if picker < 0 && strings.Contains(row, "bob") && title >= 0 {
			picker = i
		}
	}
	if title < 0 || picker < 0 || picker < title {
		t.Fatalf("draft preview row %d, picker row %d: want the preview above the picker", title, picker)
	}
}

// In the thread pane the preview is drawn over the reply box, from the
// thread box's own draft and state.
func TestDraftPreviewInTheThread(t *testing.T) {
	a := draftApp(t, 160, 40, "")
	a.compose.SetPreview(false)
	focusThreadPanel(t, a)
	_ = a.beginInsertMode(true)
	a.threadCompose.SetValue(richDraft)
	a.threadCompose.SetPreview(true)
	rows := strings.Join(previewRows(t, a), "\n")
	if !strings.Contains(rows, "• ship v0.28 on Tuesday") {
		t.Fatalf("thread preview:\n%s", rows)
	}
	if strings.Count(strings.Join(frameRows(a), "\n"), "── draft preview ") != 1 {
		t.Fatal("want one draft preview on screen, the thread box's")
	}
}

// In the stacked layout the thread is drawn alone, and its box's
// preview with it; Esc back to the channel puts it away.
func TestDraftPreviewInTheStackedThread(t *testing.T) {
	a := stackedApp(t, 110)
	updateAndRender(t, a, keyCode(tea.KeyEnter)) // open the selected message's thread
	assertFront(t, a, false, true)
	send(a, keyPress('i'))
	for _, r := range "**hi**" {
		send(a, keyPress(r))
	}
	send(a, ctrlR)
	if rows := previewRows(t, a); len(rows) != 2 || rows[1] != "hi" {
		t.Fatalf("stacked thread preview rows %q, want the title and hi", rows)
	}
	assertFront(t, a, false, true)
	send(a, keyCode(tea.KeyEscape))
	if rows := previewRows(t, a); rows != nil {
		t.Fatalf("preview still drawn after Esc: %q", rows)
	}
}

// With the window split, only the focused window has a compose box, and
// the preview is drawn there alone, the region's rows keeping their
// widths.
func TestDraftPreviewInASplitWindow(t *testing.T) {
	a := draftApp(t, 160, 40, richDraft)
	a.SetMode(ModeNormal)
	if cmd := a.splitWindow(wintree.SplitSideBySide); cmd != nil {
		t.Fatal("precondition: the split was refused")
	}
	_ = a.beginInsertMode(false)
	a.compose.SetValue(richDraft)
	a.compose.SetPreview(true)
	on := strings.Split(a.View().Content, "\n")
	if n := strings.Count(ansi.Strip(strings.Join(on, "\n")), "── draft preview "); n != 1 {
		t.Fatalf("%d draft previews on screen, want 1", n)
	}
	a.compose.SetPreview(false)
	off := strings.Split(a.View().Content, "\n")
	for i := range on {
		if ansi.StringWidth(on[i]) != ansi.StringWidth(off[i]) {
			t.Fatalf("row %d changed width with the preview: %q", i, ansi.Strip(on[i]))
		}
	}
}

// The preview's emoji images upload as the frame is drawn, as a row's
// do: in image mode a custom emoji is placed and its upload written.
func TestDraftPreviewUploadsItsEmojiImages(t *testing.T) {
	t.Setenv("TMUX", "")
	emojiutil.SetImageMode(true, 2)
	t.Cleanup(func() { emojiutil.SetImageMode(false, 2) })
	const url = "https://example.test/draft-parrot.png"
	key := emojiutil.EmojiCacheKey(url)
	kitty := imgpkg.NewKittyRenderer(imgpkg.NewRegistry())
	kitty.SetSource(key, stdimage.NewRGBA(stdimage.Rect(0, 0, 2, 2)))
	fetcher := &draftPlaceFetcher{key: key, placed: kitty.RenderKey(key, stdimage.Pt(2, 1))}

	a := draftApp(t, 120, 40, "party :parrot:")
	a.SetEmojiContext(messages.EmojiContext{
		PlaceCtx: emojiutil.PlaceContext{Fetcher: fetcher}, Cells: 2,
		Customs: map[string]string{"parrot": url},
	})
	var uploads bytes.Buffer
	old := imgpkg.KittyOutput
	imgpkg.KittyOutput = imgpkg.SerializeOutput(&uploads)
	t.Cleanup(func() { imgpkg.KittyOutput = old })

	rows := strings.Join(previewRows(t, a), "\n")
	if strings.Contains(rows, ":parrot:") {
		t.Fatalf("the emoji was drawn as text, not placed:\n%s", rows)
	}
	if uploads.Len() == 0 {
		t.Fatal("drawing the preview wrote no kitty upload for its emoji")
	}
}

// draftPlaceFetcher has one emoji image already rendered.
type draftPlaceFetcher struct {
	mu     sync.Mutex
	key    string
	placed imgpkg.Render
}

func (f *draftPlaceFetcher) Prerendered(key string, _ stdimage.Point, _ imgpkg.Protocol) (imgpkg.Render, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.placed, key == f.key
}

func (f *draftPlaceFetcher) Fetch(context.Context, imgpkg.FetchRequest) (imgpkg.FetchResult, error) {
	return imgpkg.FetchResult{}, nil
}

// draftUsers are the people the property drafts mention by name.
var draftUsers = []mentionpicker.User{{ID: "U2", DisplayName: "bob"}, {ID: "U3", DisplayName: "carol.w"}}

// draftText is a draft as a person types it: ComposerText with typed
// mentions of draftUsers and #general among its words.
func draftText(t *rapid.T) string {
	word := rapid.OneOf(proptest.Word(), rapid.SampledFrom([]string{"@bob", "@carol.w", "@here", "#general", "@nobody"}))
	return proptest.NoVS16(proptest.ComposerTextWith(word, []string{"U2", "U9"}, []string{"C1"}).Draw(t, "draft"))
}

// The preview's body is the placeholder row Enter makes for the same
// draft, drawn by the same pane at the same width: for the channel box
// (beginSend) and the thread box (beginReply).
func TestDraftPreviewIsThePlaceholderEnterMakes(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		thread := rapid.Bool().Draw(rt, "thread")
		draft := draftText(rt)
		a := draftApp(t, rapid.IntRange(100, 200).Draw(rt, "w"), 40, "")
		a.messagepane.SetUserNames(map[string]string{"U1": "alice", "U2": "bob", "U3": "Carol W"})
		c, pane := &a.compose, contentPane(a.messagepane)
		box := draftPreviewBox{compose: c, pane: pane, panel: PanelMessages, cache: &a.renderCache.draftMsg}
		if thread {
			a.compose.SetPreview(false)
			focusThreadPanel(t, a)
			a.threadPanel.SetUserNames(map[string]string{"U1": "alice", "U2": "bob", "U3": "Carol W"})
			_ = a.beginInsertMode(true)
			c, pane = &a.threadCompose, a.threadPanel
			box = draftPreviewBox{compose: c, pane: pane, panel: PanelThread, threadTS: a.threadPanel.ThreadTS(), cache: &a.renderCache.draftThread}
		}
		c.SetUsers(draftUsers)
		c.SetValue(draft)
		c.SetPreview(true)
		width := a.computeFrame().MsgWidth - 2
		if thread {
			width = a.computeFrame().ThreadWidth - 2
		}
		got := a.draftBody(box, width).Rows

		_ = pressEnter(a)
		rows := a.messagepane.Messages()
		if thread {
			rows = a.threadPanel.Replies()
		}
		placeholder := rows[len(rows)-1]
		if !messages.IsLocalTS(placeholder.TS) {
			rt.Fatalf("Enter made no placeholder for %q", draft)
		}
		want := pane.RenderDraft(placeholder, width).Rows
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			rt.Fatalf("draft %q\npreview:     %q\nplaceholder: %q", draft, got, want)
		}
	})
}

// Every frame row is as wide with the preview open as with it closed,
// for drafts with wide and combining runes: the preview rows are
// exactly the pane's width and the border stays whole.
func TestDraftPreviewRowsAreThePanesWidth(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		a := draftApp(t, rapid.IntRange(60, 200).Draw(rt, "w"), rapid.IntRange(16, 50).Draw(rt, "h"), draftText(rt))
		on := strings.Split(a.View().Content, "\n")
		a.compose.SetPreview(false)
		off := strings.Split(a.View().Content, "\n")
		if len(on) != len(off) {
			rt.Fatalf("%d rows with the preview, %d without", len(on), len(off))
		}
		for i := range on {
			if w, want := ansi.StringWidth(on[i]), ansi.StringWidth(off[i]); w != want {
				rt.Fatalf("row %d is %d cells with the preview, %d without:\n%s", i, w, want, ansi.Strip(on[i]))
			}
		}
	})
}

// Typing with the preview open renders the draft once per change;
// a frame with nothing new renders nothing.
func TestDraftPreviewRendersOncePerChange(t *testing.T) {
	a := draftApp(t, 120, 40, richDraft)
	_ = a.View()
	first := a.renderCache.draftMsg.body.Rows
	_ = a.View()
	if again := a.renderCache.draftMsg.body.Rows; &again[0] != &first[0] {
		t.Fatal("an unchanged frame rendered the draft again")
	}
	send(a, keyPress('!'))
	_ = a.View()
	if again := a.renderCache.draftMsg.body.Rows; &again[0] == &first[0] {
		t.Fatal("a typed character did not render the draft again")
	}
}

func BenchmarkDraftPreview(b *testing.B) {
	for _, on := range []bool{true, false} {
		b.Run(map[bool]string{true: "on", false: "off"}[on], func(b *testing.B) {
			a := draftApp(b, 160, 50, strings.Repeat("- ship **v0.28** on Tuesday with @person.42 and [the notes](https://example.com/x)\n", 25))
			users := make([]mentionpicker.User, 20000)
			for i := range users {
				users[i] = mentionpicker.User{ID: fmt.Sprintf("U%05d", i), DisplayName: fmt.Sprintf("person.%d", i)}
			}
			a.compose.SetUsers(users)
			a.compose.SetPreview(on)
			for b.Loop() {
				send(a, keyPress('x'))
				_ = a.View()
			}
		})
	}
}
