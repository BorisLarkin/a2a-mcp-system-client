package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"local-proxy/internal/auth"
	"local-proxy/internal/db"
)

type AuthHandler struct {
	authManager *auth.Manager
	db          *gorm.DB
}

func NewAuthHandler(authManager *auth.Manager, db *gorm.DB) *AuthHandler {
	return &AuthHandler{authManager: authManager, db: db}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	User         UserInfo `json:"user"`
}

type UserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Ищем пользователя
	var user db.User
	if err := h.db.Where("username = ? AND is_active = ?", req.Username, true).First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Проверяем пароль (пока простое сравнение, т.к. хеши не используются)
	if user.PasswordHash != req.Password {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: UserInfo{
			ID:       user.ID.String(),
			Username: user.Username,
			Role:     user.Role,
		},
	})
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "refresh_token required"})
		return
	}

	claims, err := h.authManager.ParseToken(req.RefreshToken)
	if err != nil {
		c.JSON(401, gin.H{"error": "Invalid refresh token"})
		return
	}

	userID, _ := uuid.Parse(claims.Subject)
	var user db.User
	if err := h.db.First(&user, "id = ?", userID).Error; err != nil {
		c.JSON(401, gin.H{"error": "User not found"})
		return
	}

	accessToken, _ := h.authManager.GenerateAccessToken(user.ID, user.Username, user.Role)
	refreshToken, _ := h.authManager.GenerateRefreshToken(user.ID)

	c.JSON(200, gin.H{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user": gin.H{
			"id":       user.ID.String(),
			"username": user.Username,
			"role":     user.Role,
		},
	})
}

func (h *AuthHandler) Me(c *gin.Context) {
	userID, _ := GetUserIDFromContext(c)
	username, _ := GetUsernameFromContext(c)
	role, _ := GetRoleFromContext(c)

	c.JSON(http.StatusOK, gin.H{
		"user": UserInfo{
			ID:       userID.String(),
			Username: username,
			Role:     role,
		},
	})
}
