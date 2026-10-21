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

type pendingEventTestHandler struct {
	mockEventHandler
	pending         chan struct{}
	connectStarted  chan struct{}
	connectRelease  chan struct{}
	connectFinished chan struct{}
	repaired        chan struct{}
	serialized      chan bool
}

func (h *pendingEventTestHandler) PendingEvents() <-chan struct{} { return h.pending }
func (h *pendingEventTestHandler) OnPendingEvents() {
	select {
	case <-h.connectFinished:
		h.serialized <- true
	default:
		h.serialized <- false
	}
	close(h.repaired)
}
func (h *pendingEventTestHandler) OnConnect() {
	close(h.connectStarted)
	<-h.connectRelease
	close(h.connectFinished)
}

func TestStartWebSocketDispatchesPendingEventsWithoutIncomingFrames(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	stop := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"hello"}`)); err != nil {
			t.Errorf("hello: %v", err)
			return
		}
		// No more frames: a deferred repair must wake the dispatch loop itself.
		<-stop
	}))
	defer srv.Close()
	defer close(stop)
	c := &Client{wsBaseURL: "ws://" + srv.Listener.Addr().String()}
	h := &pendingEventTestHandler{
		pending: make(chan struct{}, 1), connectStarted: make(chan struct{}),
		connectRelease: make(chan struct{}), connectFinished: make(chan struct{}),
		repaired: make(chan struct{}), serialized: make(chan bool, 1),
	}
	if err := c.StartWebSocket(h); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = c.StopWebSocket()
		<-c.WsDone()
	}()
	defer close(h.connectRelease)
	<-h.connectStarted // hello dispatched
	h.pending <- struct{}{}
	h.connectRelease <- struct{}{}
	<-h.repaired // the pending event was dispatched on the idle connection
	if !<-h.serialized {
		t.Fatal("pending event was not serialized with OnConnect")
	}
}

// The client pings an otherwise silent socket with Slack's application
// ping, whose pong is what keeps the read deadline from closing it.
func TestStartWebSocketPingsAnIdleSocket(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	got := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		_, frame, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("read: %v", err)
		}
		got <- string(frame)
		_, _, _ = conn.ReadMessage() // until the client closes
	}))
	defer srv.Close()
	c := &Client{wsBaseURL: "ws://" + srv.Listener.Addr().String(), pingInterval: time.Millisecond}
	if err := c.StartWebSocket(&mockEventHandler{}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = c.StopWebSocket()
		<-c.WsDone()
	}()
	if frame := <-got; frame != `{"id":1,"type":"ping"}` {
		t.Fatalf("first frame = %q, want the first ping", frame)
	}
}

// A subscription too big for one frame reaches the gateway trimmed to the
// IDs that fit, in order, instead of as a frame that closes the socket.
func TestSubscribePresenceSendsOnlyWhatFits(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	got := make(chan []byte, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		_, frame, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("read: %v", err)
		}
		got <- frame
		_, _, _ = conn.ReadMessage() // until the client closes
	}))
	defer srv.Close()
	c := &Client{wsBaseURL: "ws://" + srv.Listener.Addr().String(), pingInterval: time.Hour}
	if err := c.StartWebSocket(&mockEventHandler{}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = c.StopWebSocket()
		<-c.WsDone()
	}()
	ids := make([]string, 400)
	for i := range ids {
		ids[i] = fmt.Sprintf("U%010d", i)
	}
	n, err := c.SubscribePresence(ids)
	if err != nil {
		t.Fatal(err)
	}
	frame := <-got
	if len(frame) > maxClientFrame {
		t.Fatalf("sent a %d-byte frame", len(frame))
	}
	var sub struct{ IDs []string }
	if err := json.Unmarshal(frame, &sub); err != nil {
		t.Fatal(err)
	}
	if n == 0 || n == len(ids) || !slices.Equal(sub.IDs, ids[:n]) {
		t.Fatalf("subscribed %d ids (reported %d), want the first %d of %d", len(sub.IDs), n, n, len(ids))
	}
}

// writeJSON refuses a frame over the limit instead of sending it.
func TestWriteJSONRefusesAnOversizedFrame(t *testing.T) {
	big := map[string]string{"text": strings.Repeat("x", maxClientFrame)}
	if err := writeJSON(nil, big); !errors.Is(err, errFrameTooLarge) {
		t.Fatalf("writeJSON(oversized) = %v, want errFrameTooLarge", err)
	}
}
