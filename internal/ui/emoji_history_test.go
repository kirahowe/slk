package ui

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/gammons/slk/internal/core"
	"github.com/gammons/slk/internal/ids"
	"github.com/gammons/slk/internal/ui/messages"
)

// emojiHistoryLog is an in-memory emoji history (most recently used
// first) and every name RecordFrecent was given, in order.
type emojiHistoryLog struct {
	mu       sync.Mutex
	history  []string
	recorded []string
}

func (h *emojiHistoryLog) snapshot() (history, recorded []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.history), slices.Clone(h.recorded)
}

// emojiHistoryApp returns an App whose emoji history is kept in
// memory, and the history.
func emojiHistoryApp(t *testing.T, opts ...testOpt) (*App, *emojiHistoryLog) {
	t.Helper()
	a := newTestApp(t, opts...)
	h := &emojiHistoryLog{}
	a.SetReactionService(core.NewReactionService(
		func(ids.ChannelID, ids.MessageTS, string) error { return nil },
		func(ids.ChannelID, ids.MessageTS, string) error { return nil },
		func(limit int) []core.EmojiEntry {
			h.mu.Lock()
			defer h.mu.Unlock()
			var out []core.EmojiEntry
			for _, n := range h.history[:min(limit, len(h.history))] {
				out = append(out, core.EmojiEntry{Name: n})
			}
			return out
		},
		func(name string) {
			h.mu.Lock()
			defer h.mu.Unlock()
			h.recorded = append(h.recorded, name)
			h.history = append([]string{name}, slices.DeleteFunc(h.history, func(n string) bool { return n == name })...)
		},
	))
	return a, h
}

// updateSettled delivers msg through Update and settles what it starts.
func updateSettled(t *testing.T, a *App, msg tea.Msg) {
	t.Helper()
	_, cmd := a.Update(msg)
	settle(t, a, cmd)
}

// autocompleteRows is what the channel composer's emoji autocomplete
// lists after typing s, one name per row in order. The composer is
// emptied again afterwards.
func autocompleteRows(t *testing.T, a *App, s string) []string {
	t.Helper()
	a.SetMode(ModeInsert)
	_ = a.compose.Focus()
	typeInCompose(a, s)
	view := stripANSI(a.compose.EmojiPickerView(80))
	a.compose.SetValue("")
	a.compose.CloseEmoji()
	a.SetMode(ModeNormal)
	if view == "" {
		t.Fatalf("no emoji autocomplete after typing %q", s)
	}
	var rows []string
	for _, line := range strings.Split(view, "\n") {
		fields := strings.Fields(strings.Trim(line, "│╭╮╰╯─▌ "))
		if len(fields) > 0 {
			rows = append(rows, strings.Trim(fields[len(fields)-1], ":"))
		}
	}
	return rows
}

// lastLocalTS is the placeholder id of the newest pending send.
func lastLocalTS(t *testing.T, a *App) string {
	t.Helper()
	if len(a.pendingOps.inserts) == 0 {
		t.Fatal("no send is pending")
	}
	return a.pendingOps.inserts[len(a.pendingOps.inserts)-1].localTS
}

// A sent message's emoji are recorded once Slack accepts it, under
// their canonical names, and then lead the composer's autocomplete.
// An unknown shortcode, a code span and a typed URL record nothing.
func TestSentMessageEmojiLeadTheAutocomplete(t *testing.T) {
	a, h := emojiHistoryApp(t, withActiveChannel("C1"))
	if rows := autocompleteRows(t, a, ":ro"); len(rows) == 0 || rows[0] == "rose" {
		t.Fatalf("precondition: rows = %v, want an alphabetical list not led by rose", rows)
	}

	_ = a.beginSend(sendRequest{ChannelID: "C1",
		Text: "for you :rose: :thumbsup: 12:30:45 `:tada:` see:https://x.io/a"})
	if _, recorded := h.snapshot(); len(recorded) != 0 {
		t.Fatalf("recorded %v before Slack answered, want nothing", recorded)
	}
	updateSettled(t, a, MessageSentMsg{ChannelID: "C1", LocalTS: lastLocalTS(t, a),
		Message: messages.MessageItem{TS: "1700000000.000001", Text: "for you"}})

	if _, recorded := h.snapshot(); !reflect.DeepEqual(recorded, []string{"rose", "+1"}) {
		t.Errorf("recorded = %v, want [rose +1] (canonical names, no :30:, :tada: or :https:)", recorded)
	}
	if rows := autocompleteRows(t, a, ":ro"); len(rows) == 0 || rows[0] != "rose" {
		t.Errorf("autocomplete rows = %v, want rose first", rows)
	}
}

