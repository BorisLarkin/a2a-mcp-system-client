// ./local-proxy/api/v1/admin.go
package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"local-proxy/api/middleware"
	"local-proxy/internal/config"
	"local-proxy/internal/db"
	"local-proxy/pkg/utils"
)

type AdminHandler struct {
	db     *gorm.DB
	config *config.Config
}

func NewAdminHandler(db *gorm.DB, config *config.Config) *AdminHandler {
	return &AdminHandler{
		db:     db,
		config: config,
	}
}

// GetSettings - получение настроек диспетчерской
func (h *AdminHandler) GetSettings(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)

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

	var aiSettings db.AISettings
	h.db.Where("dispatcher_id = ?", user.DispatcherID).First(&aiSettings)

	// Получаем статистику
	var ticketStats struct {
		Total      int64 `json:"total"`
		New        int64 `json:"new"`
		InProgress int64 `json:"in_progress"`
		Resolved   int64 `json:"resolved"`
		Closed     int64 `json:"closed"`
	}

	h.db.Model(&db.Ticket{}).
		Where("dispatcher_id = ?", user.DispatcherID).
		Count(&ticketStats.Total)

	h.db.Model(&db.Ticket{}).
		Where("dispatcher_id = ? AND status = ?", user.DispatcherID, "new").
		Count(&ticketStats.New)

	h.db.Model(&db.Ticket{}).
		Where("dispatcher_id = ? AND status = ?", user.DispatcherID, "in_progress").
		Count(&ticketStats.InProgress)

	h.db.Model(&db.Ticket{}).
		Where("dispatcher_id = ? AND status = ?", user.DispatcherID, "resolved").
		Count(&ticketStats.Resolved)

	h.db.Model(&db.Ticket{}).
		Where("dispatcher_id = ? AND status = ?", user.DispatcherID, "closed").
		Count(&ticketStats.Closed)

	// Получаем каналы
	var channels []db.Channel
	h.db.Where("dispatcher_id = ?", user.DispatcherID).Find(&channels)

	// Получаем операторов
	var operators []db.User
	h.db.Where("dispatcher_id = ? AND role IN (?)",
		user.DispatcherID, []string{"admin", "operator"}).Find(&operators)

	c.JSON(http.StatusOK, gin.H{
		"dispatcher":   dispatcher,
		"ai_settings":  aiSettings,
		"ticket_stats": ticketStats,
		"channels":     channels,
		"operators":    operators,
		"server_info": gin.H{
			"version": "1.0.0",
			"uptime":  time.Since(h.config.Server.StartTime).String(),
		},
	})
}

// UpdateSettingsRequest - обновление настроек
type UpdateSettingsRequest struct {
	DispatcherName     string                 `json:"dispatcher_name"`
	OrchestratorAPIKey string                 `json:"orchestrator_api_key,omitempty"`
	Settings           map[string]interface{} `json:"settings"`
	AISettings         struct {
		Enabled             *bool    `json:"enabled"`
		AutoRespond         *bool    `json:"auto_respond"`
		ConfidenceThreshold *float64 `json:"confidence_threshold"`
		UseInternetSearch   *bool    `json:"use_internet_search"`
		CommunicationStyle  *string  `json:"communication_style"`
		SystemContext       *string  `json:"system_context"`
	} `json:"ai_settings"`
}

