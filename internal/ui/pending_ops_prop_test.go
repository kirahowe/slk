package ui

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/gammons/slk/internal/ui/messages"
)

// However sends in two conversations begin, echo and end (in any
// order, a send's echo before or after its result, sends failing with
// or without Slack having posted them, and the user's messages from
// other clients among them), each message is shown exactly once: a
// successful send by its result, never by its echo, and anything else
// by its echo, once released. A held message is never shown while a
// send is pending in its conversation, and once every send has ended
// nothing is held and no placeholder is pending. A pane of either
// conversation may be reloaded from Slack at any point (pendingOps.view):
// it then shows every pending send's placeholder, no message twice and
// none of the user's messages that may still be a pending send's (Slack
// may have posted a send before answering it).
//
// Some of the top-level sends are forwards, ended by EndForward. A
// successful forward is shown once by its result, and ends up showing
// Slack's copy: its echo, held and handed back by EndForward, or
// replacing the result's stand-in once (ForwardEcho), or a reload.
func TestHoldAndReleaseShowsEachMessageOnce(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		p := newPendingOps()
		convs := []conversationKey{{"", "C1", ""}, {"", "C1", "5.000000"}}
		base := time.Now().Unix()
		type send struct {
			ins      pendingInsert
			real     string
			ended    bool
			ok       bool
			echoed   bool
			posts    bool // Slack posted it (true unless it failed cleanly)
			resulted bool
			forward  bool
			replaced bool // its echo replaced the stand-in
		}
		var sends []*send
		var foreign []string
		foreignIn := map[string]conversationKey{}
		shown := map[string]string{} // TS -> "result", "echo" or "fetch"
		// slackCopy are the forwards whose rows show Slack's copy.
		slackCopy := map[string]bool{}
		sendOf := func(ts string) *send {
			for _, s := range sends {
				if s.real == ts {
					return s
				}
			}
			return nil
		}
		// viaFetch are messages a reload held: their echo may follow
		// their release, and showing a shown row again changes nothing
		// (messages.Insert).
		viaFetch := map[string]bool{}
		next := 0
		newTS := func() string { next++; return fmt.Sprintf("%d.%06d", base, next) }
		// show records that a message is shown. A reload shows rows
		// already shown again, which changes nothing; anything else
		// must show a message once.
		show := func(ts, by string) {
			prev, ok := shown[ts]
			switch {
			case by == "fetch":
				if !ok {
					shown[ts] = by
				}
				return
			case ok && prev != "fetch" && !(by == "echo" && viaFetch[ts]):
				t.Fatalf("%s shown twice (by %s, then %s)", ts, prev, by)
			}
			shown[ts] = by
		}
		// deliver is reduceNewMessage's decision for one of the user's
		// messages.
		deliver := func(k conversationKey, ts string) {
			msg := NewMessageMsg{ChannelID: k.channelID, Message: messages.MessageItem{TS: ts, ThreadTS: k.threadTS, UserID: "USELF"}}
			if p.ForwardEcho(msg) {
				s := sendOf(ts)
				if s == nil || !s.forward || !s.ok || s.replaced {
					t.Fatalf("ForwardEcho(%s) for %+v", ts, s)
				}
				s.replaced, slackCopy[ts] = true, true
				return
			}
			if p.IsSelfSent(ts) || p.HoldEcho(msg) {
				return
			}
			show(ts, "echo")
		}
		pendingIn := func(k conversationKey) bool {
			return slices.ContainsFunc(sends, func(s *send) bool { return !s.ended && s.ins.conversation() == k })
		}
		// end is the result of s: EndForward for a forward, which
		// settles with its held echo if there is one, else EndInsert.
		// It shows what the result shows.
		end := func(s *send, sentTS string) (pendingInsert, []NewMessageMsg, bool) {
			if !s.forward {
				ins, release, ok := p.EndInsert(s.ins.localTS, sentTS)
				if s.ok {
					show(s.real, "result")
				}
				return ins, release, ok
			}
			ins, echo, echoed, release, ok := p.EndForward(s.ins.localTS, sentTS)
			switch {
			case echoed && (!s.ok || echo.Message.TS != s.real):
				t.Fatalf("EndForward(%s, %q) handed back %s", s.ins.localTS, sentTS, echo.Message.TS)
			case echoed:
				show(s.real, "result")
				slackCopy[s.real] = true
			case s.ok:
				show(s.real, "result")
			}
			return ins, release, ok
		}
		for range rapid.IntRange(1, 40).Draw(t, "steps") {
			switch rapid.IntRange(0, 4).Draw(t, "action") {
			case 0: // begin
				k := rapid.SampledFrom(convs).Draw(t, "conv")
				local := p.NextLocalTS()
				s := &send{ins: pendingInsert{
					localTS: local, channelID: k.channelID, threadTS: k.threadTS,
					row: messages.MessageItem{TS: local, ThreadTS: k.threadTS, UserID: "USELF"}, begun: time.Now(),
				}, real: newTS()}
				// A forward goes to a conversation's top level.
				s.forward = k.threadTS == "" && rapid.Bool().Draw(t, "forward")
				p.BeginInsert(s.ins)
				sends = append(sends, s)
			case 1: // the echo of a send Slack posted
				var cands []*send
				for _, s := range sends {
					if !s.echoed && (!s.resulted || s.posts) {
						cands = append(cands, s)
					}
				}
				if len(cands) == 0 {
					continue
				}
				s := rapid.SampledFrom(cands).Draw(t, "echo")
				s.echoed, s.posts = true, true
				deliver(s.ins.conversation(), s.real)
			case 2: // a message from another client
				ts := newTS()
				k := rapid.SampledFrom(convs).Draw(t, "conv")
				foreign = append(foreign, ts)
				foreignIn[ts] = k
				deliver(k, ts)
			case 3: // a result
				var open []*send
				for _, s := range sends {
					if !s.ended {
						open = append(open, s)
					}
				}
				if len(open) == 0 {
					continue
				}
				s := rapid.SampledFrom(open).Draw(t, "result")
				s.ended, s.resulted = true, true
				s.ok = rapid.Bool().Draw(t, "ok")
				sentTS := ""
				if s.ok {
					sentTS, s.posts = s.real, true
				} else if !s.echoed && !s.posts {
					s.posts = rapid.Bool().Draw(t, "posted anyway")
				}
				ins, release, ok := end(s, sentTS)
				if !ok || !reflect.DeepEqual(ins, s.ins) {
					t.Fatalf("EndInsert(%s) = %+v %v", s.ins.localTS, ins, ok)
				}
				if len(release) > 0 && pendingIn(s.ins.conversation()) {
					t.Fatalf("released %v while a send is pending in %v", release, s.ins.conversation())
				}
				for _, m := range release {
					if conversationOf(m.TeamID, m.ChannelID, m.Message) != s.ins.conversation() {
						t.Fatalf("released %+v for conversation %v", m, s.ins.conversation())
					}
					show(m.Message.TS, "echo")
				}
			case 4: // a pane of a conversation reloads from Slack
				k := rapid.SampledFrom(convs).Draw(t, "reload")
				var fetched []messages.MessageItem
				for _, ts := range foreign {
					if foreignIn[ts] == k {
						fetched = append(fetched, messages.MessageItem{TS: ts, ThreadTS: k.threadTS, UserID: "USELF"})
					}
				}
				for _, s := range sends {
					if s.ins.conversation() != k {
						continue
					}
					// Slack may have posted a pending send already.
					if !s.posts && !s.ended && rapid.Bool().Draw(t, "posted before its result") {
						s.posts = true
					}
					if s.posts {
						fetched = append(fetched, messages.MessageItem{TS: s.real, ThreadTS: k.threadTS, UserID: "USELF"})
					}
				}
				slices.SortFunc(fetched, func(x, y messages.MessageItem) int { return messages.CompareTS(x.TS, y.TS) })
				heldBefore := map[string]bool{}
				for _, m := range p.held[k] {
					heldBefore[m.Message.TS] = true
				}
				rows, _ := p.view(paneView{teamID: k.teamID, channelID: k.channelID, threadTS: k.threadTS, selfID: "USELF"}, fetched, nil)
				for _, m := range p.held[k] {
					if !heldBefore[m.Message.TS] {
						viaFetch[m.Message.TS] = true
					}
				}
				seen := map[string]bool{}
				for _, r := range rows {
					if seen[r.TS] {
						t.Fatalf("reload of %v shows %s twice: %v", k, r.TS, rowsOf(rows))
					}
					seen[r.TS] = true
				}
				for _, s := range sends {
					if s.ins.conversation() != k || s.ended {
						continue
					}
					if !seen[s.ins.localTS] {
						t.Fatalf("reload of %v lost the placeholder of pending send %s: %v", k, s.ins.localTS, rowsOf(rows))
					}
					if seen[s.real] {
						t.Fatalf("reload of %v shows pending send %s beside its placeholder: %v", k, s.real, rowsOf(rows))
					}
				}
				for _, m := range p.held[k] {
					if seen[m.Message.TS] {
						t.Fatalf("reload of %v shows held %s", k, m.Message.TS)
					}
				}
				for _, r := range rows {
					if !messages.IsLocalTS(r.TS) {
						if s := sendOf(r.TS); s != nil && s.forward {
							slackCopy[r.TS] = true
						}
						show(r.TS, "fetch")
					}
				}
			}
		}
		// Every send ends, and every posted send's echo arrives.
		for _, s := range sends {
			if !s.ended {
				s.ended, s.ok, s.posts = true, true, true
				_, release, _ := end(s, s.real)
				for _, m := range release {
					show(m.Message.TS, "echo")
				}
			}
		}
		for _, s := range sends {
			if s.posts && !s.echoed {
				deliver(s.ins.conversation(), s.real)
			}
		}
		for _, s := range sends {
			want := ""
			switch {
			case s.ok:
				want = "result"
			case s.posts:
				want = "echo"
			}
			if shown[s.real] != want {
				t.Fatalf("send %s (ok=%v posted=%v) shown by %q, want %q", s.real, s.ok, s.posts, shown[s.real], want)
			}
			if s.forward && s.ok && !slackCopy[s.real] {
				t.Fatalf("forward %s still shows its stand-in after its echo", s.real)
			}
		}
		for _, ts := range foreign {
			if shown[ts] != "echo" {
				t.Fatalf("message %s from another client never shown", ts)
			}
		}
		if len(p.inserts) != 0 {
			t.Fatalf("inserts left: %+v", p.inserts)
		}
		for k, held := range p.held {
			if len(held) != 0 {
				t.Fatalf("held in %v after every send ended: %+v", k, held)
			}
		}
	})
}
