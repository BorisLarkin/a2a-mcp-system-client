// ./local-proxy/api/v1/websocket.go
package v1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"

	"local-proxy/internal/auth"
	"local-proxy/internal/db"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// В продакшене проверять origin
		return true
	},
}

type WebSocketClient struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Username     string
	Role         string
	DispatcherID uuid.UUID
	Conn         *websocket.Conn
	Send         chan []byte
	Hub          *WebSocketHub
}

type WebSocketMessage struct {
	Type      string          `json:"type"`
	Data      json.RawMessage `json:"data,omitempty"`
	TicketID  *uuid.UUID      `json:"ticket_id,omitempty"`
	Channel   string          `json:"channel,omitempty"`
	Timestamp time.Time       `json:"timestamp"`
}

type WebSocketHub struct {
	clients    map[uuid.UUID]*WebSocketClient
	byUser     map[uuid.UUID][]uuid.UUID // user_id -> []client_ids
	byRole     map[string][]uuid.UUID    // role -> []client_ids
	byTicket   map[uuid.UUID][]uuid.UUID // ticket_id -> []client_ids (подписчики)
	mu         sync.RWMutex
	db         *gorm.DB
	auth       *auth.Manager
	register   chan *WebSocketClient
	unregister chan *WebSocketClient
	broadcast  chan *WebSocketMessage
}

func NewWebSocketHub(db *gorm.DB, auth *auth.Manager) *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[uuid.UUID]*WebSocketClient),
		byUser:     make(map[uuid.UUID][]uuid.UUID),
		byRole:     make(map[string][]uuid.UUID),
		byTicket:   make(map[uuid.UUID][]uuid.UUID),
		db:         db,
		auth:       auth,
		register:   make(chan *WebSocketClient),
		unregister: make(chan *WebSocketClient),
		broadcast:  make(chan *WebSocketMessage, 256),
	}
}

func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case message := <-h.broadcast:
			h.handleBroadcast(message)
		}
	}
}

func (h *WebSocketHub) registerClient(client *WebSocketClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client.ID] = client

	// Индексируем по пользователю
	h.byUser[client.UserID] = append(h.byUser[client.UserID], client.ID)

	// Индексируем по роли
	h.byRole[client.Role] = append(h.byRole[client.Role], client.ID)

	// Отправляем приветственное сообщение
	welcomeMsg := WebSocketMessage{
		Type:      "connected",
		Timestamp: time.Now(),
		Data:      json.RawMessage(`{"message": "Connected to WebSocket server"}`),
	}
	client.Send <- h.marshalMessage(welcomeMsg)

	fmt.Printf("WebSocket client registered: %s (user: %s, role: %s)\n",
		client.ID, client.Username, client.Role)
}

func (h *WebSocketHub) unregisterClient(client *WebSocketClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client.ID]; ok {
		delete(h.clients, client.ID)

		// Удаляем из byUser
		if userClients, ok := h.byUser[client.UserID]; ok {
			for i, id := range userClients {
				if id == client.ID {
					h.byUser[client.UserID] = append(userClients[:i], userClients[i+1:]...)
					break
				}
			}
			if len(h.byUser[client.UserID]) == 0 {
				delete(h.byUser, client.UserID)
			}
		}

		// Удаляем из byRole
		if roleClients, ok := h.byRole[client.Role]; ok {
			for i, id := range roleClients {
				if id == client.ID {
					h.byRole[client.Role] = append(roleClients[:i], roleClients[i+1:]...)
					break
				}
			}
			if len(h.byRole[client.Role]) == 0 {
				delete(h.byRole, client.Role)
			}
		}

		// Удаляем из подписок на тикеты
		for ticketID, subscribers := range h.byTicket {
			for i, id := range subscribers {
				if id == client.ID {
					h.byTicket[ticketID] = append(subscribers[:i], subscribers[i+1:]...)
					break
				}
			}
			if len(h.byTicket[ticketID]) == 0 {
				delete(h.byTicket, ticketID)
			}
		}

		close(client.Send)
		fmt.Printf("WebSocket client unregistered: %s\n", client.ID)
	}
}

