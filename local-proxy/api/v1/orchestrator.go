// ./local-proxy/api/v1/orchestrator.go
package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"local-proxy/internal/config"
	"local-proxy/internal/db"
)

type OrchestratorHandler struct {
	db     *gorm.DB
	config *config.Config
	redis  *redis.Client
	client *http.Client
}

func NewOrchestratorHandler(db *gorm.DB, config *config.Config, redis *redis.Client) *OrchestratorHandler {
	return &OrchestratorHandler{
		db:     db,
		config: config,
		redis:  redis,
		client: &http.Client{
			Timeout: config.Orchestrator.Timeout,
		},
	}
}

// ClassifyRequest - запрос на классификацию
type ClassifyRequest struct {
	Text     string `json:"text" binding:"required"`
	TicketID string `json:"ticket_id,omitempty"`
}

// ClassifyResponse - ответ классификации
type ClassifyResponse struct {
	Category    string                 `json:"category"`
	Confidence  float64                `json:"confidence"`
	Entities    []string               `json:"entities"`
	Intent      string                 `json:"intent,omitempty"`
	Sentiment   string                 `json:"sentiment,omitempty"`
	Urgency     string                 `json:"urgency,omitempty"`
	Suggestions []string               `json:"suggestions,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Classify - классификация текста через оркестратор
func (h *OrchestratorHandler) Classify(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)

	var req ClassifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Проверяем кеш
	cacheKey := fmt.Sprintf("classification:%s", req.Text)
	var cachedResponse ClassifyResponse
	if err := h.redis.Get(c.Request.Context(), cacheKey).Scan(&cachedResponse); err == nil {
		c.JSON(http.StatusOK, cachedResponse)
		return
	}

	// Получаем диспетчерскую пользователя
	var user db.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var dispatcher db.Dispatcher
	if err := h.db.Where("id = ?", user.DispatcherID).First(&dispatcher).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Dispatcher not found"})
		return
	}

	// Подготавливаем запрос к оркестратору
	orchestratorReq := map[string]interface{}{
		"text":          req.Text,
		"dispatcher_id": dispatcher.OrchestratorDispatcherID,
		"api_key":       dispatcher.OrchestratorAPIKey,
		"request_type":  "classification",
		"metadata": map[string]interface{}{
			"user_id":   userID.String(),
			"ticket_id": req.TicketID,
			"timestamp": time.Now().Unix(),
		},
	}

	// Отправляем запрос
	response, err := h.callOrchestrator(c.Request.Context(), "classify", orchestratorReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to classify text", "details": err.Error()})
		return
	}

	// Парсим ответ
	var classifyResp ClassifyResponse
	if err := json.Unmarshal(response, &classifyResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response"})
		return
	}

	// Сохраняем в кеш (на 1 час)
	h.redis.Set(c.Request.Context(), cacheKey, classifyResp, time.Hour)

	// Логируем запрос
	if req.TicketID != "" {
		ticketID, _ := uuid.Parse(req.TicketID)
		h.logOrchestratorCall(c.Request.Context(), user.DispatcherID, &ticketID, "classify", orchestratorReq, classifyResp, nil)
	}

	c.JSON(http.StatusOK, classifyResp)
}

// GenerateRequest - запрос на генерацию ответа
type GenerateRequest struct {
	TicketID          uuid.UUID              `json:"ticket_id" binding:"required"`
	Context           string                 `json:"context"`
	PreviousMessages  []string               `json:"previous_messages,omitempty"`
	Style             string                 `json:"style,omitempty"`
	AdditionalContext map[string]interface{} `json:"additional_context,omitempty"`
}

// GenerateResponse - генерация ответа через оркестратор
func (h *OrchestratorHandler) GenerateResponse(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)

	var req GenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Получаем тикет
	var ticket db.Ticket
	if err := h.db.
		Preload("Client").
		Preload("Channel").
		Where("id = ?", req.TicketID).
		First(&ticket).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	// Проверяем доступ
	var user db.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	if user.Role != "admin" && ticket.DispatcherID != user.DispatcherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Получаем диспетчерскую
	var dispatcher db.Dispatcher
	if err := h.db.Where("id = ?", user.DispatcherID).First(&dispatcher).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Dispatcher not found"})
		return
	}

	// Получаем AI настройки
	var aiSettings db.AISettings
	if err := h.db.Where("dispatcher_id = ?", user.DispatcherID).First(&aiSettings).Error; err != nil {
		// Используем настройки по умолчанию
		aiSettings = db.AISettings{
			CommunicationStyle:  "balanced",
			ConfidenceThreshold: 0.7,
		}
	}

	// Получаем сообщения тикета
	var messages []db.TicketMessage
	if err := h.db.Where("ticket_id = ?", req.TicketID).
		Order("created_at ASC").
		Find(&messages).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load messages"})
		return
	}

	// Формируем историю сообщений
	var messageHistory []string
	for _, msg := range messages {
		messageHistory = append(messageHistory, msg.MessageText)
	}

	// Формируем запрос к оркестратору
	orchestratorReq := map[string]interface{}{
		"ticket_id":       req.TicketID.String(),
		"original_text":   ticket.OriginalText,
		"message_history": messageHistory,
		"dispatcher_id":   dispatcher.OrchestratorDispatcherID,
		"api_key":         dispatcher.OrchestratorAPIKey,
		"request_type":    "generate",
		"config": map[string]interface{}{
			"communication_style":  req.Style,
			"confidence_threshold": aiSettings.ConfidenceThreshold,
			"use_internet_search":  aiSettings.UseInternetSearch,
		},
		"metadata": map[string]interface{}{
			"user_id":   userID.String(),
			"category":  ticket.Category,
			"priority":  ticket.Priority,
			"timestamp": time.Now().Unix(),
		},
	}

	// Добавляем дополнительный контекст если есть
	if req.Context != "" {
		orchestratorReq["context"] = req.Context
	}
	if req.AdditionalContext != nil {
		orchestratorReq["additional_context"] = req.AdditionalContext
	}

	// Отправляем запрос
	response, err := h.callOrchestrator(c.Request.Context(), "generate", orchestratorReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate response", "details": err.Error()})
		return
	}

	// Парсим ответ
	var generateResp map[string]interface{}
	if err := json.Unmarshal(response, &generateResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response"})
		return
	}

	// Сохраняем AI ответ в тикет
	if aiResponse, ok := generateResp["response"].(string); ok {
		ticket.AIResponse = aiResponse
		ticket.AIProcessedAt = &[]time.Time{time.Now()}[0]

		// Сохраняем анализ если есть
		if analysis, ok := generateResp["analysis"].(map[string]interface{}); ok {
			ticket.AIAnalysis = db.JSONB(analysis)
		}

		if err := h.db.Save(&ticket).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save AI response"})
			return
		}
	}

	// Логируем запрос
	h.logOrchestratorCall(c.Request.Context(), user.DispatcherID, &req.TicketID, "generate", orchestratorReq, generateResp, nil)

	c.JSON(http.StatusOK, generateResp)
}

// callOrchestrator - вызов удаленного оркестратора
func (h *OrchestratorHandler) callOrchestrator(ctx context.Context, endpoint string, data interface{}) ([]byte, error) {
	url := fmt.Sprintf("%s/api/v1/%s", h.config.Orchestrator.URL, endpoint)

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+h.config.Orchestrator.APIKey)
	req.Header.Set("X-Client-Version", "local-proxy/v1.0")

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("orchestrator returned status %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// logOrchestratorCall - логирование вызова оркестратора
func (h *OrchestratorHandler) logOrchestratorCall(ctx context.Context, dispatcherID uuid.UUID, ticketID *uuid.UUID, requestType string, requestData, responseData interface{}, err error) {
	logEntry := db.OrchestratorLog{
		DispatcherID: dispatcherID,
		TicketID:     ticketID,
		RequestType:  requestType,
		RequestData:  db.JSONB(requestData.(map[string]interface{})),
		CreatedAt:    time.Now(),
	}

	if responseData != nil {
		logEntry.ResponseData = db.JSONB(responseData.(map[string]interface{}))
		logEntry.StatusCode = 200
	}

	if err != nil {
		logEntry.ErrorMessage = err.Error()
		logEntry.StatusCode = 500
	}

	h.db.Create(&logEntry)
}

// GetOrchestratorLogs - получение логов вызовов оркестратора
func (h *OrchestratorHandler) GetOrchestratorLogs(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)

	// Получаем пользователя
	var user db.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Только админы могут смотреть логи
	if user.Role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "50")
	ticketID := c.Query("ticket_id")
	requestType := c.Query("request_type")

	pageInt := 1
	limitInt := 50
	fmt.Sscanf(page, "%d", &pageInt)
	fmt.Sscanf(limit, "%d", &limitInt)

	offset := (pageInt - 1) * limitInt
	if offset < 0 {
		offset = 0
	}

	// Строим запрос
	query := h.db.Model(&db.OrchestratorLog{}).
		Where("dispatcher_id = ?", user.DispatcherID)

	if ticketID != "" {
		ticketUUID, err := uuid.Parse(ticketID)
		if err == nil {
			query = query.Where("ticket_id = ?", ticketUUID)
		}
	}

	if requestType != "" {
		query = query.Where("request_type = ?", requestType)
	}

	var total int64
	query.Count(&total)

	var logs []db.OrchestratorLog
	if err := query.
		Order("created_at DESC").
		Offset(offset).
		Limit(limitInt).
		Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load logs"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"logs":  logs,
		"total": total,
		"page":  pageInt,
		"limit": limitInt,
	})
}
