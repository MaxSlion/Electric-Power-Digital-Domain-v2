package ws

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second
	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second
	// Send pings to peer with this period (must be < pongWait)
	pingPeriod = (pongWait * 9) / 10
	// Maximum message size allowed
	maxMessageSize = 1024 * 1024 // 1MB
)

// Client represents a WebSocket connection
type Client struct {
	hub      *Hub
	conn     *websocket.Conn
	send     chan []byte
	jobID    string
	userID   string
	lastPing time.Time
}

// Hub maintains active WebSocket connections and broadcasts messages
type Hub struct {
	mu       sync.RWMutex
	clients  map[string]map[*Client]struct{}
	register chan *Client
	remove   chan *Client
	logger   *zap.Logger
	ctx      context.Context
	cancel   context.CancelFunc
}

// NewHub creates a new WebSocket hub
func NewHub() *Hub {
	ctx, cancel := context.WithCancel(context.Background())
	h := &Hub{
		clients:  make(map[string]map[*Client]struct{}),
		register: make(chan *Client, 100),
		remove:   make(chan *Client, 100),
		ctx:      ctx,
		cancel:   cancel,
	}
	go h.run()
	return h
}

// NewHubWithLogger creates a hub with structured logging
func NewHubWithLogger(logger *zap.Logger) *Hub {
	h := NewHub()
	h.logger = logger
	return h
}

func (h *Hub) run() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			return
		case client := <-h.register:
			h.mu.Lock()
			if h.clients[client.jobID] == nil {
				h.clients[client.jobID] = make(map[*Client]struct{})
			}
			h.clients[client.jobID][client] = struct{}{}
			h.mu.Unlock()

			if h.logger != nil {
				h.logger.Info("WebSocket client connected",
					zap.String("job_id", client.jobID),
					zap.String("user_id", client.userID))
			}

		case client := <-h.remove:
			h.mu.Lock()
			if clients, ok := h.clients[client.jobID]; ok {
				if _, ok := clients[client]; ok {
					delete(clients, client)
					close(client.send)
				}
				if len(clients) == 0 {
					delete(h.clients, client.jobID)
				}
			}
			h.mu.Unlock()

			if h.logger != nil {
				h.logger.Info("WebSocket client disconnected",
					zap.String("job_id", client.jobID))
			}

		case <-ticker.C:
			// Clean up stale connections
			h.cleanupStale()
		}
	}
}

func (h *Hub) cleanupStale() {
	h.mu.Lock()
	defer h.mu.Unlock()

	now := time.Now()
	for jobID, clients := range h.clients {
		for client := range clients {
			if now.Sub(client.lastPing) > pongWait*2 {
				delete(clients, client)
				close(client.send)
			}
		}
		if len(clients) == 0 {
			delete(h.clients, jobID)
		}
	}
}

// Subscribe registers a new client for a job ID (simple interface)
func (h *Hub) Subscribe(jobID string, conn *websocket.Conn) {
	h.SubscribeWithUser(jobID, "", conn)
}

// SubscribeWithUser registers a client with user tracking
func (h *Hub) SubscribeWithUser(jobID, userID string, conn *websocket.Conn) {
	client := &Client{
		hub:      h,
		conn:     conn,
		send:     make(chan []byte, 256),
		jobID:    jobID,
		userID:   userID,
		lastPing: time.Now(),
	}

	conn.SetReadLimit(maxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		client.lastPing = time.Now()
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	h.register <- client

	go h.writePump(client)
	go h.readPump(client)
}

func (h *Hub) readPump(client *Client) {
	defer func() {
		h.remove <- client
		client.conn.Close()
	}()

	for {
		_, _, err := client.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				if h.logger != nil {
					h.logger.Warn("WebSocket read error", zap.Error(err))
				}
			}
			return
		}
	}
}

func (h *Hub) writePump(client *Client) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		client.conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.send:
			client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				client.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := client.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			client.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// Unsubscribe removes a connection (simple interface for compatibility)
func (h *Hub) Unsubscribe(jobID string, conn *websocket.Conn) {
	// The connection will be cleaned up by the readPump when it closes
}

// Broadcast sends a message to all clients subscribed to a job
func (h *Hub) Broadcast(jobID string, payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.clients[jobID]; ok {
		for client := range clients {
			select {
			case client.send <- payload:
			default:
				// Channel full, client too slow
				go func(c *Client) { h.remove <- c }(client)
			}
		}
	}
}

// BroadcastJSON marshals and broadcasts a message
func (h *Hub) BroadcastJSON(jobID string, msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	h.Broadcast(jobID, data)
	return nil
}

// BroadcastAll sends a message to all connected clients
func (h *Hub) BroadcastAll(payload []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, clients := range h.clients {
		for client := range clients {
			select {
			case client.send <- payload:
			default:
			}
		}
	}
}

// GetClientCount returns the number of connected clients for a job
func (h *Hub) GetClientCount(jobID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.clients[jobID]; ok {
		return len(clients)
	}
	return 0
}

// GetTotalClients returns the total number of connected clients
func (h *Hub) GetTotalClients() int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	total := 0
	for _, clients := range h.clients {
		total += len(clients)
	}
	return total
}

// Close shuts down the hub gracefully
func (h *Hub) Close() {
	h.cancel()
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, clients := range h.clients {
		for client := range clients {
			close(client.send)
		}
	}
	h.clients = make(map[string]map[*Client]struct{})
}
