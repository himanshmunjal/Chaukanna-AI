// Package hub pushes live events to every open dashboard over WebSocket.
//
// Event types sent to the browser:
//
//	call_started        {call}
//	transcript_partial  {t, text}                 (live captions, not saved)
//	transcript          {t, speaker, text}
//	flags               {flags: [...]}            (new red flags)
//	analysis            {state, risk, reason}
//	warning             {text, language, audio}   (audio = base64 WAV, may be empty)
//	call_updated        {caller_name, purpose, summary}
//	call_ended          {state, risk}
package hub

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// Event is one message to the dashboard.
type Event struct {
	Type   string `json:"type"`
	CallID uint   `json:"call_id"`
	Data   any    `json:"data"`
}

// OwnerKey is where the request context holds a demo visitor's id. A connection only
// receives events for calls with the same owner. Real calls and the family have none,
// so a visitor can never receive a real call's events, and the other way round.
const OwnerKey = "demo_visitor"

type client struct {
	conn  *websocket.Conn
	send  chan []byte
	owner string
}

// Hub keeps the list of connected dashboards.
type Hub struct {
	mu       sync.RWMutex
	clients  map[*client]bool
	owners   map[uint]string // demo calls in progress -> their visitor
	upgrader websocket.Upgrader
}

// New creates a Hub.
func New() *Hub {
	return &Hub{
		clients: map[*client]bool{},
		owners:  map[uint]string{},
		upgrader: websocket.Upgrader{
			// The access middleware has already checked the origin of this request.
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

// SetOwner marks a call as a demo visitor's. Set it before the call's first event.
func (h *Hub) SetOwner(callID uint, owner string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.owners[callID] = owner
}

// ClearOwner forgets a finished demo call.
func (h *Hub) ClearOwner(callID uint) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.owners, callID)
}

// Broadcast sends an event to the dashboards allowed to see it. Slow clients are
// skipped, never waited for.
func (h *Hub) Broadcast(ev Event) {
	data, err := json.Marshal(ev)
	if err != nil {
		log.Printf("hub: marshal %s: %v", ev.Type, err)
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	owner := h.owners[ev.CallID]
	for c := range h.clients {
		if c.owner != owner {
			continue
		}
		select {
		case c.send <- data:
		default: // client's buffer is full; drop this event for it
		}
	}
}

// ServeWS upgrades an HTTP request to a dashboard WebSocket (GET /ws/dashboard).
func (h *Hub) ServeWS(ctx *gin.Context) {
	conn, err := h.upgrader.Upgrade(ctx.Writer, ctx.Request, nil)
	if err != nil {
		return
	}
	c := &client{conn: conn, send: make(chan []byte, 128), owner: ctx.GetString(OwnerKey)}

	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()

	go c.writeLoop()

	// We don't expect messages from the browser; reading just detects disconnects.
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}

	h.mu.Lock()
	delete(h.clients, c)
	close(c.send)
	h.mu.Unlock()
}

// writeLoop sends queued events and a ping every 30 s to keep proxies from closing the socket.
func (c *client) writeLoop() {
	ping := time.NewTicker(30 * time.Second)
	defer func() {
		ping.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ping.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
