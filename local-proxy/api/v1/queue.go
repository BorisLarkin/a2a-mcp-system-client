// ./local-proxy/api/v1/queue.go
package v1

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"local-proxy/internal/db"
	"local-proxy/internal/queue"
)

type QueueHandler struct {
	queue *queue.TicketQueue
	db    *gorm.DB
}

func NewQueueHandler(queue *queue.TicketQueue, db *gorm.DB) *QueueHandler {
	return &QueueHandler{
		queue: queue,
		db:    db,
	}
}

// GetQueue - получение текущей очереди
func (h *QueueHandler) GetQueue(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)

	// Получаем пользователя и его диспетчерскую
	var user db.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	limit := c.DefaultQuery("limit", "50")
	limitInt := 50
	fmt.Sscanf(limit, "%d", &limitInt)

	// Получаем очередь
	queueEntries, err := h.queue.GetQueue(c.Request.Context(), user.DispatcherID, limitInt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get queue"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"queue": queueEntries,
		"count": len(queueEntries),
	})
}

// AssignTicketRequest - запрос на назначение тикета
type AssignTicketRequest struct {
	UserID uuid.UUID `json:"user_id"`
}

// AssignTicket - назначение тикета оператору
func (h *QueueHandler) AssignTicket(c *gin.Context) {
	currentUserID, _ := GetUserIDFromContext(c)

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	var req AssignTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Проверяем, что пользователь существует и активен
	var targetUser db.User
	if err := h.db.Where("id = ? AND is_active = true", req.UserID).First(&targetUser).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Target user not found or inactive"})
		return
	}

	// Проверяем, что текущий пользователь имеет права
	var currentUser db.User
	if err := h.db.Where("id = ?", currentUserID).First(&currentUser).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Current user not found"})
		return
	}

	// Только админы могут назначать на других операторов
	if currentUser.Role != "admin" && currentUserID != req.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only admins can assign tickets to other users"})
		return
	}

	// Назначаем тикет
	if err := h.queue.AssignTicket(c.Request.Context(), ticketID, req.UserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign ticket"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Ticket assigned successfully",
		"ticket_id":   ticketID,
		"assigned_to": req.UserID,
	})
}

// UnassignTicket - снятие назначения с тикета
func (h *QueueHandler) UnassignTicket(c *gin.Context) {
	currentUserID, _ := GetUserIDFromContext(c)

	ticketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	// Получаем тикет
	var ticket db.Ticket
	if err := h.db.Where("id = ?", ticketID).First(&ticket).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	// Получаем текущего пользователя
	var currentUser db.User
	if err := h.db.Where("id = ?", currentUserID).First(&currentUser).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Current user not found"})
		return
	}

	// Проверяем права
	if currentUser.Role != "admin" && ticket.AssignedTo != nil && *ticket.AssignedTo != currentUserID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "You can only unassign tickets assigned to yourself",
		})
		return
	}

	// Снимаем назначение
	if err := h.queue.UnassignTicket(c.Request.Context(), ticketID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unassign ticket"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":   "Ticket unassigned successfully",
		"ticket_id": ticketID,
	})
}

// TakeNextTicket - взять следующий тикет из очереди
func (h *QueueHandler) TakeNextTicket(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)

	// Получаем пользователя
	var user db.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Берем следующий тикет из очереди
	nextTicketID, err := h.queue.GetNextTicket(c.Request.Context(), user.DispatcherID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get next ticket"})
		return
	}

	if nextTicketID == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "No tickets in queue",
		})
		return
	}

	// Назначаем на текущего пользователя
	if err := h.queue.AssignTicket(c.Request.Context(), *nextTicketID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign ticket"})
		return
	}

	// Получаем полную информацию о тикете
	var ticket db.Ticket
	if err := h.db.
		Preload("Client").
		Preload("Channel").
		Where("id = ?", nextTicketID).
		First(&ticket).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Ticket assigned to you",
		"ticket":  ticket,
	})
}

// GetMyTickets - получение тикетов, назначенных текущему пользователю
func (h *QueueHandler) GetMyTickets(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)

	status := c.Query("status")
	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "20")

	pageInt := 1
	limitInt := 20
	fmt.Sscanf(page, "%d", &pageInt)
	fmt.Sscanf(limit, "%d", &limitInt)

	offset := (pageInt - 1) * limitInt
	if offset < 0 {
		offset = 0
	}

	// Строим запрос
	query := h.db.Model(&db.Ticket{}).
		Preload("Client").
		Preload("Channel").
		Where("assigned_to = ?", userID)

	if status != "" {
		query = query.Where("status = ?", status)
	}

	var total int64
	query.Count(&total)

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
