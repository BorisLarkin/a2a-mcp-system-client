// ./local-proxy/api/v1/tickets.go
package v1

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"local-proxy/internal/config"
	"local-proxy/internal/db"
	"local-proxy/internal/messaging"
	"local-proxy/internal/queue"
	"local-proxy/internal/services"
	"local-proxy/internal/websocket"
)

type TicketHandler struct {
	db        *gorm.DB
	queue     *queue.TicketQueue
	wsManager *websocket.Manager
	config    *config.Config
	processor *services.TicketProcessor
	botClient *messaging.BotClient
}

func NewTicketHandler(db *gorm.DB, queue *queue.TicketQueue, wsManager *websocket.Manager, config *config.Config, processor *services.TicketProcessor, botClient *messaging.BotClient) *TicketHandler {
	return &TicketHandler{
		db:        db,
		queue:     queue,
		wsManager: wsManager,
		config:    config,
		processor: processor,
		botClient: botClient,
	}
}

type CreateTicketRequest struct {
	Text             string                 `json:"text" binding:"required"`
	Subject          string                 `json:"subject"`
	ClientID         *uuid.UUID             `json:"client_id"`
	ChannelID        *uuid.UUID             `json:"channel_id"`
	ClientExternalID string                 `json:"client_external_id"`
	Priority         string                 `json:"priority"`
	Metadata         map[string]interface{} `json:"metadata"`
}

func (h *TicketHandler) CreateTicket(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c) // nil при публичном доступе

	var req CreateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	if req.Priority == "" {
		req.Priority = "medium"
	}

	var dispatcherID uuid.UUID
	var clientID *uuid.UUID
	var channelID *uuid.UUID

	// Для публичного доступа — создаём клиента по client_external_id
	if req.ClientExternalID != "" {
		var client db.Client
		if err := h.db.Where("external_id = ?", req.ClientExternalID).First(&client).Error; err != nil {
			client = db.Client{
				ExternalID:  req.ClientExternalID,
				Name:        "Web Widget User",
				ContactInfo: req.ClientExternalID,
			}
			h.db.Create(&client)
		}
		clientID = &client.ID
	}

	// --- Определяем диспетчерскую ---
	if userID != uuid.Nil {
		var user db.User
		if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		dispatcherID = user.DispatcherID
	} else {
		var dispatcher db.Dispatcher
		if err := h.db.Where("is_active = ?", true).First(&dispatcher).Error; err != nil {
			dispID, parseErr := uuid.Parse(h.config.Dispatcher.DispatcherID)
			if parseErr != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "No active dispatcher found"})
				return
			}
			dispatcherID = dispID
		} else {
			dispatcherID = dispatcher.ID
		}
	}

	// --- Создаём/находим клиента (для публичного доступа) ---
	if req.Metadata != nil {
		if chatID, ok := req.Metadata["chat_id"]; ok {
			var externalID string
			switch v := chatID.(type) {
			case float64:
				externalID = fmt.Sprintf("%.0f", v)
			case string:
				externalID = v
			default:
				externalID = fmt.Sprintf("%v", v)
			}

			var client db.Client
			if err := h.db.Where("external_id = ?", externalID).First(&client).Error; err != nil {
				client = db.Client{
					ExternalID:  externalID,
					Name:        fmt.Sprintf("%v %v", req.Metadata["first_name"], req.Metadata["last_name"]),
					ContactInfo: fmt.Sprintf("%v", req.Metadata["username"]),
					Metadata:    db.JSONB(req.Metadata),
				}
				h.db.Create(&client)
			}
			clientID = &client.ID
			req.ClientID = clientID
		}
	}

	// Если клиент передан явно
	if req.ClientID != nil && clientID == nil {
		clientID = req.ClientID
	}

	// --- Создаём/находим канал ---
	if req.Metadata != nil {
		if chatType, ok := req.Metadata["channel_type"]; ok || true {
			_ = chatType
			var channel db.Channel
			if err := h.db.Where("dispatcher_id = ? AND type = ?", dispatcherID, "telegram").First(&channel).Error; err != nil {
				channel = db.Channel{
					DispatcherID: dispatcherID,
					Type:         "telegram",
					Name:         "Telegram Bot",
					Config: db.JSONB{
						"chat_id": fmt.Sprintf("%v", req.Metadata["chat_id"]),
					},
				}
				h.db.Create(&channel)
			}
			channelID = &channel.ID
			req.ChannelID = channelID
		}
	}

	// --- Создаём тикет ---
	ticket := db.Ticket{
		DispatcherID: dispatcherID,
		ClientID:     clientID,
		ChannelID:    channelID,
		Subject:      req.Subject,
		OriginalText: req.Text,
		Priority:     req.Priority,
		Status:       "new",
	}

	if err := h.db.Create(&ticket).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create ticket"})
		return
	}

	// --- Первое сообщение ---
	msg := db.TicketMessage{
		TicketID:    ticket.ID,
		SenderType:  "client",
		SenderID:    clientID,
		MessageText: req.Text,
		Metadata:    req.Metadata,
	}
	if err := h.db.Create(&msg).Error; err != nil {
		h.db.Delete(&ticket)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create message"})
		return
	}

	// --- В очередь ---
	if err := h.queue.AddTicket(c.Request.Context(), ticket.ID, dispatcherID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add ticket to queue"})
		return
	}

	// --- Запуск AI-обработки ---
	go h.processor.ProcessTicket(ticket.ID)

	// --- Уведомление операторам ---
	h.wsManager.SendToRole("operator", "ticket_created", gin.H{
		"ticket_id":  ticket.ID,
		"subject":    ticket.Subject,
		"priority":   ticket.Priority,
		"created_at": ticket.CreatedAt,
	})

	c.JSON(http.StatusCreated, gin.H{
		"ticket_id": ticket.ID,
		"message":   "Ticket created successfully",
	})
}