func (h *WebSocketHub) handleBroadcast(message *WebSocketMessage) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	switch message.Channel {
	case "role":
		if message.Type == "" {
			return
		}
		// Отправляем всем пользователям с определенной ролью
		if clients, ok := h.byRole[message.Channel]; ok {
			for _, clientID := range clients {
				if client, exists := h.clients[clientID]; exists {
					select {
					case client.Send <- h.marshalMessage(*message):
					default:
						// Канал переполнен, закрываем соединение
						go h.unregisterClient(client)
					}
				}
			}
		}

	case "user":
		// Отправляем конкретному пользователю
		if message.Data == nil {
			return
		}
		var data struct {
			UserID uuid.UUID `json:"user_id"`
		}
		if err := json.Unmarshal(message.Data, &data); err != nil {
			return
		}

		if clients, ok := h.byUser[data.UserID]; ok {
			for _, clientID := range clients {
				if client, exists := h.clients[clientID]; exists {
					select {
					case client.Send <- h.marshalMessage(*message):
					default:
						go h.unregisterClient(client)
					}
				}
			}
		}

	case "ticket":
		// Отправляем всем подписчикам тикета
		if message.TicketID == nil {
			return
		}
		if subscribers, ok := h.byTicket[*message.TicketID]; ok {
			for _, clientID := range subscribers {
				if client, exists := h.clients[clientID]; exists {
					select {
					case client.Send <- h.marshalMessage(*message):
					default:
						go h.unregisterClient(client)
					}
				}
			}
		}

	case "broadcast":
		// Отправляем всем
		for _, client := range h.clients {
			select {
			case client.Send <- h.marshalMessage(*message):
			default:
				go h.unregisterClient(client)
			}
		}
	}
}

func (h *WebSocketHub) marshalMessage(msg WebSocketMessage) []byte {
	data, _ := json.Marshal(msg)
	return data
}

// Публичные методы для отправки сообщений

// SendToUser отправляет сообщение конкретному пользователю
func (h *WebSocketHub) SendToUser(userID uuid.UUID, msgType string, data interface{}) {
	jsonData, _ := json.Marshal(data)

	message := WebSocketMessage{
		Type:      msgType,
		Data:      jsonData,
		Channel:   "user",
		Timestamp: time.Now(),
	}

	userMsg := struct {
		UserID uuid.UUID `json:"user_id"`
	}{UserID: userID}

	userMsgData, _ := json.Marshal(userMsg)
	message.Data = userMsgData

	h.broadcast <- &message
}

// SendToRole отправляет сообщение всем пользователям с определенной ролью
func (h *WebSocketHub) SendToRole(role string, msgType string, data interface{}) {
	jsonData, _ := json.Marshal(data)

	message := WebSocketMessage{
		Type:      msgType,
		Data:      jsonData,
		Channel:   role,
		Timestamp: time.Now(),
	}

	h.broadcast <- &message
}

// SendToTicket отправляет сообщение всем подписчикам тикета
func (h *WebSocketHub) SendToTicket(ticketID uuid.UUID, msgType string, data interface{}) {
	jsonData, _ := json.Marshal(data)

	message := WebSocketMessage{
		Type:      msgType,
		Data:      jsonData,
		TicketID:  &ticketID,
		Channel:   "ticket",
		Timestamp: time.Now(),
	}

	h.broadcast <- &message
}

// Broadcast отправляет сообщение всем подключенным клиентам
func (h *WebSocketHub) Broadcast(msgType string, data interface{}) {
	jsonData, _ := json.Marshal(data)

	message := WebSocketMessage{
		Type:      msgType,
		Data:      jsonData,
		Channel:   "broadcast",
		Timestamp: time.Now(),
	}

	h.broadcast <- &message
}

// SubscribeTicket подписывает клиента на обновления тикета
func (h *WebSocketHub) SubscribeTicket(clientID uuid.UUID, ticketID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.byTicket[ticketID] = append(h.byTicket[ticketID], clientID)
}

// UnsubscribeTicket отписывает клиента от обновлений тикета
func (h *WebSocketHub) UnsubscribeTicket(clientID uuid.UUID, ticketID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if subscribers, ok := h.byTicket[ticketID]; ok {
		for i, id := range subscribers {
			if id == clientID {
				h.byTicket[ticketID] = append(subscribers[:i], subscribers[i+1:]...)
				break
			}
		}
		if len(h.byTicket[ticketID]) == 0 {
			delete(h.byTicket, ticketID)
		}
	}
}

// WebSocketHandler - обработчик WebSocket соединений
type WebSocketHandler struct {
	hub  *WebSocketHub
	db   *gorm.DB
	auth *auth.Manager
}

func NewWebSocketHandler(hub *WebSocketHub, db *gorm.DB, auth *auth.Manager) *WebSocketHandler {
	return &WebSocketHandler{
		hub:  hub,
		db:   db,
		auth: auth,
	}
}

