// ./local-proxy/api/v1/auth.go
package v1

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"local-proxy/internal/auth"
	"local-proxy/internal/db"
	"local-proxy/pkg/utils"
)

type AuthHandler struct {
	authManager *auth.Manager
	db          *gorm.DB
}

func NewAuthHandler(authManager *auth.Manager, db *gorm.DB) *AuthHandler {
	return &AuthHandler{
		authManager: authManager,
		db:          db,
	}
}

// LoginRequest - запрос на вход
type LoginRequest struct {
	Username string `json:"username" binding:"required,min=3,max=100"`
	Password string `json:"password" binding:"required,min=6"`
}

// LoginResponse - ответ с токенами
type LoginResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	UserID       uuid.UUID `json:"user_id"`
	Username     string    `json:"username"`
	Role         string    `json:"role"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// Login - вход пользователя
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Находим пользователя
	var user db.User
	if err := h.db.Where("username = ? AND is_active = true", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Проверяем пароль
	if !utils.VerifyPassword(user.PasswordHash, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Генерируем токены
	accessToken, err := h.authManager.GenerateAccessToken(user.ID, user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	refreshToken, err := h.authManager.GenerateRefreshToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate refresh token"})
		return
	}

	// Обновляем время последнего входа
	now := time.Now()
	user.LastLoginAt = &now
	h.db.Save(&user)

	// Возвращаем ответ
	expiresAt := time.Now().Add(24 * time.Hour) // 24 часа для access token

	c.JSON(http.StatusOK, LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		UserID:       user.ID,
		Username:     user.Username,
		Role:         user.Role,
		ExpiresAt:    expiresAt,
	})
}

// RefreshTokenRequest - запрос на обновление токена
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

// RefreshToken - обновление access токена
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Парсим refresh токен
	claims, err := h.authManager.ParseToken(req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
		return
	}

	// Находим пользователя
	var user db.User
	if err := h.db.Where("id = ? AND is_active = true", claims.UserID).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
		return
	}

	// Генерируем новый access токен
	accessToken, err := h.authManager.GenerateAccessToken(user.ID, user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Генерируем новый refresh токен
	refreshToken, err := h.authManager.GenerateRefreshToken(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate refresh token"})
		return
	}

	expiresAt := time.Now().Add(24 * time.Hour)

	c.JSON(http.StatusOK, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"expires_at":    expiresAt,
	})
}

// Logout - выход пользователя (на клиенте просто удаляем токен)
func (h *AuthHandler) Logout(c *gin.Context) {
	// В stateless JWT логаут реализуется на клиенте
	// Можно добавить blacklist токенов в Redis при необходимости

	c.JSON(http.StatusOK, gin.H{
		"message": "Successfully logged out",
	})
}

// Profile - получение профиля текущего пользователя
func (h *AuthHandler) Profile(c *gin.Context) {
	userID, err := GetUserIDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var user db.User
	if err := h.db.Select("id", "username", "email", "full_name", "role", "created_at").
		Where("id = ?", userID).
		First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user": user,
	})
}
