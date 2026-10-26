package main

import (
	"sync"

	"github.com/gammons/slk/internal/ui"
)

// activityBadge orders a workspace's Activity-badge totals on their way
// to the UI. Two goroutines produce them: the WebSocket event owner
// (badge_counts_updated) and the reconnect catch-up's client.counts
// fetch, which runs off the owner. It uses selfStatus's scheme: a fetch
// records the write count before its request, and its answer is
// dropped if an event wrote a total since, because Slack gave the
// answer before that event. Each total is reported while the lock is
// held, so the UI receives them in the order they were accepted.
//
// The zero value is ready to use.
type activityBadge struct {
	mu  sync.Mutex
	gen uint64 // writes so far; a fetch's mark is the count it began at
}

// activityBadgeMark is the write count a fetch saw when it began.
type activityBadgeMark uint64

// mark records the write count at the start of a fetch, for applyFetched.
func (b *activityBadge) mark() activityBadgeMark {
	b.mu.Lock()
	defer b.mu.Unlock()
	return activityBadgeMark(b.gen)
}

// Set accepts a total the socket reported and passes it to report.
func (b *activityBadge) Set(total int, report func(int)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.gen++
	report(total)
}

// applyFetched accepts the total of a fetch begun at m and passes it to
// report, unless something wrote a total since m. It reports whether
// the total was kept.
func (b *activityBadge) applyFetched(m activityBadgeMark, total int, report func(int)) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.gen != uint64(m) {
		return false
	}
	b.gen++
	report(total)
	return true
}

// activityBadgeReport returns the report func that hands teamID's total
// to the UI (nil program: a test without one).
func activityBadgeReport(program teaSender, teamID string) func(int) {
	return func(total int) {
		if program != nil {
			program.Send(ui.ActivityBadgeUpdatedMsg{TeamID: teamID, Total: total})
		}
	}
}

// OnBadgeCountsUpdated forwards the server's Activity-badge
// recalculation to the UI. The total is absolute, so the UI can apply
// it without knowing what changed. Going through wsCtx.Badge makes a
// reconnect's client.counts answer that was in flight when this
// arrived too old to apply.
func (h *rtmEventHandler) OnBadgeCountsUpdated(activityTotal int) {
	if h.wsCtx == nil {
		return
	}
	h.wsCtx.Badge.Set(activityTotal, activityBadgeReport(h.program, h.workspaceID))
}
