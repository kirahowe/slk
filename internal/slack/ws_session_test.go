package slackclient

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/gorilla/websocket"
)

// pipeGateway makes c dial an in-memory gateway: each dial gets one end
// of a net.Pipe and serve gets the other, with the dial's number (from 1).
// Nothing touches the network, so the connection runs in a synctest
// bubble with its deadlines on the fake clock.
func pipeGateway(c *Client, serve func(n int, conn net.Conn)) {
	var dials int
	c.wsBaseURL = "ws://gateway.test"
	c.wsNetDial = func(context.Context, string, string) (net.Conn, error) {
		dials++
		client, server := net.Pipe()
		go serve(dials, server)
		return client, nil
	}
}

// acceptUpgrade answers the client's upgrade request on conn, by hand,
// with the 101 response gorilla checks for.
func acceptUpgrade(conn net.Conn) error {
	req, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		return err
	}
	sum := sha1.Sum([]byte(req.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	_, err = fmt.Fprintf(conn, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n",
		base64.StdEncoding.EncodeToString(sum[:]))
	return err
}

// serverFrame is a final, unmasked frame of the given opcode, as the
// gateway sends it. Payloads stay under 126 bytes.
func serverFrame(opcode byte, payload string) []byte {
	return append([]byte{0x80 | opcode, byte(len(payload))}, payload...)
}

// A write to a peer that stopped reading fails once writeTimeout passes
// instead of blocking forever, and while it is stuck it holds up neither
// the reader answering a ping nor the writes of the connection that
// replaced it.
func TestWedgedWriteTimesOutWithoutBlockingTheNextSession(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ping, finish := make(chan struct{}), make(chan struct{})
		c := &Client{pingInterval: time.Hour}
		pipeGateway(c, func(n int, conn net.Conn) {
			defer conn.Close()
			if err := acceptUpgrade(conn); err != nil {
				t.Errorf("upgrade: %v", err)
				return
			}
			if n > 1 {
				io.Copy(io.Discard, conn)
				return
			}
			// The first gateway never reads again, so the client's next
			// write blocks. Halfway through that write's deadline it
			// pings, which the client can only answer by writing too.
			<-ping
			time.Sleep(writeTimeout / 2)
			if _, err := conn.Write(serverFrame(websocket.PingMessage, "")); err != nil {
				t.Errorf("ping: %v", err)
			}
			<-finish
		})
		defer close(finish)

		first, err := c.StartWebSocket(t.Context(), &mockEventHandler{})
		if err != nil {
			t.Fatal(err)
		}
		wedged := time.Now()
		typed := make(chan error)
		go func() { typed <- c.SendTyping("C1") }()
		synctest.Wait()
		close(ping)
		synctest.Wait()

		// The reconnect: the new connection's write must not queue
		// behind the stuck one.
		second, err := c.StartWebSocket(t.Context(), &mockEventHandler{})
		if err != nil {
			t.Fatal(err)
		}
		if err := c.SendTyping("C2"); err != nil {
			t.Fatalf("SendTyping on the new connection: %v", err)
		}
		if d := time.Since(wedged); d != 0 {
			t.Fatalf("the new connection's write waited %v for the old one", d)
		}

		if err := <-typed; err == nil {
			t.Fatal("the write to a peer that never reads succeeded")
		}
		if d := time.Since(wedged); d != writeTimeout {
			t.Fatalf("the stuck write failed after %v, want %v", d, writeTimeout)
		}
		// The failed write closed the old connection, and its reader,
		// which was waiting to answer the ping, has stopped.
		<-first.done
		if d := time.Since(wedged); d != writeTimeout {
			t.Fatalf("the old connection ended after %v, want %v", d, writeTimeout)
		}

		c.StopWebSocket()
		<-second.done
	})
}

// A reader that stops on a read error closes its connection rather than
// leaving the socket open for the peer to give up on.
func TestReaderClosesItsConnectionOnReadError(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		closed := make(chan struct{})
		c := &Client{pingInterval: time.Hour}
		pipeGateway(c, func(_ int, conn net.Conn) {
			defer conn.Close()
			if err := acceptUpgrade(conn); err != nil {
				t.Errorf("upgrade: %v", err)
				return
			}
			// Opcode 3 is reserved, so the client's read fails.
			if _, err := conn.Write(serverFrame(3, "")); err != nil {
				t.Errorf("bad frame: %v", err)
			}
			io.Copy(io.Discard, conn) // until the client closes its end
			close(closed)
		})
		s, err := c.StartWebSocket(t.Context(), &mockEventHandler{})
		if err != nil {
			t.Fatal(err)
		}
		<-s.done
		<-closed
	})
}

