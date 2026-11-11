package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync/atomic"
	"testing"
)

// waitResolverIdle blocks until no lookup for userID holds the
// resolver's inflight slot: the Request it follows has run to the end,
// success or failure. No deadline; a hang is caught by go test -timeout.
func waitResolverIdle(r *userResolver, userID string) {
	for {
		if _, busy := r.inflight.Load(userID); !busy {
			return
		}
		runtime.Gosched()
	}
}

// failingUsersInfo answers every users.info with status and body and
// counts the calls.
func failingUsersInfo(t *testing.T, status int, body string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if status == http.StatusTooManyRequests {
			w.Header().Set("Retry-After", "1")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

// A user Slack says does not exist (user_not_found) or cannot be shown
// to this user (user_not_visible) is looked up once per session: each
// read of a channel that mentions them calls Request again, and every
// one of those would otherwise be a users.info call.
func TestUserResolver_MissingUserIsLookedUpOnce(t *testing.T) {
	const reads = 5
	for _, code := range []string{"user_not_found", "user_not_visible"} {
		t.Run(code, func(t *testing.T) {
			t.Parallel()
			srv, calls := failingUsersInfo(t, http.StatusOK, `{"ok":false,"error":"`+code+`"}`)
			r := newUserResolver("T1", newTestClient(t, srv), newTestDB(t), nil, nil, nil, nil)
			for range reads {
				r.Request("U404")
				waitResolverIdle(r, "U404")
			}
			t.Logf("%s: %d users.info calls over %d reads", code, calls.Load(), reads)
			if got := calls.Load(); got != 1 {
				t.Errorf("users.info calls = %d over %d reads; want 1", got, reads)
			}
		})
	}
}

// Through the edge batch the same holds, and the batch itself is not
// repeated: edge leaves a missing user out of its answer, the per-user
// fallback learns user_not_found, and later Requests stop before
// queueing.
func TestUserResolver_MissingUserIsLookedUpOnceThroughEdge(t *testing.T) {
	const reads = 3
	srv, calls := failingUsersInfo(t, http.StatusOK, `{"ok":false,"error":"user_not_found"}`)
	batcher := &fakeBatcher{}
	r := newUserResolver("T1", newTestClient(t, srv), newTestDB(t), nil, nil, batcher, nil)
	for range reads {
		r.Request("U404")
		waitResolverIdle(r, "U404")
	}
	t.Logf("edge: %d batches and %d users.info calls over %d reads", len(batcher.calls()), calls.Load(), reads)
	if got := len(batcher.calls()); got != 1 {
		t.Errorf("edge batches = %d over %d reads; want 1", got, reads)
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("users.info calls = %d over %d reads; want 1", got, reads)
	}
}

// The direct path reconnect uses learns and honours the same record.
func TestUserResolver_MissingUserSkipsDirectLookup(t *testing.T) {
	srv, calls := failingUsersInfo(t, http.StatusOK, `{"ok":false,"error":"user_not_found"}`)
	r := newUserResolver("T1", newTestClient(t, srv), newTestDB(t), nil, nil, nil, nil)
	r.resolveOneContext(context.Background(), "U404")
	r.resolveOneContext(context.Background(), "U404")
	r.Request("U404")
	waitResolverIdle(r, "U404")
	if got := calls.Load(); got != 1 {
		t.Errorf("users.info calls = %d; want 1", got)
	}
}

// A failure that says nothing about the user (rate limit, server
// error, a refusal that is not about the ID) leaves them retryable:
// every read asks again.
func TestUserResolver_TransientFailureIsRetried(t *testing.T) {
	const reads = 3
	cases := []struct {
		name   string
		status int
		body   string
	}{
		{"429", http.StatusTooManyRequests, `{"ok":false,"error":"ratelimited"}`},
		{"500", http.StatusInternalServerError, `oops`},
		{"ratelimited body", http.StatusOK, `{"ok":false,"error":"ratelimited"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv, calls := failingUsersInfo(t, tc.status, tc.body)
			r := newUserResolver("T1", newTestClient(t, srv), newTestDB(t), nil, nil, nil, nil)
			for range reads {
				r.Request("U001")
				waitResolverIdle(r, "U001")
			}
			t.Logf("%s: %d users.info calls over %d reads", tc.name, calls.Load(), reads)
			if got := calls.Load(); got != reads {
				t.Errorf("users.info calls = %d over %d reads; want %d", got, reads, reads)
			}
		})
	}
}

// A lookup cut short by its context is not an answer about the user:
// the next Request still asks.
func TestUserResolver_CancelledLookupIsRetried(t *testing.T) {
	srv, calls := failingUsersInfo(t, http.StatusOK, `{"ok":false,"error":"user_not_found"}`)
	r := newUserResolver("T1", newTestClient(t, srv), newTestDB(t), nil, nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r.resolveOneContext(ctx, "U404")
	r.Request("U404")
	waitResolverIdle(r, "U404")
	if got := calls.Load(); got != 1 {
		t.Errorf("users.info calls = %d; want 1, from the Request after the cancelled lookup", got)
	}
}

// The record is per workspace: a user missing in one workspace's
// resolver is still looked up by another's.
func TestUserResolver_MissingUserIsPerWorkspace(t *testing.T) {
	srv, calls := failingUsersInfo(t, http.StatusOK, `{"ok":false,"error":"user_not_found"}`)
	client := newTestClient(t, srv)
	db := newTestDB(t)
	r1 := newUserResolver("T1", client, db, nil, nil, nil, nil)
	r2 := newUserResolver("T2", client, db, nil, nil, nil, nil)
	r1.Request("U404")
	waitResolverIdle(r1, "U404")
	r2.Request("U404")
	waitResolverIdle(r2, "U404")
	if got := calls.Load(); got != 2 {
		t.Errorf("users.info calls = %d; want 2, one per workspace", got)
	}
}

// Reconnect's batch leaves out an ID already known to be missing.
func TestUserResolver_ResolveNowSkipsMissingUser(t *testing.T) {
	srv, _ := failingUsersInfo(t, http.StatusOK, `{"ok":false,"error":"user_not_found"}`)
	batcher := &fakeBatcher{}
	r := newUserResolver("T1", newTestClient(t, srv), newTestDB(t), nil, nil, batcher, nil)
	r.resolveOneContext(context.Background(), "U404")
	r.ResolveNowContext(context.Background(), []string{"U404", "U001"})
	sent := batcher.calls()
	if len(sent) != 1 || len(sent[0]) != 1 {
		t.Fatalf("edge batches = %v; want one batch of U001 alone", sent)
	}
	if _, ok := sent[0]["U001"]; !ok {
		t.Errorf("edge batches = %v; want one batch of U001 alone", sent)
	}
}
