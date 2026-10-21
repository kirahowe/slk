package ui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/gammons/slk/internal/canvas"
	"github.com/gammons/slk/internal/core"
	"github.com/gammons/slk/internal/ui/messages"
)

// canvasFake answers the canvas viewer's requests from fixed data and
// records what it was asked.
type canvasFake struct {
	ids      map[string][]string // channel -> file IDs
	canvases map[string]core.Canvas
	listed   []string // "team/channel"
	fetched  []string // "team/file"
}

func (f *canvasFake) service() core.CanvasService {
	return core.NewCanvasService(core.CanvasServiceFuncs{
		ConversationCanvases: func(_ context.Context, teamID, channelID string) ([]string, error) {
			f.listed = append(f.listed, teamID+"/"+channelID)
			return f.ids[channelID], nil
		},
		Canvas: func(_ context.Context, teamID, fileID string) (core.Canvas, error) {
			f.fetched = append(f.fetched, teamID+"/"+fileID)
			c, ok := f.canvases[fileID]
			if !ok {
				return core.Canvas{}, errors.New("file_not_found")
			}
			return c, nil
		},
	})
}

func canvasDoc(t *testing.T, html string) canvas.Document {
	t.Helper()
	doc, err := canvas.Parse(`<div class="quip-canvas-content">` + html + `</div>`)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

// canvasApp is an App on D1 in T1, whose canvas service holds a 1:1
// notes canvas (F1) on D1 and a second canvas (F2), and no canvas on C2.
func canvasApp(t *testing.T) (*App, *canvasFake) {
	t.Helper()
	a := newTestApp(t, append(normalOpts(), withActiveTeam("T1"), withActiveChannel("D1"))...)
	a.userNames = map[string]string{"U1": "seth"}
	fake := &canvasFake{
		ids: map[string][]string{"D1": {"F1", "F2"}},
		canvases: map[string]core.Canvas{
			"F1": {FileID: "F1", Title: "Seth <> Kira", Permalink: "https://x.slack.com/docs/T1/F1",
				Doc: canvasDoc(t, `<h1>Seth &lt;&gt; Kira</h1><p class="line">Topics with <control><a>@U1</a></control></p>`+
					`<div data-section-style='5'><ul><li>promo packet</li></ul></div>`)},
			"F2": {FileID: "F2", Title: "Second", Doc: canvasDoc(t, `<p class="line">other canvas</p>`)},
		},
	}
	a.SetCanvasService(fake.service())
	return a, fake
}

// settle runs cmd and feeds every message it produces back through
// Update, until nothing more comes, and returns the messages seen.
func settle(t *testing.T, a *App, cmd tea.Cmd) []tea.Msg {
	t.Helper()
	var seen []tea.Msg
	queue := []tea.Cmd{cmd}
	for rounds := 0; len(queue) > 0; rounds++ {
		if rounds > 20 {
			t.Fatal("commands did not settle")
		}
		var next []tea.Cmd
		for _, c := range queue {
			for _, m := range drainBatch(c) {
				if m == nil {
					continue
				}
				seen = append(seen, m)
				_, more := a.Update(m)
				if more != nil {
					next = append(next, more)
				}
			}
		}
		queue = next
	}
	return seen
}

func pressCanvas(t *testing.T, a *App, k tea.KeyMsg) []tea.Msg {
	t.Helper()
	_, cmd := a.Update(k)
	return settle(t, a, cmd)
}

func canvasScreen(a *App) string { return ansi.Strip(a.View().Content) }

func TestCanvasKeyOpensTheConversationsCanvas(t *testing.T) {
	a, fake := canvasApp(t)
	pressCanvas(t, a, keyPress('c'))

	if a.mode != ModeCanvas || !a.canvasView.IsVisible() {
		t.Fatalf("mode = %v, visible = %v", a.mode, a.canvasView.IsVisible())
	}
	if strings.Join(fake.listed, ",") != "T1/D1" || strings.Join(fake.fetched, ",") != "T1/F1" {
		t.Fatalf("listed %v, fetched %v", fake.listed, fake.fetched)
	}
	screen := canvasScreen(a)
	for _, want := range []string{"Seth <> Kira  (1 of 2)", "Topics with @seth", "• promo packet", "tab next canvas"} {
		if !strings.Contains(screen, want) {
			t.Errorf("screen lacks %q:\n%s", want, screen)
		}
	}
}

func TestCanvasCommandOpensTheCanvas(t *testing.T) {
	a, fake := canvasApp(t)
	settle(t, a, cmdCanvas(a, commandArgs{}))
	if !a.canvasView.IsVisible() || len(fake.fetched) != 1 {
		t.Fatalf("visible = %v, fetched %v", a.canvasView.IsVisible(), fake.fetched)
	}
}

func TestCanvasKeyWithNoCanvasToasts(t *testing.T) {
	a, _ := canvasApp(t)
	a.activeChannelID = "C2"
	_, cmd := a.Update(keyPress('c'))
	// Apply the listing by hand and look before the toast's clear tick
	// runs (ticks fire at once in tests).
	for _, m := range drainBatch(cmd) {
		a.Update(m)
	}
	if a.canvasView.IsVisible() || a.mode != ModeNormal {
		t.Fatalf("viewer left open: mode %v", a.mode)
	}
	if !strings.Contains(statusbarText(a), noCanvasToast) {
		t.Fatalf("no %q toast; status bar %q", noCanvasToast, statusbarText(a))
	}
}

func TestCanvasTabCyclesAndEscCloses(t *testing.T) {
	a, fake := canvasApp(t)
	pressCanvas(t, a, keyPress('c'))
	pressCanvas(t, a, keyCode(tea.KeyTab))
	if got := fake.fetched[len(fake.fetched)-1]; got != "T1/F2" {
		t.Fatalf("tab fetched %s, want T1/F2", got)
	}
	if !strings.Contains(canvasScreen(a), "other canvas") {
		t.Fatalf("second canvas not shown:\n%s", canvasScreen(a))
	}
	pressCanvas(t, a, keyCode(tea.KeyEscape))
	if a.canvasView.IsVisible() || a.mode != ModeNormal {
		t.Fatalf("esc: visible %v, mode %v", a.canvasView.IsVisible(), a.mode)
	}
}

// A canvas that arrives after the user moved on to another one is
// dropped, not drawn over it.
func TestCanvasStaleResultIsDropped(t *testing.T) {
	a, _ := canvasApp(t)
	req := a.canvasView.OpenFile("T1", "F1")
	next := a.canvasView.OpenFile("T1", "F2")
	a.Update(CanvasLoadedMsg{Req: req, Canvas: core.Canvas{Title: "Stale", Doc: canvasDoc(t, "<p>stale</p>")}})
	if strings.Contains(canvasScreen(a), "stale") {
		t.Fatal("stale canvas drawn")
	}
	a.Update(CanvasLoadedMsg{Req: next, Canvas: core.Canvas{Title: "Fresh", Doc: canvasDoc(t, "<p>fresh</p>")}})
	if !strings.Contains(canvasScreen(a), "fresh") {
		t.Fatalf("current canvas not drawn:\n%s", canvasScreen(a))
	}
}

func TestCanvasFailureShowsErrorAndRetries(t *testing.T) {
	a, fake := canvasApp(t)
	delete(fake.canvases, "F1")
	pressCanvas(t, a, keyPress('c'))
	if !strings.Contains(canvasScreen(a), "Couldn't load the canvas: file_not_found") {
		t.Fatalf("no error shown:\n%s", canvasScreen(a))
	}
	fake.canvases["F1"] = core.Canvas{Title: "Back", Doc: canvasDoc(t, "<p>recovered</p>")}
	pressCanvas(t, a, keyPress('r'))
	if !strings.Contains(canvasScreen(a), "recovered") {
		t.Fatalf("retry did not load:\n%s", canvasScreen(a))
	}
}

func TestCanvasOOpensPermalink(t *testing.T) {
	a, _ := canvasApp(t)
	var opened []string
	a.setDesktopForTest(func(d *core.DesktopServiceFuncs) {
		d.Open = func(target string) error { opened = append(opened, target); return nil }
	})
	pressCanvas(t, a, keyPress('c'))
	pressCanvas(t, a, keyPress('o'))
	if strings.Join(opened, ",") != "https://x.slack.com/docs/T1/F1" {
		t.Fatalf("opened %v", opened)
	}
}

func TestCanvasScrollKeysMoveTheText(t *testing.T) {
	a, fake := canvasApp(t)
	var lines strings.Builder
	for i := range 100 {
		lines.WriteString(`<p class="line">line ` + string(rune('A'+i%26)) + string(rune('a'+i/26)) + `</p>`)
	}
	fake.canvases["F1"] = core.Canvas{Title: "Long", Doc: canvasDoc(t, lines.String())}
	pressCanvas(t, a, keyPress('c'))
	if !strings.Contains(canvasScreen(a), "line Aa") {
		t.Fatalf("top not shown:\n%s", canvasScreen(a))
	}
	pressCanvas(t, a, keyPress('G'))
	if strings.Contains(canvasScreen(a), "line Aa") || !strings.Contains(canvasScreen(a), "line Vd") {
		t.Fatalf("G did not reach the bottom:\n%s", canvasScreen(a))
	}
	pressCanvas(t, a, keyPress('g'))
	if !strings.Contains(canvasScreen(a), "line Aa") {
		t.Fatalf("g did not return to the top:\n%s", canvasScreen(a))
	}
}

// The mouse wheel scrolls the canvas, as it does every modal overlay.
func TestCanvasMouseWheelScrolls(t *testing.T) {
	a, fake := canvasApp(t)
	var lines strings.Builder
	for i := range 60 {
		fmt.Fprintf(&lines, `<p class="line">row %d</p>`, i)
	}
	fake.canvases["F1"] = core.Canvas{Title: "Long", Doc: canvasDoc(t, lines.String())}
	pressCanvas(t, a, keyPress('c'))
	// "row 0 " (with the padding after it) cannot match "row 05".
	if !strings.Contains(canvasScreen(a), "row 0 ") {
		t.Fatalf("top not shown:\n%s", canvasScreen(a))
	}
	for range 5 {
		a.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: 50, Y: 10})
	}
	if strings.Contains(canvasScreen(a), "row 0 ") {
		t.Fatalf("wheel did not scroll:\n%s", canvasScreen(a))
	}
}

