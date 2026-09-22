package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// dialWebSocket dials a WebSocket endpoint with optional custom headers and
// starts a goroutine that closes the connection when ctx is done.
// On handshake failure the HTTP status and body (truncated) are included so
// auth/path errors are not reduced to a bare "bad handshake".
func dialWebSocket(ctx context.Context, url string, headers http.Header) (*websocket.Conn, error) {
	dialer := websocket.Dialer{
		HandshakeTimeout: 30 * time.Second,
	}
	conn, resp, err := dialer.Dial(url, headers)
	if err != nil {
		return nil, wrapHandshakeError(err, resp)
	}

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	return conn, nil
}

func wrapHandshakeError(err error, resp *http.Response) error {
	if resp == nil {
		return fmt.Errorf("websocket dial: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	status := resp.Status
	body := ""
	if resp.Body != nil {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		body = strings.TrimSpace(string(raw))
	}
	if body == "" {
		return fmt.Errorf("websocket handshake failed (%s): %w", status, err)
	}
	return fmt.Errorf("websocket handshake failed (%s): %s", status, body)
}

// closeWebSocket safely closes a WebSocket connection, ignoring nil.
func closeWebSocket(conn *websocket.Conn) {
	if conn != nil {
		_ = conn.Close()
	}
}