// helloHandler hands every hello to the test, blocking the dispatcher
// until the test takes it.
type helloHandler struct {
	mockEventHandler
	hellos chan struct{}
}

func (h *helloHandler) OnConnect() { h.hellos <- struct{}{} }

// The read deadline times the peer, not the dispatcher: a handler call
// that takes longer than wsTimeout does not make the next read fail.
func TestSlowDispatchDoesNotExpireTheReadDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		more := make(chan struct{})
		c := &Client{pingInterval: time.Hour}
		pipeGateway(c, func(_ int, conn net.Conn) {
			defer conn.Close()
			if err := acceptUpgrade(conn); err != nil {
				t.Errorf("upgrade: %v", err)
				return
			}
			hello := serverFrame(websocket.TextMessage, `{"type":"hello"}`)
			conn.Write(hello)
			conn.Write(hello) // read, then held until the dispatcher is free
			<-more
			conn.Write(hello)
			io.Copy(io.Discard, conn)
		})
		h := &helloHandler{hellos: make(chan struct{})}
		s, err := c.StartWebSocket(t.Context(), h)
		if err != nil {
			t.Fatal(err)
		}
		synctest.Wait() // the dispatcher is in the first OnConnect
		time.Sleep(2 * wsTimeout)
		<-h.hellos
		close(more)
		<-h.hellos
		<-h.hellos // the third hello arrived: the connection survived
		c.StopWebSocket()
		<-s.done
	})
}

// The dispatcher marks a connection the gateway rate-limited, which is
// how the ConnectionManager learns to back off.
func TestRatelimitedErrorFrameMarksTheSession(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := &Client{pingInterval: time.Hour}
		pipeGateway(c, func(_ int, conn net.Conn) {
			defer conn.Close()
			if err := acceptUpgrade(conn); err != nil {
				t.Errorf("upgrade: %v", err)
				return
			}
			conn.Write(serverFrame(websocket.TextMessage, `{"type":"error","error":{"msg":"Ratelimited","code":17}}`))
		})
		s, err := c.StartWebSocket(t.Context(), &mockEventHandler{})
		if err != nil {
			t.Fatal(err)
		}
		<-s.done
		if !s.rateLimited.Load() {
			t.Fatal("a Ratelimited error frame did not mark the session")
		}
	})
}

// Only the Ratelimited error counts as rate limiting.
func TestDispatchReportsOnlyRatelimited(t *testing.T) {
	for frame, want := range map[string]bool{
		`{"type":"error","error":{"msg":"Ratelimited","code":17}}`:   true,
		`{"type":"error","error":{"msg":"something else","code":1}}`: false,
		`{"type":"hello"}`: false,
	} {
		if got := dispatchWebSocketEvent([]byte(frame), &mockEventHandler{}); got != want {
			t.Errorf("dispatchWebSocketEvent(%s) = %v, want %v", frame, got, want)
		}
	}
}

// Stop returns Run at once while the upgrade is stalled, instead of
// waiting for the handshake timeout or forever.
func TestStopInterruptsAStalledUpgrade(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := &Client{}
		pipeGateway(c, func(_ int, conn net.Conn) {
			defer conn.Close()
			io.Copy(io.Discard, conn) // reads the request, never answers
		})
		cm := NewConnectionManager(c, &mockEventHandler{})
		returned := make(chan struct{})
		go func() {
			cm.Run(t.Context())
			close(returned)
		}()
		synctest.Wait()
		stopped := time.Now()
		cm.Stop()
		<-returned
		if d := time.Since(stopped); d != 0 {
			t.Fatalf("Run returned %v after Stop", d)
		}
	})
}

// The UI's typing, the presence subscription, shutdown's Stop and WsDone
// all run on their own goroutines while the manager replaces the
// connection. The race detector checks they share it safely.
func TestWebSocketCallsAcrossReconnectsAreRaceFree(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade: %v", err)
			return
		}
		defer conn.Close()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}))
	defer srv.Close()
	c := &Client{wsBaseURL: "ws://" + srv.Listener.Addr().String(), pingInterval: time.Millisecond}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for _, call := range []func(){
		func() { c.SendTyping("C1") },
		func() { c.SubscribePresence([]string{"U1", "U2"}) },
		func() { _ = c.WsDone() },
		func() { c.StopWebSocket() },
	} {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
					call()
				}
			}
		})
	}
	for range 20 {
		s, err := c.StartWebSocket(t.Context(), &mockEventHandler{})
		if err != nil {
			t.Fatal(err)
		}
		c.StopWebSocket()
		<-s.done
	}
	close(stop)
	wg.Wait()
}