// UpdateSettings - обновление настроек диспетчерской
func (h *AdminHandler) UpdateSettings(c *gin.Context) {
	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	// Находим диспетчерскую (первую активную)
	var dispatcher db.Dispatcher
	if err := h.db.Where("is_active = ?", true).First(&dispatcher).Error; err != nil {
		c.JSON(404, gin.H{"error": "Dispatcher not found"})
		return
	}

	// Обновляем AISettings
	var aiSettings db.AISettings
	result := h.db.Where("dispatcher_id = ?", dispatcher.ID).First(&aiSettings)
	if result.Error != nil {
		// Создаём, если нет
		aiSettings = db.AISettings{DispatcherID: dispatcher.ID}
		h.db.Create(&aiSettings)
		h.db.Where("dispatcher_id = ?", dispatcher.ID).First(&aiSettings)
	}

	updates := make(map[string]interface{})

	if v, ok := req["enabled"]; ok {
		updates["enabled"] = v
	}
	if v, ok := req["auto_respond"]; ok {
		updates["auto_respond"] = v
	}
	if v, ok := req["confidence_threshold"]; ok {
		updates["confidence_threshold"] = v
	}
	if v, ok := req["communication_style"]; ok {
		updates["communication_style"] = v
	}
	if v, ok := req["system_context"]; ok {
		updates["system_context"] = v
	}
	if v, ok := req["use_internet_search"]; ok {
		updates["use_internet_search"] = v
	}
	if v, ok := req["escalation_timeout_minutes"]; ok {
		updates["escalation_timeout_minutes"] = v
	}

	if len(updates) > 0 {
		updates["updated_at"] = time.Now()
		h.db.Model(&aiSettings).Updates(updates)
	}

	// Перезагружаем, чтобы вернуть актуальные данные
	h.db.Where("dispatcher_id = ?", dispatcher.ID).First(&aiSettings)

	// Синхронизируем с оркестратором
	go func(dispID uuid.UUID, apiKey string) {
		configJSON, _ := json.Marshal(map[string]interface{}{
			"communication_style":  req["communication_style"],
			"confidence_threshold": req["confidence_threshold"],
			"company_context":      req["system_context"],
		})

		orchURL := h.config.Orchestrator.URL + "/api/v1/dispatchers/" + dispID.String() + "/config"
		httpClient := &http.Client{Timeout: 10 * time.Second}
		orchReq, _ := http.NewRequest("PUT", orchURL, bytes.NewReader(configJSON))
		orchReq.Header.Set("Content-Type", "application/json")
		orchReq.Header.Set("X-API-Key", apiKey)
		httpClient.Do(orchReq)
	}(dispatcher.ID, dispatcher.OrchestratorAPIKey)

	c.JSON(200, gin.H{
		"message":     "Settings updated successfully",
		"ai_settings": aiSettings,
		"dispatcher":  dispatcher,
	})
}

// ListUsers - список пользователей
func (h *AdminHandler) ListUsers(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)

	var currentUser db.User
	if err := h.db.Where("id = ?", userID).First(&currentUser).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	page := c.DefaultQuery("page", "1")
	limit := c.DefaultQuery("limit", "50")
	role := c.Query("role")

	pageInt := 1
	limitInt := 50
	fmt.Sscanf(page, "%d", &pageInt)
	fmt.Sscanf(limit, "%d", &limitInt)

	offset := (pageInt - 1) * limitInt
	if offset < 0 {
		offset = 0
	}

	// Строим запрос
	query := h.db.Model(&db.User{}).
		Where("dispatcher_id = ?", currentUser.DispatcherID)

	if role != "" {
		query = query.Where("role = ?", role)
	}

	var total int64
	query.Count(&total)

	var users []db.User
	if err := query.
		Select("id", "username", "email", "full_name", "role", "is_active", "last_login_at", "created_at").
		Order("created_at DESC").
		Offset(offset).
		Limit(limitInt).
		Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load users"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"users": users,
		"total": total,
		"page":  pageInt,
		"limit": limitInt,
	})
}

// CreateUserRequest - создание пользователя
type CreateUserRequest struct {
	Username string `json:"username" binding:"required,min=3,max=100"`
	Email    string `json:"email" binding:"required,email"`
	FullName string `json:"full_name" binding:"required"`
	Role     string `json:"role" binding:"required,oneof=admin operator viewer"`
	Password string `json:"password" binding:"required,min=8"`
}

// CreateUser - создание нового пользователя
func (h *AdminHandler) CreateUser(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)

	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	var currentUser db.User
	if err := h.db.Where("id = ?", userID).First(&currentUser).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Проверяем, что username уникален
	var existingUser db.User
	if err := h.db.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Username already exists"})
		return
	}

	// Хешируем пароль
	passwordHash, err := utils.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to hash password"})
		return
	}

	// Создаем пользователя
	user := db.User{
		DispatcherID: currentUser.DispatcherID,
		Username:     req.Username,
		Email:        req.Email,
		FullName:     req.FullName,
		Role:         req.Role,
		PasswordHash: passwordHash,
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := h.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Очищаем пароль из ответа
	user.PasswordHash = ""

	c.JSON(http.StatusCreated, gin.H{
		"message": "User created successfully",
		"user":    user,
	})
}

// UpdateUserRequest - обновление пользователя
type UpdateUserRequest struct {
	Email    string `json:"email,omitempty"`
	FullName string `json:"full_name,omitempty"`
	Role     string `json:"role,omitempty" enums:"admin,operator,viewer"`
	Password string `json:"password,omitempty"`
	IsActive *bool  `json:"is_active,omitempty"`
}

