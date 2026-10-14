package realtime

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/gorilla/websocket"
)

// client is one WebSocket connection. send is written and closed only by
// the hub; control is written only by readPump (never closed), so the two
// can't race. writePump is the sole writer to conn.
type client struct {
	id      string
	hub     *Hub
	conn    *websocket.Conn
	send    chan []byte
	control chan []byte
}

var pongMessage = []byte(`{"type":"pong"}`)

// readPump keeps the read deadline alive via pongs, answers {"type":"ping"},
// ignores any other inbound message, and unregisters on disconnect.
func (c *client) readPump() {
	defer func() {
		select {
		case c.hub.unregister <- c:
		case <-c.hub.done:
		}
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) &&
				!errors.Is(err, websocket.ErrReadLimit) {
				c.hub.logger.Debug("websocket read error", "client_id", c.id, "error", err)
			}
			return
		}
		var msg struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(data, &msg) == nil && msg.Type == "ping" {
			select {
			case c.control <- pongMessage:
			default: // client is spamming pings; skip
			}
		}
	}
}

// writePump is the only goroutine writing to conn: queued messages, pong
// replies, and periodic pings. When the hub closes send it writes a close
// frame and exits.
func (c *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	write := func(messageType int, data []byte) error {
		_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
		return c.conn.WriteMessage(messageType, data)
	}

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				_ = write(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseGoingAway, "server closing connection"))
				return
			}
			if err := write(websocket.TextMessage, msg); err != nil {
				return
			}
		case msg := <-c.control:
			if err := write(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			if err := write(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