// HandleWebSocket - основной обработчик WebSocket
func (h *WebSocketHandler) HandleWebSocket(c *gin.Context) {
	// Получаем токен из query параметра или заголовка
	token := c.Query("token")
	if token == "" {
		token = c.GetHeader("Sec-WebSocket-Protocol")
		// WebSocket протокол не поддерживает заголовки как HTTP
		// Используем подпротокол для передачи токена
	}

	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No token provided"})
		return
	}

	// Валидируем токен
	claims, err := h.auth.ValidateToken(token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	// Получаем пользователя
	var user db.User
	if err := h.db.Where("id = ?", claims.UserID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Апгрейдим соединение до WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		fmt.Printf("Failed to upgrade connection: %v\n", err)
		return
	}

	// Создаем клиента
	client := &WebSocketClient{
		ID:           uuid.New(),
		UserID:       user.ID,
		Username:     user.Username,
		Role:         user.Role,
		DispatcherID: user.DispatcherID,
		Conn:         conn,
		Send:         make(chan []byte, 256),
		Hub:          h.hub,
	}

	// Регистрируем клиента
	h.hub.register <- client

	// Запускаем горутины для чтения и записи
	go client.writePump()
	go client.readPump(h)
}

// readPump - чтение сообщений от клиента
func (c *WebSocketClient) readPump(handler *WebSocketHandler) {
	defer func() {
		c.Hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(512 * 1024) // 512KB
	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg WebSocketMessage
		err := c.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Printf("WebSocket error: %v\n", err)
			}
			break
		}

		// Обрабатываем сообщение
		c.handleMessage(msg, handler)
	}
}

// writePump - запись сообщений клиенту
func (c *WebSocketClient) writePump() {
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

// handleMessage - обработка входящих сообщений
func (c *WebSocketClient) handleMessage(msg WebSocketMessage, handler *WebSocketHandler) {
	switch msg.Type {
	case "ping":
		response := WebSocketMessage{
			Type:      "pong",
			Timestamp: time.Now(),
		}
		c.Send <- handler.hub.marshalMessage(response)

	case "subscribe_ticket":
		if msg.TicketID != nil {
			c.Hub.SubscribeTicket(c.ID, *msg.TicketID)

			// Отправляем подтверждение
			response := WebSocketMessage{
				Type:      "subscribed",
				TicketID:  msg.TicketID,
				Timestamp: time.Now(),
				Data:      json.RawMessage(`{"status": "success"}`),
			}
			c.Send <- handler.hub.marshalMessage(response)
		}

	case "unsubscribe_ticket":
		if msg.TicketID != nil {
			c.Hub.UnsubscribeTicket(c.ID, *msg.TicketID)

			response := WebSocketMessage{
				Type:      "unsubscribed",
				TicketID:  msg.TicketID,
				Timestamp: time.Now(),
				Data:      json.RawMessage(`{"status": "success"}`),
			}
			c.Send <- handler.hub.marshalMessage(response)
		}

	case "typing":
		// Уведомляем других подписчиков о том, что пользователь печатает
		if msg.TicketID != nil {
			data := map[string]interface{}{
				"user_id":   c.UserID,
				"username":  c.Username,
				"is_typing": true,
			}
			jsonData, _ := json.Marshal(data)

			notification := WebSocketMessage{
				Type:      "typing",
				TicketID:  msg.TicketID,
				Data:      jsonData,
				Timestamp: time.Now(),
			}

			c.Hub.SendToTicket(*msg.TicketID, "typing", notification)
		}

	case "mark_read":
		// Отмечаем сообщения как прочитанные
		if msg.TicketID != nil {
			// TODO: Обновить статус прочтения в БД
			c.Hub.SendToTicket(*msg.TicketID, "marked_read", map[string]interface{}{
				"user_id":   c.UserID,
				"ticket_id": msg.TicketID,
			})
		}
	}
}

// GetStats - получение статистики WebSocket соединений
func (h *WebSocketHandler) GetStats(c *gin.Context) {
	h.hub.mu.RLock()
	defer h.hub.mu.RUnlock()

	stats := gin.H{
		"total_clients":              len(h.hub.clients),
		"total_users":                len(h.hub.byUser),
		"total_ticket_subscriptions": len(h.hub.byTicket),
		"clients_by_role":            make(map[string]int),
	}

	for role, clients := range h.hub.byRole {
		stats["clients_by_role"].(map[string]int)[role] = len(clients)
	}

	c.JSON(http.StatusOK, stats)
}