// UpdateUser - обновление пользователя
func (h *AdminHandler) UpdateUser(c *gin.Context) {
	currentUserID, _ := GetUserIDFromContext(c)

	targetUserID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var currentUser db.User
	if err := h.db.Where("id = ?", currentUserID).First(&currentUser).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Current user not found"})
		return
	}

	// Проверяем, что обновляем пользователя своей диспетчерской
	var targetUser db.User
	if err := h.db.Where("id = ?", targetUserID).First(&targetUser).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Target user not found"})
		return
	}

	if targetUser.DispatcherID != currentUser.DispatcherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Cannot update user from another dispatcher"})
		return
	}

	// Обновляем поля
	updates := make(map[string]interface{})

	if req.Email != "" {
		updates["email"] = req.Email
	}

	if req.FullName != "" {
		updates["full_name"] = req.FullName
	}

	if req.Role != "" {
		updates["role"] = req.Role
	}

	if req.Password != "" {
		passwordHash, err := utils.HashPassword(req.Password)
		if err == nil {
			updates["password_hash"] = passwordHash
		}
	}

	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	updates["updated_at"] = time.Now()

	if err := h.db.Model(&targetUser).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "User updated successfully",
		"user":    targetUser,
	})
}

// ListChannels - список каналов связи
func (h *AdminHandler) ListChannels(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)

	var user db.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var channels []db.Channel
	if err := h.db.Where("dispatcher_id = ?", user.DispatcherID).
		Order("created_at DESC").
		Find(&channels).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load channels"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"channels": channels,
		"count":    len(channels),
	})
}

// CreateChannelRequest - создание канала
type CreateChannelRequest struct {
	Type   string                 `json:"type" binding:"required,oneof=telegram email web"`
	Name   string                 `json:"name" binding:"required"`
	Config map[string]interface{} `json:"config" binding:"required"`
}

// CreateChannel - создание нового канала связи
func (h *AdminHandler) CreateChannel(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)

	var req CreateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var user db.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Проверяем, что нет канала с таким именем
	var existingChannel db.Channel
	if err := h.db.Where("dispatcher_id = ? AND name = ?", user.DispatcherID, req.Name).
		First(&existingChannel).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Channel with this name already exists"})
		return
	}

	// Создаем канал
	channel := db.Channel{
		DispatcherID: user.DispatcherID,
		Type:         req.Type,
		Name:         req.Name,
		Config:       db.JSONB(req.Config),
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := h.db.Create(&channel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create channel"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Channel created successfully",
		"channel": channel,
	})
}

// UpdateChannel - обновление канала
func (h *AdminHandler) UpdateChannel(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)

	channelID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid channel ID"})
		return
	}

	var updateData map[string]interface{}
	if err := c.ShouldBindJSON(&updateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var user db.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var channel db.Channel
	if err := h.db.Where("id = ?", channelID).First(&channel).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}

	if channel.DispatcherID != user.DispatcherID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Обновляем разрешенные поля
	if name, ok := updateData["name"].(string); ok {
		channel.Name = name
	}

	if config, ok := updateData["config"].(map[string]interface{}); ok {
		channel.Config = db.JSONB(config)
	}

	if isActive, ok := updateData["is_active"].(bool); ok {
		channel.IsActive = isActive
	}

	channel.UpdatedAt = time.Now()

	if err := h.db.Save(&channel).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update channel"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Channel updated successfully",
		"channel": channel,
	})
}

