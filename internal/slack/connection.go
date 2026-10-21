package slackclient

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/gammons/slk/internal/debuglog"
)

// socket is the part of Client the ConnectionManager drives, so tests
// can run the reconnect loop against a fake connection.
type socket interface {
	StartWebSocket(handler EventHandler) error
	WsDone() <-chan struct{}
	StopWebSocket() error
}

const (
	minBackoff = 1 * time.Second
	maxBackoff = 30 * time.Second

	// healthyAfter is how long a connection must stay up before the
	// backoff resets. A dial that succeeds is not enough: when the
	// gateway rate-limits a user it accepts the upgrade, sends
	// {"type":"error","error":{"msg":"Ratelimited"}} and closes the
	// socket about five seconds later. Resetting on the dial retried
	// such a socket every six seconds indefinitely, which kept it rate
	// limited (2026-10-21). With keepAlive pinging, a healthy socket
	// stays up far longer than this.
	healthyAfter = 30 * time.Second
)

// ConnectionManager manages the WebSocket connection lifecycle with
// automatic reconnection using exponential backoff.
type ConnectionManager struct {
	client  socket
	handler EventHandler
	cancel  context.CancelFunc

	// jitter returns the random extra wait added to a backoff of d, so
	// workspaces that dropped together do not redial together.
	jitter func(d time.Duration) time.Duration
}

// NewConnectionManager creates a new connection manager.
func NewConnectionManager(client *Client, handler EventHandler) *ConnectionManager {
	return &ConnectionManager{
		client:  client,
		handler: handler,
		jitter:  quarterJitter,
	}
}

// quarterJitter is a random duration in [0, d/4).
func quarterJitter(d time.Duration) time.Duration {
	return rand.N(d/4 + 1)
}

// Run starts the connection loop. It connects, waits for disconnect,
// and reconnects with exponential backoff. Blocks until ctx is cancelled.
// The backoff resets only after a connection has stayed up for
// healthyAfter.
func (cm *ConnectionManager) Run(ctx context.Context) {
	ctx, cm.cancel = context.WithCancel(ctx)

	backoff := minBackoff
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		started := time.Now()
		if err := cm.client.StartWebSocket(cm.handler); err != nil {
			debuglog.WS("websocket dial failed: %v", err)
			cm.handler.OnDisconnect()
		} else {
			select {
			case <-ctx.Done():
				cm.client.StopWebSocket()
				return
			case <-cm.client.WsDone():
			}
			if time.Since(started) >= healthyAfter {
				backoff = minBackoff
			}
		}

		wait := backoff + cm.jitter(backoff)
		debuglog.WS("reconnecting in %v", wait)
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		backoff = nextBackoff(backoff, maxBackoff)
	}
}

// Stop cancels the connection loop and closes the WebSocket.
func (cm *ConnectionManager) Stop() {
	if cm.cancel != nil {
		cm.cancel()
	}
	cm.client.StopWebSocket()
}

func nextBackoff(current, max time.Duration) time.Duration {
	next := current * 2
	if next > max {
		return max
	}
	return next
}
