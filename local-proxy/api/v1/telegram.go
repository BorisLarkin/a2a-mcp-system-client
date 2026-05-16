// ./local-proxy/api/v1/telegram.go
package v1

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"local-proxy/internal/config"
	"local-proxy/internal/db"
	"local-proxy/internal/queue"
	"local-proxy/internal/services"
	"local-proxy/internal/websocket"
)

type TelegramHandler struct {
	db        *gorm.DB
	queue     *queue.TicketQueue
	config    *config.Config
	wsManager *websocket.Manager
	processor *services.TicketProcessor // ← добавить
}

func NewTelegramHandler(db *gorm.DB, queue *queue.TicketQueue, config *config.Config, wsManager *websocket.Manager, processor *services.TicketProcessor) *TelegramHandler {
	return &TelegramHandler{
		db:        db,
		queue:     queue,
		config:    config,
		wsManager: wsManager,
		processor: processor,
	}
}

// TelegramUpdate - структура обновления от Telegram
type TelegramUpdate struct {
	UpdateID      int64                  `json:"update_id"`
	Message       *TelegramMessage       `json:"message,omitempty"`
	CallbackQuery *TelegramCallbackQuery `json:"callback_query,omitempty"`
}

// TelegramMessage - структура сообщения Telegram
type TelegramMessage struct {
	MessageID int64 `json:"message_id"`
	From      struct {
		ID           int64  `json:"id"`
		IsBot        bool   `json:"is_bot"`
		FirstName    string `json:"first_name"`
		LastName     string `json:"last_name,omitempty"`
		Username     string `json:"username,omitempty"`
		LanguageCode string `json:"language_code,omitempty"`
	} `json:"from"`
	Chat struct {
		ID        int64  `json:"id"`
		Type      string `json:"type"` // "private", "group", "supergroup", "channel"
		Title     string `json:"title,omitempty"`
		Username  string `json:"username,omitempty"`
		FirstName string `json:"first_name,omitempty"`
		LastName  string `json:"last_name,omitempty"`
	} `json:"chat"`
	Date     int64         `json:"date"`
	Text     string        `json:"text,omitempty"`
	Caption  string        `json:"caption,omitempty"`
	Photo    []interface{} `json:"photo,omitempty"`
	Document interface{}   `json:"document,omitempty"`
	Audio    interface{}   `json:"audio,omitempty"`
	Video    interface{}   `json:"video,omitempty"`
	Voice    interface{}   `json:"voice,omitempty"`
	Location interface{}   `json:"location,omitempty"`
	Contact  interface{}   `json:"contact,omitempty"`
}

// TelegramCallbackQuery - структура callback query Telegram
type TelegramCallbackQuery struct {
	ID   string `json:"id"`
	From struct {
		ID        int64  `json:"id"`
		IsBot     bool   `json:"is_bot"`
		FirstName string `json:"first_name"`
		Username  string `json:"username,omitempty"`
	} `json:"from"`
	Message struct {
		MessageID int64 `json:"message_id"`
		Chat      struct {
			ID    int64  `json:"id"`
			Type  string `json:"type"`
			Title string `json:"title,omitempty"`
		} `json:"chat"`
		Date int64  `json:"date"`
		Text string `json:"text,omitempty"`
	} `json:"message,omitempty"`
	ChatInstance string `json:"chat_instance"`
	Data         string `json:"data,omitempty"`
}