// GetAnalytics - получение аналитики
func (h *AdminHandler) GetAnalytics(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)

	var user db.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	period := c.DefaultQuery("period", "30d")
	from := c.DefaultQuery("from", time.Now().AddDate(0, 0, -30).Format("2006-01-02"))
	to := c.DefaultQuery("to", time.Now().Format("2006-01-02"))

	fromTime, _ := time.Parse("2006-01-02", from)
	toTime, _ := time.Parse("2006-01-02", to)
	toTime = toTime.Add(24 * time.Hour)

	// Статистика по тикетам
	var ticketStats struct {
		Total        int64 `json:"total"`
		Resolved     int64 `json:"resolved"`
		AutoResolved int64 `json:"auto_resolved"`
		Escalated    int64 `json:"escalated"`
	}

	h.db.Model(&db.Ticket{}).
		Where("dispatcher_id = ?", user.DispatcherID).
		Where("created_at BETWEEN ? AND ?", fromTime, toTime).
		Count(&ticketStats.Total)

	h.db.Model(&db.Ticket{}).
		Where("dispatcher_id = ? AND status = ?", user.DispatcherID, "resolved").
		Where("created_at BETWEEN ? AND ?", fromTime, toTime).
		Count(&ticketStats.Resolved)

	// Авто-решенные тикеты (статус resolved ИЛИ waiting_for_feedback)
	h.db.Model(&db.Ticket{}).Where(
		"dispatcher_id = ? AND status IN ('resolved','waiting_for_feedback') AND created_at BETWEEN ? AND ?",
		user.DispatcherID, fromTime, toTime,
	).Count(&ticketStats.AutoResolved)

	// Эскалированные (статус waiting)
	h.db.Model(&db.Ticket{}).Where(
		"dispatcher_id = ? AND status = 'waiting' AND created_at BETWEEN ? AND ?",
		user.DispatcherID, fromTime, toTime,
	).Count(&ticketStats.Escalated)

	// Динамика по дням
	var dailyStats []struct {
		Date     string `json:"date"`
		Count    int64  `json:"count"`
		Resolved int64  `json:"resolved"`
	}

	h.db.Raw(`
        SELECT 
            DATE(created_at) as date,
            COUNT(*) as count,
            SUM(CASE WHEN status = 'resolved' THEN 1 ELSE 0 END) as resolved
        FROM tickets 
        WHERE dispatcher_id = ? 
            AND created_at BETWEEN ? AND ?
        GROUP BY DATE(created_at)
        ORDER BY date DESC
    `, user.DispatcherID, fromTime, toTime).Scan(&dailyStats)

	// Статистика по категориям
	var categoryStats []struct {
		Category string `json:"category"`
		Count    int64  `json:"count"`
	}

	h.db.Model(&db.Ticket{}).
		Select("category, COUNT(*) as count").
		Where("dispatcher_id = ?", user.DispatcherID).
		Where("category IS NOT NULL").
		Group("category").
		Order("count DESC").
		Limit(10).
		Scan(&categoryStats)

	// Статистика по операторам
	var operatorStats []struct {
		UserID   uuid.UUID `json:"user_id"`
		Username string    `json:"username"`
		FullName string    `json:"full_name"`
		Tickets  int64     `json:"tickets"`
		Resolved int64     `json:"resolved"`
		AvgTime  float64   `json:"avg_resolve_time"`
	}

	h.db.Raw(`
        SELECT 
            u.id as user_id,
            u.username,
            u.full_name,
            COUNT(t.id) as tickets,
            SUM(CASE WHEN t.status = 'resolved' THEN 1 ELSE 0 END) as resolved,
            AVG(EXTRACT(EPOCH FROM (t.resolved_at - t.created_at)) / 60) as avg_resolve_time
        FROM users u
        LEFT JOIN tickets t ON t.assigned_to = u.id 
            AND t.created_at BETWEEN ? AND ?
        WHERE u.dispatcher_id = ? AND u.role IN ('admin', 'operator')
        GROUP BY u.id, u.username, u.full_name
        ORDER BY tickets DESC
    `, fromTime, toTime, user.DispatcherID).Scan(&operatorStats)

	c.JSON(http.StatusOK, gin.H{
		"period": gin.H{
			"from":  from,
			"to":    to,
			"label": period,
		},
		"ticket_stats":   ticketStats,
		"daily_stats":    dailyStats,
		"category_stats": categoryStats,
		"operator_stats": operatorStats,
	})
}

// ListAgents — список агентов диспетчерской
func (h *AdminHandler) ListAgents(c *gin.Context) {
	dispatcherID := h.getDispatcherID(c)
	if dispatcherID == uuid.Nil {
		return
	}

	// Получаем API-ключ
	var dispatcher db.Dispatcher
	if err := h.db.First(&dispatcher, "id = ?", dispatcherID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Dispatcher not found"})
		return
	}

	// Запрашиваем агентов у оркестратора
	orchURL := h.config.Orchestrator.URL + "/api/v1/agents"
	httpClient := &http.Client{Timeout: 10 * time.Second}
	req, _ := http.NewRequest("GET", orchURL, nil)
	req.Header.Set("X-API-Key", dispatcher.OrchestratorAPIKey)

	resp, err := httpClient.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Orchestrator unreachable"})
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)

	c.JSON(resp.StatusCode, result)
}

// DeleteAgent — удаление агента
func (h *AdminHandler) DeleteAgent(c *gin.Context) {
	agentID := c.Param("id")

	var dispatcher db.Dispatcher
	h.db.First(&dispatcher, "is_active = true")

	orchURL := fmt.Sprintf("%s/api/v1/agents/%s", h.config.Orchestrator.URL, agentID)
	req, _ := http.NewRequest("DELETE", orchURL, nil)
	req.Header.Set("X-API-Key", dispatcher.OrchestratorAPIKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Orchestrator unreachable"})
		return
	}
	defer resp.Body.Close()
	c.JSON(resp.StatusCode, gin.H{"message": "Agent deleted"})
}

