// ./local-proxy/internal/websocket/manager.go
package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// В продакшене проверять origin
		return true
	},
}

type Client struct {
	ID      uuid.UUID
	UserID  uuid.UUID
	Role    string
	Conn    *websocket.Conn
	Send    chan []byte
	Manager *Manager
}

type Message struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

type Manager struct {
	clients map[uuid.UUID]*Client
	users   map[uuid.UUID][]uuid.UUID // user_id -> []client_ids
	mu      sync.RWMutex
	redis   *redis.Client
	ctx     context.Context
}

func NewManager() *Manager {
	m := &Manager{
		clients: make(map[uuid.UUID]*Client),
		users:   make(map[uuid.UUID][]uuid.UUID),
		mu:      sync.RWMutex{},
		redis:   nil, // будет установлен позже
		ctx:     context.Background(),
	}

	go m.handleRedisMessages()

	return m
}

func (m *Manager) SetRedisClient(redis *redis.Client) {
	m.redis = redis
}

// ServeWS обработка WebSocket подключения
func (m *Manager) ServeWS(w http.ResponseWriter, r *http.Request, userID uuid.UUID, role string) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}

	client := &Client{
		ID:      uuid.New(),
		UserID:  userID,
		Role:    role,
		Conn:    conn,
		Send:    make(chan []byte, 256),
		Manager: m,
	}

	m.registerClient(client)

	go client.writePump()
	go client.readPump()
}

func (m *Manager) registerClient(client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.clients[client.ID] = client
	m.users[client.UserID] = append(m.users[client.UserID], client.ID)

	log.Printf("Client connected: %s (user: %s)", client.ID, client.UserID)
}

func (m *Manager) unregisterClient(client *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.clients, client.ID)

	// Удаляем из списка пользователя
	if userClients, ok := m.users[client.UserID]; ok {
		for i, id := range userClients {
			if id == client.ID {
				m.users[client.UserID] = append(userClients[:i], userClients[i+1:]...)
				break
			}
		}

		if len(m.users[client.UserID]) == 0 {
			delete(m.users, client.UserID)
		}
	}

	close(client.Send)
	log.Printf("Client disconnected: %s", client.ID)
}

// SendToUser отправляет сообщение всем клиентам пользователя
func (m *Manager) SendToUser(userID uuid.UUID, msgType string, data interface{}) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if clientIDs, ok := m.users[userID]; ok {
		message := Message{
			Type:      msgType,
			Data:      data,
			Timestamp: time.Now(),
		}

		jsonMsg, err := json.Marshal(message)
		if err != nil {
			log.Printf("Failed to marshal message: %v", err)
			return
		}

		for _, clientID := range clientIDs {
			if client, ok := m.clients[clientID]; ok {
				select {
				case client.Send <- jsonMsg:
				default:
					// Канал полон, пропускаем
					go m.unregisterClient(client)
				}
			}
		}
	}
}

// SendToRole отправляет сообщение всем пользователям с определенной ролью
func (m *Manager) SendToRole(role string, msgType string, data interface{}) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	message := Message{
		Type:      msgType,
		Data:      data,
		Timestamp: time.Now(),
	}

	jsonMsg, err := json.Marshal(message)
	if err != nil {
		log.Printf("Failed to marshal message: %v", err)
		return
	}

	for _, client := range m.clients {
		if client.Role == role {
			select {
			case client.Send <- jsonMsg:
			default:
				go m.unregisterClient(client)
			}
		}
	}
}

// Broadcast отправляет сообщение всем подключенным клиентам
func (m *Manager) Broadcast(msgType string, data interface{}) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	message := Message{
		Type:      msgType,
		Data:      data,
		Timestamp: time.Now(),
	}

	jsonMsg, err := json.Marshal(message)
	if err != nil {
		log.Printf("Failed to marshal message: %v", err)
		return
	}

	for _, client := range m.clients {
		select {
		case client.Send <- jsonMsg:
		default:
			go m.unregisterClient(client)
		}
	}
}

func (m *Manager) handleRedisMessages() {
	if m.redis == nil {
		return
	}

	pubsub := m.redis.Subscribe(m.ctx,
		"ticket:created",
		"ticket:updated",
		"ticket:assigned",
		"ticket:resolved",
		"queue:updated",
		"message:new",
	)

	defer pubsub.Close()

	ch := pubsub.Channel()

	for msg := range ch {
		switch msg.Channel {
		case "ticket:created":
			m.Broadcast("ticket_created", json.RawMessage(msg.Payload))
		case "ticket:updated":
			m.Broadcast("ticket_updated", json.RawMessage(msg.Payload))
		case "ticket:assigned":
			m.Broadcast("ticket_assigned", json.RawMessage(msg.Payload))
		case "ticket:resolved":
			m.Broadcast("ticket_resolved", json.RawMessage(msg.Payload))
		case "queue:updated":
			m.SendToRole("operator", "queue_updated", json.RawMessage(msg.Payload))
		case "message:new":
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(msg.Payload), &data); err == nil {
				if ticketID, ok := data["ticket_id"].(string); ok {
					if assignedTo, ok := data["assigned_to"].(string); ok {
						userID, err := uuid.Parse(assignedTo)
						if err == nil {
							m.SendToUser(userID, "new_message", data)
						}
					}
				}
			}
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
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

func (c *Client) readPump() {
	defer func() {
		c.Manager.unregisterClient(c)
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(512 * 1024) // 512KB
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Обработка входящих сообщений
		var msg Message
		if err := json.Unmarshal(message, &msg); err == nil {
			c.handleMessage(msg)
		}
	}
}

func (c *Client) handleMessage(msg Message) {
	switch msg.Type {
	case "ping":
		response := Message{
			Type:      "pong",
			Timestamp: time.Now(),
		}
		jsonMsg, _ := json.Marshal(response)
		c.Send <- jsonMsg

	case "subscribe_ticket":
		if data, ok := msg.Data.(map[string]interface{}); ok {
			if ticketID, ok := data["ticket_id"].(string); ok {
				// Подписываемся на обновления тикета
				if c.Manager.redis != nil {
					channel := fmt.Sprintf("ticket:%s", ticketID)
					// Здесь можно добавить логику подписки
				}
			}
		}
	}
}

func (m *Manager) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, client := range m.clients {
		client.Conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, "server shutdown"))
		client.Conn.Close()
		close(client.Send)
	}

	m.clients = make(map[uuid.UUID]*Client)
	m.users = make(map[uuid.UUID][]uuid.UUID)
}