// HandleWebhook - обработка webhook от Telegram
func (h *TelegramHandler) HandleWebhook(c *gin.Context) {
	var update struct {
		Message struct {
			MessageID int `json:"message_id"`
			Chat      struct {
				ID int64 `json:"id"`
			} `json:"chat"`
			Text string `json:"text"`
		} `json:"message"`
		CallbackQuery struct {
			ID      string `json:"id"`
			Data    string `json:"data"`
			Message struct {
				Chat struct {
					ID int64 `json:"id"`
				} `json:"chat"`
			} `json:"message"`
		} `json:"callback_query"`
	}
	if err := c.ShouldBindJSON(&update); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	if update.Message.Text != "" {
		// Новое сообщение от клиента
		chatID := strconv.FormatInt(update.Message.Chat.ID, 10)
		text := update.Message.Text

		// Ищем существующий открытый тикет для этого клиента
		var ticket db.Ticket
		h.db.Joins("JOIN clients ON clients.id = tickets.client_id").
			Where("clients.external_id = ? AND tickets.status NOT IN ('resolved','closed')", chatID).
			First(&ticket)

		if ticket.ID != uuid.Nil {
			// Добавляем сообщение к тикету
			msg := db.TicketMessage{
				TicketID:    ticket.ID,
				SenderType:  "client",
				MessageText: text,
			}
			h.db.Create(&msg)
			c.JSON(200, gin.H{"status": "message added"})
		} else {
			// Создаём новый тикет
			h.createTicketFromTelegram(text, chatID)
			c.JSON(201, gin.H{"status": "new ticket"})
		}
	} else if update.CallbackQuery.Data != "" {
		// Обработка кнопок
		h.handleCallback(update.CallbackQuery)
		c.JSON(200, gin.H{"status": "ok"})
	}
}

// handleMessage - обработка входящего сообщения
func (h *TelegramHandler) handleMessage(ctx context.Context, msg *TelegramMessage) {
	// Находим канал Telegram
	var channel db.Channel
	if err := h.db.WithContext(ctx).Where("type = ? AND config->>'chat_id' = ?", "telegram", fmt.Sprintf("%d", msg.Chat.ID)).
		First(&channel).Error; err != nil {
		// Канал не найден, возможно нужно создать
		h.createTelegramChannel(msg)
		return
	}

	// Находим или создаем клиента
	client := h.findOrCreateClient(msg, channel.ID)

	// Создаем тикет или находим активный
	ticket := h.findOrCreateTicket(ctx, msg, client.ID, channel.DispatcherID)

	// Создаем сообщение
	message := db.TicketMessage{
		TicketID:    ticket.ID,
		SenderType:  "client",
		SenderID:    &client.ID,
		MessageText: msg.Text,
		Metadata: map[string]interface{}{
			"telegram_message_id": msg.MessageID,
			"telegram_chat_id":    msg.Chat.ID,
			"telegram_from_id":    msg.From.ID,
			"telegram_date":       msg.Date,
		},
		CreatedAt: time.Unix(msg.Date, 0),
	}

	if err := h.db.WithContext(ctx).Create(&message).Error; err != nil {
		fmt.Printf("Failed to create message: %v\n", err)
		return
	}

	// Обновляем тикет
	ticket.UpdatedAt = time.Now()
	h.db.WithContext(ctx).Save(&ticket)

	// Если тикет новый, добавляем в очередь
	if ticket.Status == "new" {
		// Передаем только нужные данные, не весь gin.Context
		h.queue.AddTicket(ctx, ticket.ID, channel.DispatcherID)

		// Отправляем уведомление операторам
		h.wsManager.SendToRole("operator", "telegram_message", gin.H{
			"ticket_id": ticket.ID,
			"client":    client.Name,
			"message":   msg.Text,
			"timestamp": message.CreatedAt,
		})
	}

	// Автоматический ответ если включен AI
	h.maybeSendAutoResponse(ctx, ticket, channel)
}