// A send that fails records nothing, so the same text sent again
// counts once.
func TestFailedSendThenRetryRecordsOnce(t *testing.T) {
	a, h := emojiHistoryApp(t, withActiveChannel("C1"))
	_ = a.beginSend(sendRequest{ChannelID: "C1", Text: "ship :rocket:"})
	updateSettled(t, a, MessageSendFailedMsg{ChannelID: "C1", LocalTS: lastLocalTS(t, a), Reason: "timeout"})
	if _, recorded := h.snapshot(); len(recorded) != 0 {
		t.Fatalf("a failed send recorded %v, want nothing", recorded)
	}
	_ = a.beginSend(sendRequest{ChannelID: "C1", Text: "ship :rocket:"})
	updateSettled(t, a, MessageSentMsg{ChannelID: "C1", LocalTS: lastLocalTS(t, a),
		Message: messages.MessageItem{TS: "1700000000.000002"}})
	if _, recorded := h.snapshot(); !reflect.DeepEqual(recorded, []string{"rocket"}) {
		t.Errorf("recorded = %v, want rocket once", recorded)
	}
}

func TestSentThreadReplyEmojiAreRecorded(t *testing.T) {
	a, h := emojiHistoryApp(t, withActiveChannel("C1"))
	_ = a.beginReply(replyRequest{ChannelID: "C1", ThreadTS: "1.0", Text: "done :tada:"})
	if _, recorded := h.snapshot(); len(recorded) != 0 {
		t.Fatalf("recorded %v before Slack answered, want nothing", recorded)
	}
	updateSettled(t, a, ThreadReplySentMsg{ChannelID: "C1", ThreadTS: "1.0", LocalTS: lastLocalTS(t, a),
		Message: messages.MessageItem{TS: "1.5", ThreadTS: "1.0"}})
	if _, recorded := h.snapshot(); !reflect.DeepEqual(recorded, []string{"tada"}) {
		t.Errorf("recorded = %v, want [tada]", recorded)
	}
}

// An upload caption's emoji are recorded when the upload succeeds, and
// not when it fails.
func TestUploadCaptionEmojiRecordedOnSuccess(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want []string
	}{
		{"success", nil, []string{"camera"}},
		{"failure", errors.New("too large"), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, h := emojiHistoryApp(t, withActiveChannel("C1"))
			a.focusedPanel = PanelMessages
			a.compose.AddAttachment(core.PendingAttachment{Filename: "a.png", Bytes: []byte("png"), Size: 3})
			a.compose.SetValue("look :camera:")
			a.setUploaderForTest(func(string, string, string, []core.PendingAttachment) tea.Cmd {
				return func() tea.Msg { return UploadResultMsg{Err: tc.err} }
			})
			settle(t, a, a.submitWithAttachments(&a.compose))
			if _, recorded := h.snapshot(); !reflect.DeepEqual(recorded, tc.want) {
				t.Errorf("recorded = %v, want %v", recorded, tc.want)
			}
		})
	}
}

// A reaction is a use once Slack adds it: not when the call fails, and
// taking one back is not a use. The pill toggle goes the same way.
func TestReactionRecordedWhenSlackAddsIt(t *testing.T) {
	a, h := emojiHistoryApp(t, withActiveChannel("C1"))
	updateSettled(t, a, ReactionSentMsg{ChannelID: "C1", MessageTS: "1.0", Emoji: "eyes", Err: errors.New("boom")})
	updateSettled(t, a, ReactionSentMsg{ChannelID: "C1", MessageTS: "1.0", Emoji: "tada", Remove: true})
	if _, recorded := h.snapshot(); len(recorded) != 0 {
		t.Fatalf("recorded %v for a failed add and a removal, want nothing", recorded)
	}

	a.SetCurrentUserID("U1")
	msg := messages.MessageItem{TS: "1.0", Reactions: []messages.ReactionItem{
		{Emoji: "eyes", Count: 1, UserIDs: []string{"U2"}},
	}}
	settle(t, a, a.toggleReactionOnMessageItem("C1", msg, "eyes"))
	if _, recorded := h.snapshot(); !reflect.DeepEqual(recorded, []string{"eyes"}) {
		t.Errorf("recorded = %v, want [eyes] after the pill was toggled on", recorded)
	}
}

