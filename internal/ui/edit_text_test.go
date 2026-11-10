package ui

import (
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"github.com/slack-go/slack"

	"github.com/gammons/slk/internal/core/blocks"
	"github.com/gammons/slk/internal/ui/messages"
	"github.com/gammons/slk/internal/ui/messages/blockkit"
	"github.com/gammons/slk/internal/ui/sidebar"
)

// listMessage is a message written in Slack's client: its text field
// has lost the line breaks and escaped the link, the block has both.
func listMessage(ts string) messages.MessageItem {
	text := func(s string) slack.RichTextSectionElement {
		return &slack.RichTextSectionTextElement{Type: slack.RTSEText, Text: s}
	}
	section := func(el ...slack.RichTextSectionElement) *slack.RichTextSection {
		return &slack.RichTextSection{Type: slack.RTESection, Elements: el}
	}
	return messages.MessageItem{
		TS:     ts,
		UserID: "U_ME",
		Text:   "a list  • one • two in <#CUJL4QPU0>&lt;https://x.com/|a link&gt;",
		Blocks: []blocks.Block{blockkit.RichTextBlock{Elements: []slack.RichTextElement{
			section(text("a list\n")),
			&slack.RichTextList{Type: slack.RTEList, Style: slack.RTEListBullet, Elements: []slack.RichTextElement{
				section(text("one")),
				section(text("two in "), &slack.RichTextSectionChannelElement{Type: slack.RTSEChannel, ChannelID: "CUJL4QPU0"}),
			}},
			section(&slack.RichTextSectionLinkElement{Type: slack.RTSELink, URL: "https://x.com/", Text: "a link"}),
		}}},
	}
}

// listMessageEdit is listMessage in the composer. Slack's client ends
// the paragraph before a list with a line break of its own, which reads
// as a blank line, and starts the link's paragraph right under the
// list; saving it sends the same block.
const listMessageEdit = "a list\n\n- one\n- two in #general\n[a link](https://x.com/)"

// SLK-330: the box shows the message as it was typed (lines, list,
// link, channel name), not Slack's flattened text field.
func TestBeginEditOfSelected_ShowsRichTextAsTyped(t *testing.T) {
	app := newTestApp(t,
		withActiveChannel("C1"),
		withChannels(sidebar.ChannelItem{ID: "CUJL4QPU0", Name: "general", Type: "channel"}),
		withMessages(listMessage("1.0")),
	)
	app.SetCurrentUserID("U_ME")
	app.focusedPanel = PanelMessages

	app.beginEditOfSelected()

	if got := app.compose.Value(); got != listMessageEdit {
		t.Errorf("compose = %q, want %q", got, listMessageEdit)
	}
}

func TestBeginEditOfSelected_ThreadPaneShowsRichTextAsTyped(t *testing.T) {
	app := newTestApp(t,
		withActiveChannel("C1"),
		withChannels(sidebar.ChannelItem{ID: "CUJL4QPU0", Name: "general", Type: "channel"}),
	)
	app.SetCurrentUserID("U_ME")
	app.threadPanel.SetThread(messages.MessageItem{TS: "P1", UserID: "U_OTHER", Text: "parent"},
		[]messages.MessageItem{listMessage("R1")}, "C1", "P1")
	app.threadVisible = true
	app.focusedPanel = PanelThread

	app.beginEditOfSelected()

	if got := app.threadCompose.Value(); got != listMessageEdit {
		t.Errorf("thread compose = %q, want %q", got, listMessageEdit)
	}
}

// openEdit selects row ts of the channel pane and presses E on it, and
// returns what the composer was seeded with.
func openEdit(t *testing.T, a *App, ts string) string {
	t.Helper()
	a.focusedPanel = PanelMessages
	a.messagepane.SelectByTS(ts)
	_ = a.beginEditOfSelected()
	if !a.editing.Matches("C1", ts) {
		t.Fatalf("no edit of %s began", ts)
	}
	return a.compose.Value()
}

