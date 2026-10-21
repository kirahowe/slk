package slackclient

import (
	"context"
	"errors"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"pgregory.net/rapid"
)

func TestBackoffSequence(t *testing.T) {
	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	expected := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
		30 * time.Second,
		30 * time.Second,
	}

	for i, want := range expected {
		if backoff != want {
			t.Errorf("step %d: expected %v, got %v", i, want, backoff)
		}
		backoff = nextBackoff(backoff, maxBackoff)
	}
}

// fakeSocket is a gateway whose dials each take dialTime, then fail or
// keep a connection up for life before it drops.
type fakeSocket struct {
	dialTime    time.Duration
	life        time.Duration
	fail        bool // every dial fails
	rateLimited bool // the gateway rate-limits every connection
	dials       []time.Time
	last        *wsSession
	stop        func() // called on the nth dial
	n           int
}

func (f *fakeSocket) StartWebSocket(context.Context, EventHandler) (*wsSession, error) {
	f.dials = append(f.dials, time.Now())
	if len(f.dials) == f.n {
		f.stop()
	}
	time.Sleep(f.dialTime)
	if f.fail {
		return nil, errors.New("dial failed")
	}
	s := &wsSession{done: make(chan struct{})}
	s.rateLimited.Store(f.rateLimited)
	f.last = s
	go func() {
		time.Sleep(f.life)
		close(s.done)
	}()
	return s, nil
}
func (f *fakeSocket) StopWebSocket() error { return nil }

// redialGaps runs a ConnectionManager without jitter over f until its
// nth dial, and returns the time from each drop (or failed dial) to the
// next dial.
func redialGaps(t *testing.T, f *fakeSocket, dials int) []time.Duration {
	var gaps []time.Duration
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		f.stop, f.n = cancel, dials
		cm := newConnectionManager(f, &mockEventHandler{}, func(time.Duration) time.Duration { return 0 })
		cm.Run(ctx)
		if f.last != nil {
			<-f.last.done // the last connection's drop, so the bubble can end
		}
		up := f.dialTime
		if !f.fail {
			up += f.life
		}
		for i := 1; i < len(f.dials); i++ {
			gaps = append(gaps, f.dials[i].Sub(f.dials[i-1])-up)
		}
	})
	return gaps
}

// seconds turns whole seconds into durations.
func seconds(ns ...time.Duration) []time.Duration {
	out := make([]time.Duration, len(ns))
	for i, n := range ns {
		out[i] = n * time.Second
	}
	return out
}

// A connection the gateway drops seconds after accepting it (its answer
// to a rate-limited user) must back off, not redial every second.
func TestShortLivedConnectionsBackOff(t *testing.T) {
	got := redialGaps(t, &fakeSocket{life: 5 * time.Second}, 8)
	if want := seconds(1, 2, 4, 8, 16, 30, 30); !slices.Equal(got, want) {
		t.Fatalf("redial gaps = %v, want %v", got, want)
	}
}

// Dials that fail outright back off the same way.
func TestFailedDialsBackOff(t *testing.T) {
	got := redialGaps(t, &fakeSocket{dialTime: 3 * time.Second, fail: true}, 8)
	if want := seconds(1, 2, 4, 8, 16, 30, 30); !slices.Equal(got, want) {
		t.Fatalf("redial gaps = %v, want %v", got, want)
	}
}

// A connection that stayed up past healthyAfter resets the backoff, so a
// rare drop of a working socket reconnects within a second.
func TestHealthyConnectionResetsBackoff(t *testing.T) {
	for _, gap := range redialGaps(t, &fakeSocket{life: healthyAfter}, 5) {
		if gap != minBackoff {
			t.Fatalf("redial gap after a healthy connection = %v, want %v", gap, minBackoff)
		}
	}
}

// Time spent dialing is not time connected: a slow dial followed by a
// short connection still backs off.
func TestSlowDialDoesNotCountTowardHealth(t *testing.T) {
	f := &fakeSocket{dialTime: 10 * time.Second, life: healthyAfter - time.Second}
	got := redialGaps(t, f, 5)
	if want := seconds(1, 2, 4, 8); !slices.Equal(got, want) {
		t.Fatalf("redial gaps = %v, want %v", got, want)
	}
}

// After the gateway rate-limits a connection, the next dial waits the
// longest backoff at once instead of climbing to it from a second.
func TestRateLimitedConnectionWaitsMaxBackoff(t *testing.T) {
	got := redialGaps(t, &fakeSocket{life: 5 * time.Second, rateLimited: true}, 4)
	if want := seconds(30, 30, 30); !slices.Equal(got, want) {
		t.Fatalf("redial gaps = %v, want %v", got, want)
	}
}

// A Stop that comes before Run (shutdown while a workspace is still
// connecting) keeps Run from dialing at all.
func TestStopBeforeRunReturnsAtOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := &fakeSocket{life: time.Hour, n: 1}
		cm := newConnectionManager(f, &mockEventHandler{}, quarterJitter)
		f.stop = func() {
			t.Error("Run dialed after Stop")
			cm.Stop()
		}
		cm.Stop()
		cm.Run(t.Context())
		if f.last != nil {
			<-f.last.done
		}
	})
}

// The jitter never more than a quarter of the backoff it is added to.
func TestQuarterJitterBound(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		d := time.Duration(rapid.Int64Range(0, int64(time.Hour)).Draw(t, "d"))
		if j := quarterJitter(d); j < 0 || j > d/4 {
			t.Fatalf("quarterJitter(%v) = %v", d, j)
		}
	})
}
