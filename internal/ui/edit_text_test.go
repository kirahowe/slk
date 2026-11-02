package ui

import (
	"errors"
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

const listMessageEdit = "a list\n- one\n- two in #general\n\n[a link](https://x.com/)"

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

// A message typed in slk comes back in the editor as it was typed;
// saving it unchanged sends the text it was sent with, and a save that
// fails reopens the editor on what the user typed.
func TestEditUnchanged_SendsTheSameMessage(t *testing.T) {
	const typed = "hi @alice, see:\n- one\n- **two**\n\n[docs](https://x.com/)"
	a, sent := editAppLog(t, errors.New("boom"))
	wire := a.compose.TranslateMentionsForSend(typed)
	row := messages.MessageItem{TS: "1.000000", UserID: "USELF"}
	messages.TypedBody(wire).Apply(&row)
	a.messagepane.SetMessages([]messages.MessageItem{row})
	a.focusedPanel = PanelMessages
	a.messagepane.SelectByTS("1.000000")

	_ = a.beginEditOfSelected()
	if got := a.compose.Value(); got != typed {
		t.Fatalf("editor = %q, want %q", got, typed)
	}
	res := deliveredMsgs[MessageEditedMsg](t, pressEnter(a))[0]
	if len(*sent) != 1 || (*sent)[0] != wire {
		t.Fatalf("sent %q, want %q", *sent, wire)
	}
	a.Update(res)
	if !a.editing.Matches("C1", "1.000000") {
		t.Fatalf("editing=%+v, want the failed edit reopened", *a.editing)
	}
	if got := a.compose.Value(); got != typed {
		t.Errorf("reopened editor = %q, want %q", got, typed)
	}
}
