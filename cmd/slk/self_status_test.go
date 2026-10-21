package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/gammons/slk/internal/ui"
	"github.com/gammons/slk/internal/ui/sidebar"
)

// A request while a job runs cancels it and waits for it to return; of
// several such requests only the newest runs, and nothing overlaps.
func TestLatestRunnerRunsNewestAfterCurrent(t *testing.T) {
	var r latestRunner
	var mu sync.Mutex
	var log []string
	record := func(s string) {
		mu.Lock()
		log = append(log, s)
		mu.Unlock()
	}
	started := make(chan struct{})
	release := make(chan struct{})
	r.Run(func(ctx context.Context) {
		close(started)
		<-release
		if ctx.Err() == nil {
			t.Error("superseded job's context was not cancelled")
		}
		record("1")
	})
	<-started
	r.Run(func(context.Context) { record("2") })
	done3 := make(chan struct{})
	r.Run(func(context.Context) { record("3"); close(done3) })
	close(release)
	<-done3
	// A later request runs too, so nothing was left queued behind 3.
	done4 := make(chan struct{})
	r.Run(func(context.Context) { record("4"); close(done4) })
	<-done4

	mu.Lock()
	defer mu.Unlock()
	if want := []string{"1", "3", "4"}; !slices.Equal(log, want) {
		t.Fatalf("ran %v, want %v", log, want)
	}
}

// selfStatusServer answers users.getPresence and dnd.info. Presence
// request n (from 1) sends n on started, waits for release(n) and then
// answers presence[n-1]. Releasing twice is harmless.
func selfStatusServer(t *testing.T, presence ...string) (srv *httptest.Server, started <-chan int, release func(n int)) {
	t.Helper()
	startedCh := make(chan int, len(presence))
	releases := make([]chan struct{}, len(presence))
	onces := make([]sync.Once, len(presence))
	for i := range releases {
		releases[i] = make(chan struct{})
	}
	release = func(n int) { onces[n-1].Do(func() { close(releases[n-1]) }) }
	var presenceCalls atomic.Int32
	srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/users.getPresence":
			n := int(presenceCalls.Add(1))
			if n > len(presence) {
				t.Errorf("presence request %d; only %d expected", n, len(presence))
				http.Error(w, `{"ok":false,"error":"unexpected"}`, http.StatusInternalServerError)
				return
			}
			startedCh <- n
			<-releases[n-1]
			_, _ = w.Write([]byte(`{"ok":true,"presence":"` + presence[n-1] + `"}`))
		case "/api/dnd.info":
			_, _ = w.Write([]byte(`{"ok":true,"dnd_enabled":false}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			http.Error(w, `{"ok":false,"error":"unexpected_path"}`, http.StatusNotFound)
		}
	}))
	// Cleanups run last-in first-out: blocked handlers must be let go
	// before Close waits for them.
	t.Cleanup(srv.Close)
	t.Cleanup(func() {
		for n := range presence {
			release(n + 1)
		}
	})
	return srv, startedCh, release
}

// selfStatusHandler is a handler on a fresh workspace whose StatusChangeMsgs
// arrive on the returned channel.
func selfStatusHandler(t *testing.T, srv *httptest.Server) (*rtmEventHandler, <-chan ui.StatusChangeMsg) {
	t.Helper()
	wctx := &WorkspaceContext{
		Client: newTestClient(t, srv), TeamID: "T1", UserID: "USELF",
		channels: []sidebar.ChannelItem{{ID: "D1", Type: "dm", DMUserID: "U1"}},
	}
	wctx.publishChannels()
	statuses := make(chan ui.StatusChangeMsg, 16)
	h := &rtmEventHandler{wsCtx: wctx, workspaceID: "T1", program: sendFunc(func(msg tea.Msg) {
		if m, ok := msg.(ui.StatusChangeMsg); ok {
			statuses <- m
		}
	})}
	return h, statuses
}

// A reconnect flap: the first connect's fetch is still waiting on Slack
// when the second connect comes. Even when the first answer is applied
// after the second fetch has begun, the second connect's answer is the
// one left showing.
func TestOnConnectNewestSelfStatusWins(t *testing.T) {
	srv, started, release := selfStatusServer(t, "away", "active")
	h, statuses := selfStatusHandler(t, srv)

	h.OnConnect()
	<-started
	h.OnConnect()
	<-started
	// The older fetch's answer comes back last, the order that let it win
	// when each connect started its own goroutine. Now the second connect
	// cancels the first fetch, so release(1) answers nothing.
	release(2)
	<-statuses
	release(1)
	last := <-statuses

	if last.Presence != "active" {
		t.Fatalf("last StatusChangeMsg presence = %q, want the second connect's active", last.Presence)
	}
	if got := h.wsCtx.Self.Get().Presence; got != "active" {
		t.Fatalf("stored presence = %q, want the second connect's active", got)
	}
}

// Overlapping connect-time fetches write the self status while the event
// owner writes and reads it for manual_presence_change and dnd_updated.
// Under -race this reports any of those accesses left unsynchronized; the
// race detector orders by happens-before, so it does not depend on timing.
func TestSelfStatusFetchesRaceEvents(t *testing.T) {
	srv, started, release := selfStatusServer(t, "away", "active")
	h, statuses := selfStatusHandler(t, srv)

	h.OnConnect()
	<-started
	h.OnConnect()
	<-started
	release(1)
	release(2)
	h.OnSelfPresenceChange("away")
	h.OnDNDChange(true, 0)
	_ = h.wsCtx.Self.Get()

	// Two event reports and one per fetch run.
	for range 4 {
		<-statuses
	}
}

// A manual_presence_change that lands while the connect-time fetch is
// waiting on Slack is newer than the fetch's answer, so it stays.
func TestSelfStatusFetchKeepsNewerEvent(t *testing.T) {
	srv, started, release := selfStatusServer(t, "away")
	h, statuses := selfStatusHandler(t, srv)

	h.OnConnect()
	<-started
	h.OnSelfPresenceChange("active")
	<-statuses
	release(1)
	fetched := <-statuses

	if fetched.Presence != "active" {
		t.Fatalf("fetch reported presence %q, want the event's active", fetched.Presence)
	}
	if got := h.wsCtx.Self.Get().Presence; got != "active" {
		t.Fatalf("stored presence = %q, want the event's active", got)
	}
}