// GetTicket – получение тикета по ID
func (h *TicketHandler) GetTicket(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	var user db.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var ticket db.Ticket
	query := h.db.Preload("Client").Preload("Channel").Preload("AssignedUser").Where("id = ?", ticketID)
	if user.Role != "admin" {
		query = query.Where("dispatcher_id = ?", user.DispatcherID)
	}
	if err := query.First(&ticket).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	var messages []db.TicketMessage
	h.db.Where("ticket_id = ?", ticketID).Order("created_at ASC").Find(&messages)

	var aiAnalysis map[string]interface{}
	if ticket.AIAnalysis != nil {
		aiAnalysis = map[string]interface{}(ticket.AIAnalysis)
	}

	c.JSON(http.StatusOK, gin.H{
		"ticket":      ticket,
		"messages":    messages,
		"ai_analysis": aiAnalysis,
	})
}

// ListTickets – список тикетов с фильтрацией
func (h *TicketHandler) ListTickets(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)

	var user db.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	status := c.Query("status")
	priority := c.Query("priority")
	category := c.Query("category")
	assignedToMe := c.Query("assigned_to_me") == "true"
	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "50")

	query := h.db.Model(&db.Ticket{}).Preload("Client").Preload("Channel").Preload("AssignedUser")
	if user.Role != "admin" {
		query = query.Where("dispatcher_id = ?", user.DispatcherID)
	}
	if status != "" {
		statuses := strings.Split(status, ",")
		query = query.Where("status IN ?", statuses)
	}
	if priority != "" {
		query = query.Where("priority = ?", priority)
	}
	if category != "" {
		query = query.Where("category = ?", category)
	}
	if assignedToMe {
		query = query.Where("assigned_to = ?", userID)
	}

	var total int64
	query.Count(&total)

	pageInt := 1
	limitInt := 50
	fmt.Sscanf(page, "%d", &pageInt)
	fmt.Sscanf(limit, "%d", &limitInt)
	offset := (pageInt - 1) * limitInt
	if offset < 0 {
		offset = 0
	}

	var tickets []db.Ticket
	if err := query.Order("created_at DESC").Offset(offset).Limit(limitInt).Find(&tickets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load tickets"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tickets": tickets,
		"total":   total,
		"page":    pageInt,
		"limit":   limitInt,
	})
}

