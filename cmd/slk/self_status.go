package main

import (
	"context"
	"sync"
	"time"
)

// selfState is the user's own presence and DND in one workspace, as one
// consistent copy.
type selfState struct {
	Presence   string    // "active" or "away"; "" until first fetch
	DNDEnabled bool      // true if either snooze or admin-DND is active
	DNDEndTS   time.Time // unified end timestamp; zero if not in DND
}

// InDND reports whether DND is in force at now: enabled and either
// open-ended or not yet over.
func (s selfState) InDND(now time.Time) bool {
	return s.DNDEnabled && (s.DNDEndTS.IsZero() || now.Before(s.DNDEndTS))
}

// selfStatus holds a workspace's selfState. Two goroutines write it: the
// WebSocket event owner (manual_presence_change, dnd_updated) and the
// connect-time fetch, which runs off the owner so its HTTP calls do not
// stall event delivery. So it is locked, like userNameStore, rather than
// owned.
//
// The zero value is ready to use.
type selfStatus struct {
	// publish orders each change together with the report of it, so the
	// UI receives StatusChangeMsgs in the order the changes were made. It
	// is held across the send, so mu is kept separate for readers that
	// must not wait on the UI.
	publish sync.Mutex
	mu      sync.Mutex
	state   selfState
	// presenceGen and dndGen count the writes of each half. A fetch
	// records them before its requests and keeps a half only when no
	// write has landed since, so an answer Slack gave before a live
	// event does not overwrite that event.
	presenceGen, dndGen uint64
}

// selfStatusMark is the write counts a fetch saw when it began.
type selfStatusMark struct{ presence, dnd uint64 }

// fetchedDND is a dnd.info answer reduced to what selfState holds.
type fetchedDND struct {
	enabled bool
	end     time.Time
}

// Get returns the current state. Any goroutine may call it.
func (s *selfStatus) Get() selfState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// mark records the write counts at the start of a fetch, for applyFetched.
func (s *selfStatus) mark() selfStatusMark {
	s.mu.Lock()
	defer s.mu.Unlock()
	return selfStatusMark{s.presenceGen, s.dndGen}
}

// SetPresence stores a presence the socket reported, then passes the
// resulting state to report (nil to skip).
func (s *selfStatus) SetPresence(presence string, report func(selfState)) {
	s.update(func() {
		s.state.Presence = presence
		s.presenceGen++
	}, report)
}

// SetDND stores a DND state the socket reported, then passes the
// resulting state to report (nil to skip).
func (s *selfStatus) SetDND(enabled bool, end time.Time, report func(selfState)) {
	s.update(func() {
		s.state.DNDEnabled, s.state.DNDEndTS = enabled, end
		s.dndGen++
	}, report)
}

// applyFetched stores what a fetch begun at m learned (nil for a half it
// could not get), keeping each half only if nothing wrote it since m.
func (s *selfStatus) applyFetched(m selfStatusMark, presence *string, dnd *fetchedDND) {
	s.update(func() {
		if presence != nil && s.presenceGen == m.presence {
			s.state.Presence = *presence
			s.presenceGen++
		}
		if dnd != nil && s.dndGen == m.dnd {
			s.state.DNDEnabled, s.state.DNDEndTS = dnd.enabled, dnd.end
			s.dndGen++
		}
	}, nil)
}

// Report passes the current state to report, ordered with every change's
// own report.
func (s *selfStatus) Report(report func(selfState)) {
	s.update(func() {}, report)
}

func (s *selfStatus) update(change func(), report func(selfState)) {
	s.publish.Lock()
	defer s.publish.Unlock()
	s.mu.Lock()
	change()
	st := s.state
	s.mu.Unlock()
	if report != nil {
		report(st)
	}
}

// latestRunner runs a job on its own goroutine at most one at a time,
// where only the most recent request matters. A request while a job runs
// cancels that job's context and queues itself, replacing any request
// already queued, so a burst of requests ends with exactly one run of the
// newest and an older run can never finish after a newer one.
//
// The context a job gets is cancelled only when a newer request
// supersedes it; a job bounds its own work with a deadline. The zero
// value is ready to use.
type latestRunner struct {
	mu      sync.Mutex
	running bool
	next    func(context.Context)
	cancel  context.CancelFunc
}

// Run starts job now, or after the job in flight returns.
func (r *latestRunner) Run(job func(context.Context)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.running {
		r.next = job
		r.cancel()
		return
	}
	r.running = true
	r.start(job)
}

// start launches job; r.mu is held.
func (r *latestRunner) start(job func(context.Context)) {
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	go func() {
		job(ctx)
		cancel()
		r.mu.Lock()
		defer r.mu.Unlock()
		next := r.next
		r.next = nil
		if next == nil {
			r.running = false
			return
		}
		r.start(next)
	}()
}
