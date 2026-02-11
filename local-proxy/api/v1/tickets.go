// ./local-proxy/api/v1/tickets.go
package v1

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"local-proxy/internal/config"
	"local-proxy/internal/db"
	"local-proxy/internal/queue"
	"local-proxy/internal/websocket"
)

type TicketHandler struct {
	db        *gorm.DB
	queue     *queue.TicketQueue
	wsManager *websocket.Manager
	config    *config.Config
}

func NewTicketHandler(db *gorm.DB, queue *queue.TicketQueue, wsManager *websocket.Manager, config *config.Config) *TicketHandler {
	return &TicketHandler{
		db:        db,
		queue:     queue,
		wsManager: wsManager,
		config:    config,
	}
}

// CreateTicketRequest - создание тикета
type CreateTicketRequest struct {
	Subject   string                 `json:"subject"`
	Text      string                 `json:"text" binding:"required"`
	ClientID  *uuid.UUID             `json:"client_id"`
	ChannelID *uuid.UUID             `json:"channel_id"`
	Priority  string                 `json:"priority" enums:"low,medium,high,urgent"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// CreateTicket - создание нового тикета
func (h *TicketHandler) CreateTicket(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)

	var req CreateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Получаем диспетчерскую пользователя
	var user db.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Создаем тикет
	ticket := db.Ticket{
		DispatcherID: user.DispatcherID,
		ClientID:     req.ClientID,
		ChannelID:    req.ChannelID,
		Subject:      req.Subject,
		OriginalText: req.Text,
		Priority:     req.Priority,
		Status:       "new",
		CreatedAt:    time.Now(),
	}

	if err := h.db.Create(&ticket).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create ticket"})
		return
	}

	// Создаем первое сообщение от клиента
	message := db.TicketMessage{
		TicketID:    ticket.ID,
		SenderType:  "client",
		SenderID:    req.ClientID,
		MessageText: req.Text,
		Metadata:    req.Metadata,
		CreatedAt:   time.Now(),
	}

	if err := h.db.Create(&message).Error; err != nil {
		// Откатываем тикет если не удалось создать сообщение
		h.db.Delete(&ticket)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create message"})
		return
	}

	// Добавляем в очередь
	if err := h.queue.AddTicket(c.Request.Context(), ticket.ID, user.DispatcherID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add ticket to queue"})
		return
	}

	// Отправляем уведомление через WebSocket
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

// GetTicket - получение тикета по ID
func (h *TicketHandler) GetTicket(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	// Получаем пользователя для проверки доступа
	var user db.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var ticket db.Ticket
	query := h.db.
		Preload("Client").
		Preload("Channel").
		Preload("AssignedUser").
		Where("id = ?", ticketID)

	// Операторы видят только тикеты своей диспетчерской
	if user.Role != "admin" {
		query = query.Where("dispatcher_id = ?", user.DispatcherID)
	}

	if err := query.First(&ticket).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	// Получаем сообщения
	var messages []db.TicketMessage
	if err := h.db.Where("ticket_id = ?", ticketID).
		Order("created_at ASC").
		Find(&messages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load messages"})
		return
	}

	// Получаем AI анализ если есть
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

// ListTickets - список тикетов с фильтрацией
func (h *TicketHandler) ListTickets(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)

	// Получаем пользователя
	var user db.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Параметры запроса
	status := c.Query("status")
	priority := c.Query("priority")
	category := c.Query("category")
	assignedToMe := c.Query("assigned_to_me") == "true"
	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "50")

	// Конструируем запрос
	query := h.db.Model(&db.Ticket{}).
		Preload("Client").
		Preload("Channel").
		Preload("AssignedUser")

	// Фильтр по диспетчерской
	if user.Role != "admin" {
		query = query.Where("dispatcher_id = ?", user.DispatcherID)
	}

	// Фильтры
	if status != "" {
		query = query.Where("status = ?", status)
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

	// Пагинация
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

	// Выполняем запрос
	var tickets []db.Ticket
	if err := query.
		Order("created_at DESC").
		Offset(offset).
		Limit(limitInt).
		Find(&tickets).Error; err != nil {
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

// UpdateTicket - обновление тикета
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

	// Проверяем доступ
	var ticket db.Ticket
	if err := h.db.Where("id = ?", ticketID).First(&ticket).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	var user db.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if user.Role != "admin" && ticket.DispatcherID != user.DispatcherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Разрешенные поля для обновления
	allowedFields := map[string]bool{
		"status":      true,
		"priority":    true,
		"category":    true,
		"assigned_to": true,
	}

	// Фильтруем поля
	filteredUpdate := make(map[string]interface{})
	for key, value := range updateData {
		if allowedFields[key] {
			filteredUpdate[key] = value
		}
	}

	filteredUpdate["updated_at"] = time.Now()

	// Обновляем тикет
	if err := h.db.Model(&ticket).Updates(filteredUpdate).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update ticket"})
		return
	}

	// Если назначили оператора, обновляем очередь
	if assignedTo, ok := updateData["assigned_to"]; ok && assignedTo != nil {
		assignedUserID, err := uuid.Parse(assignedTo.(string))
		if err == nil {
			h.queue.AssignTicket(c.Request.Context(), ticketID, assignedUserID)
		}
	}

	// Отправляем уведомление
	h.wsManager.SendToRole("operator", "ticket_updated", gin.H{
		"ticket_id": ticketID,
		"updates":   filteredUpdate,
	})

	c.JSON(http.StatusOK, gin.H{
		"message": "Ticket updated successfully",
		"ticket":  ticket,
	})
}

// AddMessageRequest - добавление сообщения
type AddMessageRequest struct {
	MessageText string                 `json:"message_text" binding:"required"`
	SenderType  string                 `json:"sender_type" binding:"required,oneof=client operator ai"`
	Attachments map[string]interface{} `json:"attachments"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// AddMessage - добавление сообщения в тикет
func (h *TicketHandler) AddMessage(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	var req AddMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Проверяем доступ к тикету
	var ticket db.Ticket
	if err := h.db.Where("id = ?", ticketID).First(&ticket).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	var user db.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if user.Role != "admin" && ticket.DispatcherID != user.DispatcherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Создаем сообщение
	message := db.TicketMessage{
		TicketID:    ticketID,
		SenderType:  req.SenderType,
		MessageText: req.MessageText,
		Attachments: req.Attachments,
		Metadata:    req.Metadata,
		CreatedAt:   time.Now(),
	}

	// Если отправитель - оператор, сохраняем его ID
	if req.SenderType == "operator" {
		message.SenderID = &userID
	}

	if err := h.db.Create(&message).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create message"})
		return
	}

	// Обновляем время обновления тикета
	h.db.Model(&ticket).Update("updated_at", time.Now())

	// Отправляем уведомление через WebSocket
	h.wsManager.SendToUser(userID, "message_added", gin.H{
		"ticket_id":   ticketID,
		"message_id":  message.ID,
		"sender_type": req.SenderType,
		"text":        req.MessageText,
		"created_at":  message.CreatedAt,
	})

	// Если сообщение от оператора, уведомляем других операторов
	if req.SenderType == "operator" {
		h.wsManager.SendToRole("operator", "ticket_updated", gin.H{
			"ticket_id": ticketID,
			"action":    "message_added",
		})
	}

	c.JSON(http.StatusCreated, gin.H{
		"message_id": message.ID,
		"message":    "Message added successfully",
	})
}

// GetMessages - получение сообщений тикета
func (h *TicketHandler) GetMessages(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	// Проверяем доступ к тикету
	var ticket db.Ticket
	if err := h.db.Where("id = ?", ticketID).First(&ticket).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	var user db.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if user.Role != "admin" && ticket.DispatcherID != user.DispatcherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Параметры пагинации
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

	// Получаем сообщения
	var messages []db.TicketMessage
	var total int64

	h.db.Model(&db.TicketMessage{}).Where("ticket_id = ?", ticketID).Count(&total)

	if err := h.db.Where("ticket_id = ?", ticketID).
		Order("created_at DESC").
		Offset(offset).
		Limit(limitInt).
		Find(&messages).Error; err != nil {
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
