// internal/ui/emoji_history.go
//
// The user's emoji history: what counts as a use (a reaction Slack
// added, and the emoji of a message, thread reply or upload caption
// Slack accepted), and handing the ranking to every emoji picker (the
// reaction picker and both compose autocompletes), which list frecent
// emoji first. The history is the frecent_emoji cache table, read and
// written through core.ReactionService.LoadFrecent / RecordFrecent off
// the Update goroutine.
package ui

import (
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/gammons/slk/internal/emoji"
	"github.com/gammons/slk/internal/slack/mrkdwn"
)

// frecentEmojiLimit is how many of the user's most frecent emoji the
// pickers rank ahead of the rest.
const frecentEmojiLimit = 30

// emojiHistoryLoadedMsg carries the history as read after a job's
// writes: names most frecent first, the UI's own slice from then on.
type emojiHistoryLoadedMsg struct {
	gen   uint64
	names []string
}

// emojiHistoryIO orders the history jobs, which run as tea.Cmds on
// their own goroutines: each records and then reads under mu, and gen
// numbers the reads in the order they ran. The read with the highest
// gen saw every write before it, so the UI keeps only a newer one than
// it has (results can arrive out of order).
type emojiHistoryIO struct {
	mu  sync.Mutex
	gen uint64
}

// emojiHistory is the App's side of the history: the jobs' shared
// ordering and the gen of the read the pickers show. The ranking
// itself lives in the pickers, built on the Update goroutine from each
// read.
type emojiHistory struct {
	io      *emojiHistoryIO
	applied uint64
}

// emojiHistoryJob records names (canonical, as emoji.Used gives them)
// and then reads the history, as one command. With no names it only
// reads.
func (a *App) emojiHistoryJob(names []string) tea.Cmd {
	io, svc := a.emojiHistory.io, a.reactions
	return func() tea.Msg {
		io.mu.Lock()
		defer io.mu.Unlock()
		for _, n := range names {
			svc.RecordFrecent(n)
		}
		entries := svc.LoadFrecent(frecentEmojiLimit)
		io.gen++
		out := make([]string, len(entries))
		for i, e := range entries {
			out[i] = e.Name
		}
		return emojiHistoryLoadedMsg{gen: io.gen, names: out}
	}
}

// loadEmojiHistory reads the history for the pickers.
func (a *App) loadEmojiHistory() tea.Cmd { return a.emojiHistoryJob(nil) }

// recordEmojiUse adds names to the history and refreshes the pickers'
// ranking. nil when there is nothing to record.
func (a *App) recordEmojiUse(names ...string) tea.Cmd {
	if len(names) == 0 {
		return nil
	}
	return a.emojiHistoryJob(names)
}

// applyEmojiHistory hands a read to the pickers, unless a newer one is
// already showing.
func (a *App) applyEmojiHistory(m emojiHistoryLoadedMsg) {
	if m.gen <= a.emojiHistory.applied {
		return
	}
	a.emojiHistory.applied = m.gen
	f := emoji.NewFrecency(m.names)
	a.reactionPicker.SetFrecency(f)
	a.compose.SetEmojiFrecency(f)
	a.threadCompose.SetEmojiFrecency(f)
}

// usedEmoji is the emoji text sends, by the names the history records
// them under, judged against the custom emoji of the workspace it was
// typed in.
func usedEmoji(text string, customs map[string]string) []string {
	return emoji.Used(mrkdwn.EmojiShortcodes(text), customs)
}