// UpdateTicket – обновление тикета
func (h *TicketHandler) UpdateTicket(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	var updateData map[string]interface{}
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var ticket db.Ticket
	if err := h.db.First(&ticket, "id = ?", ticketID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	var user db.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if user.Role != "admin" && ticket.DispatcherID != user.DispatcherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	allowedFields := map[string]bool{
		"status": true, "priority": true, "category": true, "assigned_to": true,
		"feedback_status": true,
	}
	filteredUpdate := make(map[string]interface{})
	for key, value := range updateData {
		if allowedFields[key] {
			filteredUpdate[key] = value
		}
	}
	filteredUpdate["updated_at"] = time.Now()

	if err := h.db.Model(&ticket).Updates(filteredUpdate).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update ticket"})
		return
	}

	if assignedTo, ok := updateData["assigned_to"]; ok && assignedTo != nil {
		if assignedUserID, err := uuid.Parse(assignedTo.(string)); err == nil {
			h.queue.AssignTicket(c.Request.Context(), ticketID, assignedUserID)
		}
	}

	h.wsManager.SendToRole("operator", "ticket_updated", gin.H{
		"ticket_id": ticketID,
		"updates":   filteredUpdate,
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "Ticket updated successfully",
		"ticket":  ticket,
	})
}

// UpdateFeedback — обновление статуса тикета по callback от кнопок (публичный доступ)
func (h *TicketHandler) UpdateFeedback(c *gin.Context) {
	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	var req struct {
		Status         string `json:"status"`
		FeedbackStatus string `json:"feedback_status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	updates := map[string]interface{}{
		"updated_at": time.Now(),
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.FeedbackStatus != "" {
		updates["feedback_status"] = req.FeedbackStatus
	}

	if err := h.db.Model(&db.Ticket{}).Where("id = ?", ticketID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update ticket"})
		return
	}

	// Если эскалация — добавляем в очередь
	if req.Status == "waiting" {
		var ticket db.Ticket
		if err := h.db.First(&ticket, "id = ?", ticketID).Error; err == nil {
			h.queue.AddTicket(c.Request.Context(), ticketID, ticket.DispatcherID)
		}
	}

	// Отправляем WebSocket-уведомление
	h.wsManager.Broadcast("ticket_updated", gin.H{
		"ticket_id": ticketID.String(),
		"status":    req.Status,
	})

	c.JSON(http.StatusOK, gin.H{"message": "Feedback updated"})
}

// AddMessageRequest – добавление сообщения
type AddMessageRequest struct {
	MessageText string                 `json:"message_text" binding:"required"`
	SenderType  string                 `json:"sender_type" binding:"required,oneof=client operator ai"`
	Attachments map[string]interface{} `json:"attachments"`
	Metadata    map[string]interface{} `json:"metadata"`
}

func parseChatID(externalID string) int64 {
	var id int64
	fmt.Sscanf(externalID, "%d", &id)
	return id
}

func (h *TicketHandler) AddMessage(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)
	ticketID, _ := uuid.Parse(c.Param("id"))

	var req AddMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var ticket db.Ticket
	if err := h.db.First(&ticket, "id = ?", ticketID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	var user db.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if user.Role != "admin" && ticket.DispatcherID != user.DispatcherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	msg := db.TicketMessage{
		TicketID:    ticketID,
		SenderType:  req.SenderType,
		MessageText: req.MessageText,
		Attachments: req.Attachments,
		Metadata:    req.Metadata,
	}
	if req.SenderType == "operator" {
		msg.SenderID = &userID
	}
	if err := h.db.Create(&msg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create message"})
		return
	}

	h.db.Model(&ticket).Update("updated_at", time.Now())

	h.wsManager.SendToUser(userID, "message_added", gin.H{
		"ticket_id":   ticketID,
		"message_id":  msg.ID,
		"sender_type": req.SenderType,
		"text":        req.MessageText,
		"created_at":  msg.CreatedAt,
	})
	if req.SenderType == "operator" {
		h.wsManager.SendToRole("operator", "ticket_updated", gin.H{
			"ticket_id": ticketID,
			"action":    "message_added",
		})
	}

	// Если сообщение от оператора — отправляем клиенту через бота
	if req.SenderType == "operator" {
		// Загружаем тикет с клиентом
		var ticket db.Ticket
		if err := h.db.Preload("Client").First(&ticket, "id = ?", ticketID).Error; err == nil {
			if ticket.Client.ID != uuid.Nil {
				chatID := parseChatID(ticket.Client.ExternalID)
				if chatID != 0 {
					// Вызываем бота
					botClient := messaging.NewBotClient("http://telegram-bot:8080")
					if err := botClient.SendMessage(chatID, req.MessageText, ticketID.String()); err != nil {
						// Логируем, но не фейлим запрос
						fmt.Printf("Failed to send operator message to client: %v\n", err)
					}
				}
			}
		}
	}

	c.JSON(http.StatusCreated, gin.H{
		"message_id": msg.ID,
		"message":    "Message added successfully",
	})
}

// GetMessages – сообщения тикета
func (h *TicketHandler) GetMessages(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)
	ticketID, _ := uuid.Parse(c.Param("id"))

	var ticket db.Ticket
	if err := h.db.First(&ticket, "id = ?", ticketID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	var user db.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if user.Role != "admin" && ticket.DispatcherID != user.DispatcherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "100")
	pageInt := 1
	limitInt := 100
	fmt.Sscanf(page, "%d", &pageInt)
	fmt.Sscanf(limit, "%d", &limitInt)
	offset := (pageInt - 1) * limitInt
	if offset < 0 {
		offset = 0
	}

	var messages []db.TicketMessage
	var total int64
	h.db.Model(&db.TicketMessage{}).Where("ticket_id = ?", ticketID).Count(&total)
	if err := h.db.Where("ticket_id = ?", ticketID).Order("created_at DESC").Offset(offset).Limit(limitInt).Find(&messages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load messages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"messages": messages,
		"total":    total,
		"page":     pageInt,
		"limit":    limitInt,
	})
}

func (h *TicketHandler) MyTickets(c *gin.Context) {
	externalID := c.Query("client_external_id")
	if externalID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "client_external_id required"})
		return
	}

	var client db.Client
	if err := h.db.Where("external_id = ?", externalID).First(&client).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"tickets": []interface{}{}})
		return
	}

	var tickets []db.Ticket
	h.db.Where("client_id = ?", client.ID).
		Order("created_at DESC").
		Limit(20).
		Find(&tickets)

	// Возвращаем только базовую информацию
	type TicketInfo struct {
		ID           string    `json:"id"`
		OriginalText string    `json:"original_text"`
		Status       string    `json:"status"`
		CreatedAt    time.Time `json:"created_at"`
	}
	result := make([]TicketInfo, len(tickets))
	for i, t := range tickets {
		result[i] = TicketInfo{
			ID:           t.ID.String(),
			OriginalText: t.OriginalText,
			Status:       t.Status,
			CreatedAt:    t.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, gin.H{"tickets": result})
}

// GetPublicTicket — получение тикета по API-ключу (для виджета)
func (h *TicketHandler) GetPublicTicket(c *gin.Context) {
	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	var ticket db.Ticket
	if err := h.db.Preload("Client").First(&ticket, "id = ?", ticketID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	var messages []db.TicketMessage
	h.db.Where("ticket_id = ?", ticketID).Order("created_at ASC").Find(&messages)

	var aiAnalysis map[string]interface{}
	if ticket.AIAnalysis != nil {
		aiAnalysis = map[string]interface{}(ticket.AIAnalysis)
	}

	c.JSON(http.StatusOK, gin.H{
		"ticket":      ticket,
		"messages":    messages,
		"ai_analysis": aiAnalysis,
	})
}