func TestWorkspaceSwitchClosesCanvas(t *testing.T) {
	a, _ := canvasApp(t)
	pressCanvas(t, a, keyPress('c'))
	reduceWorkspaceSwitched(a, WorkspaceSwitchedMsg{TeamID: "T2"})
	if a.canvasView.IsVisible() || a.mode == ModeCanvas {
		t.Fatal("canvas viewer survived a workspace switch")
	}
}

// c on a message that shares a canvas opens that canvas, not the
// conversation's.
func TestCanvasKeyOpensTheSelectedMessagesCanvas(t *testing.T) {
	a, fake := canvasApp(t)
	fake.canvases["F7"] = core.Canvas{Title: "Shared", Doc: canvasDoc(t, "<p>shared canvas</p>")}
	a.messagepane.SetMessages([]messages.MessageItem{{
		TS: "1.0", UserID: "U1", UserName: "seth", Text: "here are the notes",
		Attachments: []messages.Attachment{{Kind: "canvas", Name: "Shared", FileID: "F7"}},
	}})
	focusMessages(t, a)
	pressCanvas(t, a, keyPress('c'))
	if len(fake.listed) != 0 || strings.Join(fake.fetched, ",") != "T1/F7" {
		t.Fatalf("listed %v, fetched %v; want only F7", fake.listed, fake.fetched)
	}
	if !strings.Contains(canvasScreen(a), "shared canvas") {
		t.Fatalf("shared canvas not shown:\n%s", canvasScreen(a))
	}
}

// d downloads files; a canvas is not one, so it is not offered as an
// HTML download.
func TestDownloadSkipsCanvases(t *testing.T) {
	a, _ := canvasApp(t)
	a.messagepane.SetMessages([]messages.MessageItem{{
		TS: "1.0", UserID: "U1", UserName: "seth", Text: "notes",
		Attachments: []messages.Attachment{{Kind: "canvas", Name: "Shared", FileID: "F7", DownloadURL: "https://files.slack.com/files-pri/T1-F7/x"}},
	}})
	focusMessages(t, a)
	if got := toastText(drainBatch(a.downloadFilesOfSelected())); got != "No files in message" {
		t.Fatalf("toast = %q", got)
	}
}
