package websocket

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/rs/zerolog/log"
)

// getAllowedOrigins returns the list of allowed WebSocket origins from environment.
func getAllowedOrigins() []string {
	origins := os.Getenv("ALLOWED_ORIGINS")
	if origins == "" {
		return []string{
			"http://localhost:3000",
			"http://localhost:8080",
			"http://127.0.0.1:3000",
			"http://127.0.0.1:8080",
		}
	}
	return strings.Split(origins, ",")
}

// isOriginAllowed checks if the origin is in the allowed list.
func isOriginAllowed(origin string) bool {
	allowedOrigins := getAllowedOrigins()
	for _, allowed := range allowedOrigins {
		if strings.TrimSpace(allowed) == origin {
			return true
		}
	}
	return false
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			// Allow requests without Origin header (e.g., same-origin or non-browser clients).
			return true
		}
		allowed := isOriginAllowed(origin)
		if !allowed {
			log.Warn().Str("origin", origin).Msg("websocket connection rejected: origin not allowed")
		}
		return allowed
	},
}

// Hub manages WebSocket connections for real-time updates.
type Hub struct {
	mu      sync.RWMutex
	clients map[*websocket.Conn]bool
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[*websocket.Conn]bool),
	}
}

// HandleConnect upgrades HTTP to WebSocket and registers the client.
func (h *Hub) HandleConnect(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Error().Err(err).Msg("websocket upgrade failed")
		return
	}

	h.mu.Lock()
	h.clients[conn] = true
	h.mu.Unlock()

	log.Info().Str("remote", conn.RemoteAddr().String()).Msg("websocket client connected")

	// Read loop — keep connection alive and handle disconnects.
	go func() {
		defer func() {
			h.mu.Lock()
			delete(h.clients, conn)
			h.mu.Unlock()
			conn.Close()
			log.Info().Str("remote", conn.RemoteAddr().String()).Msg("websocket client disconnected")
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				break
			}
		}
	}()
}

// Broadcast sends a message to all connected WebSocket clients.
func (h *Hub) Broadcast(event string, data interface{}) {
	msg, err := json.Marshal(map[string]interface{}{
		"event": event,
		"data":  data,
	})
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal broadcast message")
		return
	}

	// Collect failed connections under read lock.
	h.mu.RLock()
	var failedConns []*websocket.Conn
	for conn := range h.clients {
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			log.Warn().Err(err).Msg("failed to write to websocket client")
			failedConns = append(failedConns, conn)
		}
	}
	h.mu.RUnlock()

	// Remove failed connections under write lock.
	if len(failedConns) > 0 {
		h.mu.Lock()
		for _, conn := range failedConns {
			conn.Close()
			delete(h.clients, conn)
		}
		h.mu.Unlock()
	}
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