// CreateAgent — регистрация нового агента (отправляет запрос в оркестратор)
func (h *AdminHandler) CreateAgent(c *gin.Context) {
	var req struct {
		Endpoint string `json:"endpoint" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dispatcherID := h.getDispatcherID(c)
	if dispatcherID == uuid.Nil {
		return
	}

	// Отправляем запрос в оркестратор — он сам проверит карточку
	orchURL := h.config.Orchestrator.URL + "/api/v1/agents"

	requestBody, _ := json.Marshal(map[string]string{
		"endpoint":      req.Endpoint,
		"dispatcher_id": dispatcherID.String(),
	})

	httpClient := &http.Client{Timeout: 15 * time.Second}

	// Получаем API-ключ диспетчерской
	var dispatcher db.Dispatcher
	if err := h.db.First(&dispatcher, "id = ?", dispatcherID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Dispatcher not found"})
		return
	}

	orchReq, _ := http.NewRequest("POST", orchURL, bytes.NewReader(requestBody))
	orchReq.Header.Set("Content-Type", "application/json")
	orchReq.Header.Set("X-API-Key", dispatcher.OrchestratorAPIKey)

	orchResp, err := httpClient.Do(orchReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Orchestrator unreachable: " + err.Error()})
		return
	}
	defer orchResp.Body.Close()

	if orchResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(orchResp.Body)
		c.JSON(orchResp.StatusCode, gin.H{"error": "Orchestrator rejected: " + string(body)})
		return
	}

	// Парсим ответ оркестратора
	var orchResult struct {
		Agent   db.Agent `json:"agent"`
		Message string   `json:"message"`
	}
	json.NewDecoder(orchResp.Body).Decode(&orchResult)

	// Сохраняем агента в локальной БД
	agent := orchResult.Agent

	// Нормализуем capabilities и skills под формат клиентской БД
	agent.Capabilities = normalizeJSONB(agent.Capabilities)
	agent.Skills = normalizeJSONB(agent.Skills)
	agent.Metadata = normalizeJSONB(agent.Metadata)

	agent.DispatcherID = &dispatcherID
	agent.ID = uuid.New()

	if err := h.db.Create(&agent).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save agent locally: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"agent":   orchResult.Agent,
		"message": "Agent registered in orchestrator and saved locally",
	})
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func toJSON(v interface{}) db.JSONB {
	data, _ := json.Marshal(v)
	return db.JSONB{"raw": string(data)}
}

// getDispatcherID извлекает ID диспетчерской текущего пользователя
func (h *AdminHandler) getDispatcherID(c *gin.Context) uuid.UUID {
	userID, err := middleware.GetUserIDFromContext(c)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return uuid.Nil
	}
	var user db.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "User not found"})
		return uuid.Nil
	}
	return user.DispatcherID
}

// normalizeJSONB приводит любые данные к формату клиентской БД {"raw": "json-string"}
func normalizeJSONB(v interface{}) db.JSONB {
	if v == nil {
		return db.JSONB{"raw": "[]"}
	}
	// Если уже в формате {"raw": "..."}, возвращаем как есть
	if m, ok := v.(map[string]interface{}); ok {
		if raw, hasRaw := m["raw"]; hasRaw && raw != nil {
			return db.JSONB(m)
		}
	}
	// Если это строка "null" — заменяем на пустой массив
	if s, ok := v.(string); ok && (s == "null" || s == "") {
		return db.JSONB{"raw": "[]"}
	}
	// Иначе оборачиваем в {"raw": json-string}
	data, _ := json.Marshal(v)
	if string(data) == "null" {
		return db.JSONB{"raw": "[]"}
	}
	return db.JSONB{"raw": string(data)}
}

// В admin.go добавить:
func (h *AdminHandler) AddKnowledge(c *gin.Context) {
	dispatcherID := h.getDispatcherID(c)
	if dispatcherID == uuid.Nil {
		return
	}

	// Получаем API-ключ из БД
	var dispatcher db.Dispatcher
	if err := h.db.First(&dispatcher, "id = ?", dispatcherID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Dispatcher not found"})
		return
	}

	var req map[string]interface{}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	body, _ := json.Marshal(req)
	orchURL := h.config.Orchestrator.URL + "/api/v1/knowledge"

	httpClient := &http.Client{Timeout: 30 * time.Second}
	orchReq, _ := http.NewRequest("POST", orchURL, bytes.NewReader(body))
	orchReq.Header.Set("Content-Type", "application/json")
	orchReq.Header.Set("X-API-Key", dispatcher.OrchestratorAPIKey)

	resp, err := httpClient.Do(orchReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Orchestrator unreachable: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	c.JSON(resp.StatusCode, result)
}