// createTelegramChannel - создание канала Telegram
func (h *TelegramHandler) createTelegramChannel(msg *TelegramMessage) {
	// Берем первую диспетчерскую (в реальной системе нужно спрашивать у администратора)
	var dispatcher db.Dispatcher
	if err := h.db.First(&dispatcher).Error; err != nil {
		return
	}

	channelName := "Telegram Chat"
	if msg.Chat.Type != "private" {
		channelName = msg.Chat.Title
	}

	channel := db.Channel{
		DispatcherID: dispatcher.ID,
		Type:         "telegram",
		Name:         channelName,
		Config: map[string]interface{}{
			"chat_id":    fmt.Sprintf("%d", msg.Chat.ID),
			"chat_type":  msg.Chat.Type,
			"chat_title": msg.Chat.Title,
			"username":   msg.Chat.Username,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.db.Create(&channel).Error; err != nil {
		fmt.Printf("Failed to create channel: %v\n", err)
	}
}

// findOrCreateClient - поиск или создание клиента
func (h *TelegramHandler) findOrCreateClient(msg *TelegramMessage, channelID uuid.UUID) *db.Client {
	externalID := fmt.Sprintf("telegram:%d", msg.From.ID)

	var client db.Client
	if err := h.db.Where("external_id = ?", externalID).First(&client).Error; err != nil {
		// Создаем нового клиента
		clientName := msg.From.FirstName
		if msg.From.LastName != "" {
			clientName += " " + msg.From.LastName
		}

		client = db.Client{
			ExternalID:  externalID,
			ChannelID:   &channelID,
			Name:        clientName,
			ContactInfo: msg.From.Username,
			Metadata: map[string]interface{}{
				"telegram_user_id":    msg.From.ID,
				"telegram_username":   msg.From.Username,
				"telegram_first_name": msg.From.FirstName,
				"telegram_last_name":  msg.From.LastName,
				"language_code":       msg.From.LanguageCode,
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		h.db.Create(&client)
	}

	return &client
}

// findOrCreateTicket - поиск или создание тикета
func (h *TelegramHandler) findOrCreateTicket(ctx context.Context, msg *TelegramMessage, clientID uuid.UUID, dispatcherID uuid.UUID) *db.Ticket {
	// Ищем активный тикет (не закрытый и не решенный)
	var ticket db.Ticket
	if err := h.db.WithContext(ctx).Where("client_id = ? AND status NOT IN (?)",
		clientID, []string{"resolved", "closed"}).
		Order("created_at DESC").
		First(&ticket).Error; err != nil {

		// Создаем новый тикет
		subject := "Сообщение из Telegram"
		if len(msg.Text) > 50 {
			subject = msg.Text[:50] + "..."
		} else if msg.Text != "" {
			subject = msg.Text
		}

		ticket = db.Ticket{
			DispatcherID: dispatcherID,
			ClientID:     &clientID,
			Subject:      subject,
			OriginalText: msg.Text,
			Status:       "new",
			Priority:     "medium",
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
		}

		h.db.WithContext(ctx).Create(&ticket)
	}

	return &ticket
}

// maybeSendAutoResponse - отправка автоматического ответа если включен AI
func (h *TelegramHandler) maybeSendAutoResponse(ctx context.Context, ticket *db.Ticket, channel db.Channel) {
	// Проверяем AI настройки
	var aiSettings db.AISettings
	if err := h.db.WithContext(ctx).Where("dispatcher_id = ?", channel.DispatcherID).First(&aiSettings).Error; err != nil {
		return
	}

	if !aiSettings.Enabled || !aiSettings.AutoRespond {
		return
	}

	// TODO: Реализовать вызов оркестратора для генерации ответа
	// Пока просто отправляем подтверждение получения
	/*
	   response, err := h.orchestrator.GenerateResponse(ctx, ticket.OriginalText)
	   if err == nil && response.Confidence >= aiSettings.ConfidenceThreshold {
	       h.sendTelegramMessage(channel.Config["chat_id"].(string), response.Text)

	       // Сохраняем AI ответ в тикет
	       ticket.AIResponse = response.Text
	       ticket.AIProcessedAt = time.Now()
	       h.db.WithContext(ctx).S
	}
}

funcave(ticket)
	   }
	*/
}

// handleCallbackQuery - обработка callback query
func (h *TelegramHandler) handleCallbackQuery(ctx context.Context, query *TelegramCallbackQuery) {
	// TODO: Реализовать обработку кнопок
	// Например: подтверждение, отмена, выбор вариантов и т.д.
}

// SendTelegramMessageRequest - запрос на отправку сообщения в Telegram
type SendTelegramMessageRequest struct {
	ChatID      string      `json:"chat_id" binding:"required"`
	Message     string      `json:"message" binding:"required"`
	ParseMode   string      `json:"parse_mode,omitempty"` // "HTML", "Markdown", "MarkdownV2"
	ReplyMarkup interface{} `json:"reply_markup,omitempty"`
}

// SendTelegramMessage - отправка сообщения в Telegram
func (h *TelegramHandler) SendTelegramMessage(c *gin.Context) {
	// Эта функция используется операторами для отправки ответов через Telegram

	userID, _ := GetUserIDFromContext(c)

	var req SendTelegramMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Проверяем, что пользователь имеет доступ к этому чату
	var channel db.Channel
	if err := h.db.Where("type = ? AND config->>'chat_id' = ?", "telegram", req.ChatID).
		First(&channel).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}

	var user db.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if user.Role != "admin" && channel.DispatcherID != user.DispatcherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// TODO: Реализовать отправку через Telegram Bot API
	// Для этого нужно получить токен бота из конфигурации канала

	c.JSON(http.StatusOK, gin.H{
		"message": "Message sent successfully",
		"chat_id": req.ChatID,
	})
}

// createTicketFromTelegram — создаёт новый тикет из сообщения Telegram
func (h *TelegramHandler) createTicketFromTelegram(text string, chatID string) (*db.Ticket, error) {
	// Находим или создаём клиента
	var client db.Client
	if err := h.db.Where("external_id = ?", chatID).First(&client).Error; err != nil {
		client = db.Client{
			ExternalID:  chatID,
			Name:        "Telegram User " + chatID,
			ContactInfo: chatID,
		}
		h.db.Create(&client)
	}

	// Находим канал Telegram
	var channel db.Channel
	h.db.Where("type = ? AND config->>'chat_id' = ?", "telegram", chatID).First(&channel)

	// Создаём тикет
	ticket := db.Ticket{
		DispatcherID: channel.DispatcherID,
		ClientID:     &client.ID,
		ChannelID:    &channel.ID,
		Subject:      truncateText(text, 100),
		OriginalText: text,
		Status:       "new",
		Priority:     "medium",
	}
	h.db.Create(&ticket)

	// Добавляем сообщение
	msg := db.TicketMessage{
		TicketID:    ticket.ID,
		SenderType:  "client",
		SenderID:    &client.ID,
		MessageText: text,
	}
	h.db.Create(&msg)

	if h.processor != nil {
		go h.processor.ProcessTicket(ticket.ID)
	}

	return &ticket, nil
}

// handleCallback — обработка callback query (кнопок)
func (h *TelegramHandler) handleCallback(query struct {
	ID      string `json:"id"`
	Data    string `json:"data"`
	Message struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
	} `json:"message"`
}) {
	// Разбираем callback_data: "resolved:uuid" или "escalate:uuid"
	parts := strings.Split(query.Data, ":")
	if len(parts) != 2 {
		return
	}
	action := parts[0]
	ticketID, err := uuid.Parse(parts[1])
	if err != nil {
		return
	}

	switch action {
	case "resolved":
		h.db.Model(&db.Ticket{}).Where("id = ?", ticketID).Updates(map[string]interface{}{
			"status":          "resolved",
			"feedback_status": "resolved",
			"resolved_at":     time.Now(),
		})
	case "escalate":
		h.db.Model(&db.Ticket{}).Where("id = ?", ticketID).Updates(map[string]interface{}{
			"status":          "waiting",
			"feedback_status": "escalate",
			"escalated_at":    time.Now(),
		})
		// Добавляем в очередь
		var ticket db.Ticket
		if err := h.db.First(&ticket, "id = ?", ticketID).Error; err == nil {
			h.queue.AddTicket(context.Background(), ticketID, ticket.DispatcherID)
		}
	}
}

func truncateText(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
