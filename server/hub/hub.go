package hub

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Message struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type Client struct {
	UserID     int
	DeviceType string
	DeviceName string
	Conn       *websocket.Conn
	Send       chan []byte
}

type Hub struct {
	mu         sync.RWMutex
	clients    map[int]map[*Client]bool
	Register   chan *Client
	Unregister chan *Client
	Broadcast  chan *userMessage
}

type userMessage struct {
	userID  int
	message []byte
}

func New() *Hub {
	return &Hub{
		clients:    make(map[int]map[*Client]bool),
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Broadcast:  make(chan *userMessage),
	}
}

func (h *Hub) Run() {
	for {
		select {
		case c := <-h.Register:
			h.mu.Lock()
			if h.clients[c.UserID] == nil {
				h.clients[c.UserID] = make(map[*Client]bool)
			}
			h.clients[c.UserID][c] = true
			h.mu.Unlock()

		case c := <-h.Unregister:
			h.mu.Lock()
			if clients, ok := h.clients[c.UserID]; ok {
				if _, exists := clients[c]; exists {
					close(c.Send)
					delete(clients, c)
				}
				if len(clients) == 0 {
					delete(h.clients, c.UserID)
				}
			}
			h.mu.Unlock()

		case um := <-h.Broadcast:
			h.mu.RLock()
			if clients, ok := h.clients[um.userID]; ok {
				for c := range clients {
					select {
					case c.Send <- um.message:
					default:
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) SendToUser(userID int, msg Message) {
	data, _ := json.Marshal(msg)
	h.Broadcast <- &userMessage{userID: userID, message: data}
}

func (h *Hub) WritePump(c *Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func WebsocketUpgrader() websocket.Upgrader {
	return websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
}