// blocksJSON is a body's blocks as JSON, for comparing two of them.
func blocksJSON(t *testing.T, bs []blockkit.Block) string {
	t.Helper()
	var els [][]slack.RichTextElement
	for _, b := range bs {
		if rt, ok := b.(blockkit.RichTextBlock); ok {
			els = append(els, rt.Elements)
		}
	}
	raw, err := json.Marshal(els)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// E then Enter, with nothing changed, sends a message that draws as the
// old one did: the seed shows names, the list, the link and the literal
// asterisks as typed, and chat.update gets text whose block is the
// message's block.
func TestEditUnchangedSendsTheSameBlock(t *testing.T) {
	const typed = "a list\n- one\n- two for <@U9>\n\n[a link](https://x.com/) and \\*not bold\\*"
	a, sent := editAppLog(t, nil)
	row := typedRow("1.000000", typed, "")
	a.messagepane.SetMessages([]messages.MessageItem{row})

	seed := openEdit(t, a, "1.000000")
	if want := "a list\n- one\n- two for @alice\n\n[a link](https://x.com/) and *not bold\\*"; seed != want {
		t.Errorf("seed = %q, want %q", seed, want)
	}
	deliveredMsgs[MessageEditedMsg](t, pressEnter(a))
	if len(*sent) != 1 {
		t.Fatalf("sent %q, want one chat.update", *sent)
	}
	if got, want := blocksJSON(t, messages.TypedBody((*sent)[0]).Blocks), blocksJSON(t, row.Blocks); got != want {
		t.Errorf("saved block %s\nwant %s", got, want)
	}
}

// A message with no rich_text block (an old one, or an app's) is seeded
// from its text, links as markdown and Slack's escapes undone, and
// saves.
func TestEditTextOnlyMessage(t *testing.T) {
	a, sent := editAppLog(t, nil)
	a.messagepane.SetMessages([]messages.MessageItem{
		{TS: "1.000000", UserID: "USELF", Text: "a &lt; b, see <https://x.com/|the docs> <@U9>"},
	})
	if seed, want := openEdit(t, a, "1.000000"), "a < b, see [the docs](https://x.com/) @alice"; seed != want {
		t.Errorf("seed = %q, want %q", seed, want)
	}
	deliveredMsgs[MessageEditedMsg](t, pressEnter(a))
	if want := []string{"a < b, see [the docs](https://x.com/) <@U9>"}; !slices.Equal(*sent, want) {
		t.Errorf("sent %q, want %q", *sent, want)
	}
}

// A message with no text to edit (a file alone) seeds nothing, and
// Enter refuses as for any empty edit, leaving the editor open.
func TestEditEmptyMessageRefuses(t *testing.T) {
	a, sent := editAppLog(t, nil)
	a.messagepane.SetMessages([]messages.MessageItem{{TS: "1.000000", UserID: "USELF"}})
	if seed := openEdit(t, a, "1.000000"); seed != "" {
		t.Fatalf("seed = %q, want empty", seed)
	}
	deliveredMsgs[editEmptyToastMsg](t, pressEnter(a))
	if len(*sent) != 0 || !a.editing.IsActive() {
		t.Errorf("sent %q, editing %v: want the empty refusal and the editor open", *sent, a.editing.IsActive())
	}
}

// A failed edit reopens with the text the user typed, not a fresh seed
// from the message: a token they typed stays a token.
func TestEditFailureReopensWithTheTypedText(t *testing.T) {
	a := editApp(t, errors.New("boom"))
	openEdit(t, a, "1.000000")
	a.compose.SetValue("hi <@U9>")
	res := deliveredMsgs[MessageEditedMsg](t, pressEnter(a))[0]
	a.Update(res)
	if !a.editing.Matches("C1", "1.000000") {
		t.Fatal("the failed edit did not reopen")
	}
	if got := a.compose.Value(); got != "hi <@U9>" {
		t.Errorf("composer = %q, want the text as typed", got)
	}
}
