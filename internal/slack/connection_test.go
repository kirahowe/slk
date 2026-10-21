package slackclient

import (
	"context"
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

// fakeSocket is a connection that each dial keeps up for life, then drops.
type fakeSocket struct {
	life  time.Duration
	dials []time.Time
	done  chan struct{}
	stop  func() // called on the nth dial
	n     int
}

func (f *fakeSocket) StartWebSocket(EventHandler) error {
	f.dials = append(f.dials, time.Now())
	if len(f.dials) == f.n {
		f.stop()
	}
	done := make(chan struct{})
	f.done = done
	go func() {
		time.Sleep(f.life)
		close(done)
	}()
	return nil
}
func (f *fakeSocket) WsDone() <-chan struct{} { return f.done }
func (f *fakeSocket) StopWebSocket() error    { return nil }

// redialGaps runs a ConnectionManager without jitter over a socket whose
// every connection lasts life, and returns the time from each drop to
// the next dial.
func redialGaps(t *testing.T, life time.Duration, dials int) []time.Duration {
	var gaps []time.Duration
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		f := &fakeSocket{life: life, stop: cancel, n: dials}
		cm := &ConnectionManager{client: f, handler: &mockEventHandler{},
			jitter: func(time.Duration) time.Duration { return 0 }}
		cm.Run(ctx)
		<-f.done // the last connection's drop, so the bubble can end
		for i := 1; i < len(f.dials); i++ {
			gaps = append(gaps, f.dials[i].Sub(f.dials[i-1])-life)
		}
	})
	return gaps
}

// A connection the gateway drops seconds after accepting it (its answer
// to a rate-limited user) must back off, not redial every second.
func TestShortLivedConnectionsBackOff(t *testing.T) {
	got := redialGaps(t, 5*time.Second, 8)
	want := []time.Duration{1, 2, 4, 8, 16, 30, 30}
	for i := range want {
		want[i] *= time.Second
	}
	if !slices.Equal(got, want) {
		t.Fatalf("redial gaps = %v, want %v", got, want)
	}
}

// A connection that stayed up past healthyAfter resets the backoff, so a
// rare drop of a working socket reconnects within a second.
func TestHealthyConnectionResetsBackoff(t *testing.T) {
	for _, gap := range redialGaps(t, healthyAfter, 5) {
		if gap != minBackoff {
			t.Fatalf("redial gap after a healthy connection = %v, want %v", gap, minBackoff)
		}
	}
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
