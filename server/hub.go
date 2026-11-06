package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Hub WebSocket 广播中心：控制台与 overlay 都是客户端
type Hub struct {
	mu      sync.RWMutex
	clients map[*wsClient]struct{}
	upgr    websocket.Upgrader
}

type wsClient struct {
	conn *websocket.Conn
	send chan []byte
	role string
}

func newHub() *Hub {
	return &Hub{
		clients: map[*wsClient]struct{}{},
		upgr: websocket.Upgrader{
			CheckOrigin:     func(r *http.Request) bool { return true },
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
	}
}

func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *Hub) HandleWS(w http.ResponseWriter, r *http.Request) {
	role := r.URL.Query().Get("role")
	if role == "" {
		role = "console"
	}
	conn, err := h.upgr.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &wsClient{conn: conn, send: make(chan []byte, 256), role: role}

	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	go c.writePump()
	c.readPump(h)
}

func (h *Hub) remove(c *wsClient) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
}

// Broadcast 广播一条消息，慢客户端直接丢帧，绝不阻塞引擎
func (h *Hub) Broadcast(kind string, data any) {
	payload, err := json.Marshal(map[string]any{"kind": kind, "data": data, "ts": time.Now().UnixMilli()})
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- payload:
		default:
			// 客户端积压，丢弃本帧；不阻塞
		}
	}
}

func (c *wsClient) readPump(h *Hub) {
	defer func() {
		h.remove(c)
		c.conn.Close()
	}()
	c.conn.SetReadLimit(4096)
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *wsClient) writePump() {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			// 心跳也是 overlay 看门狗的活信号
			if err := c.conn.WriteMessage(websocket.TextMessage, []byte(`{"kind":"ping"}`)); err != nil {
				return
			}
		}
	}
}

func logErr(scope string, err error) {
	if err != nil {
		log.Printf("[%s] %v", scope, err)
	}
}
