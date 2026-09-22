package provider

import (
	"context"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// dialWebSocket dials a WebSocket endpoint with optional custom headers and
// starts a goroutine that closes the connection when ctx is done.
func dialWebSocket(ctx context.Context, url string, headers http.Header) (*websocket.Conn, error) {
	dialer := websocket.Dialer{
		HandshakeTimeout: 30 * time.Second,
	}
	conn, _, err := dialer.Dial(url, headers)
	if err != nil {
		return nil, err
	}

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	return conn, nil
}

// closeWebSocket safely closes a WebSocket connection, ignoring nil.
func closeWebSocket(conn *websocket.Conn) {
	if conn != nil {
		_ = conn.Close()
	}
}