// An edit is not a use, however its result comes back.
func TestEditRecordsNothing(t *testing.T) {
	a, h := emojiHistoryApp(t, withActiveChannel("C1"),
		withMessages(messages.MessageItem{TS: "1.0", UserID: "U1", Text: "hi"}))
	_ = a.beginEdit(editRequest{ChannelID: "C1", TS: "1.0", NewText: "hi :tada:"})
	updateSettled(t, a, MessageEditedMsg{ChannelID: "C1", TS: "1.0", Text: "hi :tada:", EditID: a.pendingOps.editID})
	if _, recorded := h.snapshot(); len(recorded) != 0 {
		t.Errorf("an edit recorded %v, want nothing", recorded)
	}
}

// A custom emoji records and ranks like a built-in. What a send uses is
// judged against the custom emoji of the workspace it was typed in,
// even if the user switches before Slack answers.
func TestCustomEmojiRecordedByTheWorkspaceTypedIn(t *testing.T) {
	a, h := emojiHistoryApp(t, withActiveChannel("C1"))
	customs := map[string]string{"partyparrot": "https://e/p.gif"}
	a.SetCustomEmoji(customs)
	_ = a.beginSend(sendRequest{ChannelID: "C1", Text: "yes :partyparrot:"})
	a.SetCustomEmoji(nil) // another workspace, without it
	updateSettled(t, a, MessageSentMsg{ChannelID: "C1", LocalTS: lastLocalTS(t, a),
		Message: messages.MessageItem{TS: "1700000000.000003"}})
	if _, recorded := h.snapshot(); !reflect.DeepEqual(recorded, []string{"partyparrot"}) {
		t.Fatalf("recorded = %v, want [partyparrot]", recorded)
	}

	a.SetCustomEmoji(map[string]string{"partyparrot": "https://e/p.gif", "party_blob": "https://e/b.gif"})
	if rows := autocompleteRows(t, a, ":part"); len(rows) == 0 || rows[0] != "partyparrot" {
		t.Errorf("autocomplete rows = %v, want the used custom partyparrot first", rows)
	}
}

// A read older than the one the pickers show is dropped: reads finish
// in any order, and the newest saw every write before it.
func TestOlderEmojiHistoryReadIsDropped(t *testing.T) {
	a := newTestApp(t)
	a.Update(emojiHistoryLoadedMsg{gen: 2, names: []string{"rose"}})
	a.Update(emojiHistoryLoadedMsg{gen: 1, names: []string{"rocket"}})
	if rows := autocompleteRows(t, a, ":ro"); len(rows) == 0 || rows[0] != "rose" {
		t.Errorf("autocomplete rows = %v, want rose (the newer read) first", rows)
	}
}

// Two history jobs never interleave their writes and reads, and the
// later read numbers higher: run concurrently, the read with the
// highest gen holds both writes.
func TestEmojiHistoryJobsAreOrdered(t *testing.T) {
	a, _ := emojiHistoryApp(t)
	got := deliveredMsgs[emojiHistoryLoadedMsg](t, a.recordEmojiUse("rose"), a.recordEmojiUse("tada"))
	newest := got[0]
	if got[1].gen > newest.gen {
		newest = got[1]
	}
	if !slices.Contains(newest.names, "rose") || !slices.Contains(newest.names, "tada") {
		t.Errorf("newest read (gen %d) = %v, want both rose and tada", newest.gen, newest.names)
	}
	if got[0].gen == got[1].gen {
		t.Errorf("both reads have gen %d", got[0].gen)
	}
}

// The history is read when the program starts.
func TestInitLoadsEmojiHistory(t *testing.T) {
	a, h := emojiHistoryApp(t)
	h.history = []string{"rose"}
	settle(t, a, a.Init())
	if rows := autocompleteRows(t, a, ":ro"); len(rows) == 0 || rows[0] != "rose" {
		t.Errorf("autocomplete rows = %v, want rose first after Init", rows)
	}
}
