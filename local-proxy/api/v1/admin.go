// ./local-proxy/api/v1/admin.go
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
	userID, _ := GetUserIDFromContext(c)

	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var user db.User
	if err := h.db.Where("id = ?", userID).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Обновляем диспетчерскую
	var dispatcher db.Dispatcher
	if err := h.db.Where("id = ?", user.DispatcherID).First(&dispatcher).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Dispatcher not found"})
		return
	}

	if req.DispatcherName != "" {
		dispatcher.Name = req.DispatcherName
	}

	if req.OrchestratorAPIKey != "" {
		dispatcher.OrchestratorAPIKey = req.OrchestratorAPIKey
	}

	if req.Settings != nil {
		dispatcher.Settings = db.JSONB(req.Settings)
	}

	dispatcher.UpdatedAt = time.Now()

	if err := h.db.Save(&dispatcher).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update dispatcher"})
		return
	}

	// Обновляем AI настройки
	var aiSettings db.AISettings
	if err := h.db.Where("dispatcher_id = ?", user.DispatcherID).First(&aiSettings).Error; err != nil {
		// Создаем если нет
		aiSettings = db.AISettings{
			DispatcherID: user.DispatcherID,
			CreatedAt:    time.Now(),
		}
	}

	if req.AISettings.Enabled != nil {
		aiSettings.Enabled = *req.AISettings.Enabled
	}

	if req.AISettings.AutoRespond != nil {
		aiSettings.AutoRespond = *req.AISettings.AutoRespond
	}

	if req.AISettings.ConfidenceThreshold != nil {
		aiSettings.ConfidenceThreshold = *req.AISettings.ConfidenceThreshold
	}

	if req.AISettings.UseInternetSearch != nil {
		aiSettings.UseInternetSearch = *req.AISettings.UseInternetSearch
	}

	if req.AISettings.CommunicationStyle != nil {
		aiSettings.CommunicationStyle = *req.AISettings.CommunicationStyle
	}

	if req.AISettings.SystemContext != nil {
		aiSettings.SystemContext = *req.AISettings.SystemContext
	}

	aiSettings.UpdatedAt = time.Now()

	if err := h.db.Save(&aiSettings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update AI settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":     "Settings updated successfully",
		"dispatcher":  dispatcher,
		"ai_settings": aiSettings,
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

	h.db.Model(&db.Ticket{}).
		Where("dispatcher_id = ? AND metadata->>'ai_resolved' = ?", user.DispatcherID, "true").
		Where("created_at BETWEEN ? AND ?", fromTime, toTime).
		Count(&ticketStats.AutoResolved)

	h.db.Model(&db.Ticket{}).
		Where("dispatcher_id = ? AND metadata->>'escalated' = ?", user.DispatcherID, "true").
		Where("created_at BETWEEN ? AND ?", fromTime, toTime).
		Count(&ticketStats.Escalated)

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
