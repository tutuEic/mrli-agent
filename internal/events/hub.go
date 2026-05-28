// Package events provides WebSocket event broadcasting.
package events

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Event represents a WebSocket event.
type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

// Hub manages WebSocket clients and broadcasts events.
type Hub struct {
	mu         sync.RWMutex
	clients    map[*websocket.Conn]bool
	broadcast  chan Event
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
}

// NewHub creates a new WebSocket Hub.
func NewHub() *Hub {
	h := &Hub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan Event, 256),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
	go h.run()
	return h
}

// run processes register/unregister/broadcast events.
func (h *Hub) run() {
	for {
		select {
		case conn := <-h.register:
			h.mu.Lock()
			h.clients[conn] = true
			h.mu.Unlock()
			log.Printf("[ws] Client connected (%d total)", len(h.clients))

		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[conn]; ok {
				delete(h.clients, conn)
				conn.Close()
			}
			h.mu.Unlock()
			log.Printf("[ws] Client disconnected (%d total)", len(h.clients))

		case event := <-h.broadcast:
			data, err := json.Marshal(event)
			if err != nil {
				log.Printf("[ws] Marshal error: %v", err)
				continue
			}

			h.mu.RLock()
			var dead []*websocket.Conn
			for conn := range h.clients {
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					dead = append(dead, conn)
				}
			}
			h.mu.RUnlock()

			// Cleanup dead connections
			if len(dead) > 0 {
				h.mu.Lock()
				for _, conn := range dead {
					if _, ok := h.clients[conn]; ok {
						delete(h.clients, conn)
						conn.Close()
					}
				}
				h.mu.Unlock()
			}
		}
	}
}

// Register adds a new WebSocket client.
func (h *Hub) Register(conn *websocket.Conn) {
	h.register <- conn
}

// Unregister removes a WebSocket client.
func (h *Hub) Unregister(conn *websocket.Conn) {
	h.unregister <- conn
}

// Broadcast sends an event to all connected clients.
func (h *Hub) Broadcast(eventType string, payload any) {
	h.broadcast <- Event{
		Type:    eventType,
		Payload: payload,
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Event type constants.
const (
	EventAgentStatus     = "agent.status"
	EventAgentWakeup     = "agent.wakeup"
	EventAgentRestart    = "agent.restart"
	EventAgentHealth     = "agent.health"
	EventAgentRecovered  = "agent.recovered"
	EventTaskCreated     = "task.created"
	EventTaskUpdated     = "task.updated"
	EventTaskLog         = "task.log"
	EventDashboardUpdate = "dashboard.updated"
	EventChatMessage     = "chat.message"
)

// Timestamp returns the current time formatted for events.
func Timestamp() string {
	return time.Now().Format(time.RFC3339)
}
