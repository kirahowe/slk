package slackclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// presenceFrames starts a gateway that hands every frame it reads to the
// returned channel, and a Client connected to it.
func presenceFrames(t *testing.T) (*Client, <-chan []byte) {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	got := make(chan []byte, 16)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		for {
			_, frame, err := conn.ReadMessage()
			if err != nil {
				return // the client closed
			}
			got <- frame
		}
	}))
	t.Cleanup(srv.Close)
	c := &Client{wsBaseURL: "ws://" + srv.Listener.Addr().String(), pingInterval: time.Hour}
	if _, err := c.StartWebSocket(t.Context(), &mockEventHandler{}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = c.StopWebSocket()
		<-c.WsDone()
	})
	return c, got
}

// presenceFrameOf decodes one presence frame, failing on one over the
// gateway's limit.
func presenceFrameOf(t *testing.T, frame []byte) (string, []string) {
	t.Helper()
	if len(frame) > maxClientFrame {
		t.Fatalf("sent a %d-byte frame", len(frame))
	}
	var f struct {
		Type string
		IDs  []string
	}
	if err := json.Unmarshal(frame, &f); err != nil {
		t.Fatal(err)
	}
	return f.Type, f.IDs
}

// fullSubscription returns IDs whose presence_sub frame is exactly
// maxClientFrame bytes, so every one is subscribed but the same IDs'
// presence_query frame, two bytes longer, does not fit.
func fullSubscription(t *testing.T) []string {
	t.Helper()
	var ids []string
	for i := 0; ; i++ {
		next := append(slices.Clone(ids), fmt.Sprintf("U%010d", i))
		if b, _ := json.Marshal(presenceFrame(presenceSubType, next)); len(b) > maxClientFrame {
			break
		}
		ids = next
	}
	for {
		b, _ := json.Marshal(presenceFrame(presenceSubType, ids))
		if len(b) == maxClientFrame {
			return ids
		}
		ids[len(ids)-1] += "X"
	}
}

// Every subscribed ID is queried, even when the query cannot fit in the
// one frame the subscription did: the query goes out in order, split
// across frames that each fit the gateway's limit.
func TestQueryPresenceQueriesEverySubscribedID(t *testing.T) {
	c, got := presenceFrames(t)
	ids := fullSubscription(t)

	n, err := c.SubscribePresence(ids)
	if err != nil {
		t.Fatal(err)
	}
	if n != len(ids) {
		t.Fatalf("subscribed %d of %d ids; the fixture should fill one frame exactly", n, len(ids))
	}
	if typ, sub := presenceFrameOf(t, <-got); typ != presenceSubType || !slices.Equal(sub, ids) {
		t.Fatalf("first frame is %s for %d ids, want presence_sub for all %d", typ, len(sub), len(ids))
	}

	if err := c.QueryPresence(ids[:n]); err != nil {
		t.Fatal(err)
	}
	var queried []string
	frames := 0
	for len(queried) < n {
		typ, q := presenceFrameOf(t, <-got)
		if typ != presenceQueryType {
			t.Fatalf("frame type %q, want presence_query", typ)
		}
		queried = append(queried, q...)
		frames++
	}
	if !slices.Equal(queried, ids) {
		t.Fatalf("queried %d ids, want the %d subscribed, in order", len(queried), n)
	}
	if frames != 2 {
		t.Fatalf("query took %d frames, want 2", frames)
	}
}

// An ID no frame can hold fails the query rather than looping or sending
// a frame that would close the socket.
func TestQueryPresenceRefusesAnIDTooLongForAFrame(t *testing.T) {
	c, _ := presenceFrames(t)
	err := c.QueryPresence([]string{strings.Repeat("U", maxClientFrame)})
	if !errors.Is(err, errFrameTooLarge) {
		t.Fatalf("QueryPresence(huge id) = %v, want errFrameTooLarge", err)
	}
}

func TestQueryPresenceReturnsErrorWhenNotConnected(t *testing.T) {
	if err := (&Client{}).QueryPresence([]string{"U1"}); !errors.Is(err, errNotConnected) {
		t.Fatalf("QueryPresence() = %v, want errNotConnected", err)
	}
}
