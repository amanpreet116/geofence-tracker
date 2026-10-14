package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func startHub(t *testing.T, origins ...string) (*Hub, *httptest.Server, context.CancelFunc) {
	t.Helper()
	if len(origins) == 0 {
		origins = []string{"*"}
	}
	hub := NewHub(origins, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ctx, cancel := context.WithCancel(context.Background())
	go hub.Run(ctx)
	srv := httptest.NewServer(http.HandlerFunc(hub.ServeWS))
	t.Cleanup(func() { cancel(); srv.Close() })
	return hub, srv, cancel
}

func wsURL(srv *httptest.Server) string { return "ws" + strings.TrimPrefix(srv.URL, "http") }

// dial connects and consumes the hello message.
func dial(t *testing.T, srv *httptest.Server, header http.Header) *websocket.Conn {
	t.Helper()
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL(srv), header)
	if err != nil {
		t.Fatalf("dial: %v (resp %v)", err, resp)
	}
	t.Cleanup(func() { _ = conn.Close() })
	var hello map[string]any
	readJSON(t, conn, &hello)
	if hello["type"] != "connected" || hello["client_id"] == "" {
		t.Fatalf("hello = %v", hello)
	}
	return conn
}

func readJSON(t *testing.T, conn *websocket.Conn, v any) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("decode %q: %v", data, err)
	}
}

func waitForClients(t *testing.T, hub *Hub, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for hub.ClientCount() != n {
		if time.Now().After(deadline) {
			t.Fatalf("client count = %d, want %d", hub.ClientCount(), n)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestBroadcastReachesAllClients(t *testing.T) {
	hub, srv, _ := startHub(t)
	a, b := dial(t, srv, nil), dial(t, srv, nil)
	waitForClients(t, hub, 2)

	hub.Broadcast([]byte(`{"event_id":"evt_1"}`))
	for name, c := range map[string]*websocket.Conn{"a": a, "b": b} {
		var got map[string]any
		readJSON(t, c, &got)
		if got["event_id"] != "evt_1" {
			t.Errorf("client %s got %v", name, got)
		}
	}

	// Disconnecting one client does not affect the other.
	_ = a.Close()
	waitForClients(t, hub, 1)
	hub.Broadcast([]byte(`{"event_id":"evt_2"}`))
	var got map[string]any
	readJSON(t, b, &got)
	if got["event_id"] != "evt_2" {
		t.Errorf("remaining client got %v", got)
	}
}

func TestClientPingGetsPong(t *testing.T) {
	_, srv, _ := startHub(t)
	c := dial(t, srv, nil)

	for _, msg := range []string{`not json`, `{"type":"subscribe"}`, `{"type":"ping"}`} {
		if err := c.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
			t.Fatal(err)
		}
	}
	// Junk and unknown types are ignored; only the ping is answered.
	var got map[string]any
	readJSON(t, c, &got)
	if got["type"] != "pong" {
		t.Errorf("got %v, want pong", got)
	}
}

func TestOriginCheck(t *testing.T) {
	_, srv, _ := startHub(t, "http://app.example")

	tests := []struct {
		name   string
		origin string
		wantOK bool
	}{
		{"allowed origin", "http://app.example", true},
		{"no origin (CLI client)", "", true},
		{"other origin", "http://evil.example", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := http.Header{}
			if tt.origin != "" {
				header.Set("Origin", tt.origin)
			}
			conn, resp, err := websocket.DefaultDialer.Dial(wsURL(srv), header)
			if tt.wantOK {
				if err != nil {
					t.Fatalf("dial: %v", err)
				}
				_ = conn.Close()
				return
			}
			if err == nil {
				_ = conn.Close()
				t.Fatal("cross-origin connection accepted")
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != http.StatusForbidden || !strings.Contains(string(body), `"time_ns":"`) {
				t.Errorf("got %d %s, want 403 JSON with time_ns", resp.StatusCode, body)
			}
		})
	}
}

func TestShutdownClosesClients(t *testing.T) {
	hub, srv, cancel := startHub(t)
	c := dial(t, srv, nil)
	waitForClients(t, hub, 1)

	cancel()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := c.ReadMessage()
	var closeErr *websocket.CloseError
	if !errors.As(err, &closeErr) || closeErr.Code != websocket.CloseGoingAway {
		t.Fatalf("err = %v, want close frame 1001", err)
	}
	select {
	case <-hub.Done():
	case <-time.After(time.Second):
		t.Fatal("hub did not stop")
	}
	hub.Broadcast([]byte(`{}`)) // must not block after stop
	if n := hub.ClientCount(); n != 0 {
		t.Errorf("client count after stop = %d", n)
	}
}

func TestSlowClientIsDropped(t *testing.T) {
	hub, _, _ := startHub(t)
	// A client whose writer never drains: buffer of 1.
	slow := &client{id: "slow", hub: hub, send: make(chan []byte, 1)}
	hub.register <- slow

	hub.Broadcast([]byte(`1`)) // fills the buffer
	hub.Broadcast([]byte(`2`)) // overflow → dropped, send closed
	waitForClients(t, hub, 0)

	if msg := <-slow.send; string(msg) != "1" {
		t.Errorf("first message = %q", msg)
	}
	if _, ok := <-slow.send; ok {
		t.Error("send channel not closed after drop")
	}
}
