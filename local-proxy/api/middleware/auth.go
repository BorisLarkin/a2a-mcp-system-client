// ./local-proxy/api/middleware/auth.go
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"local-proxy/internal/auth"
)

const (
	UserIDKey   = "user_id"
	UsernameKey = "username"
	RoleKey     = "role"
)

// JWTAuth middleware для проверки JWT токена
func JWTAuth(authManager *auth.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		// Формат: Bearer <token>
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format"})
			c.Abort()
			return
		}

		token := parts[1]
		claims, err := authManager.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token", "details": err.Error()})
			c.Abort()
			return
		}

		// Добавляем данные пользователя в контекст
		c.Set(UserIDKey, claims.UserID)
		c.Set(UsernameKey, claims.Username)
		c.Set(RoleKey, claims.Role)

		// Обновляем контекст Gin контекстом Go
		ctx := context.WithValue(c.Request.Context(), UserIDKey, claims.UserID)
		c.Request = c.Request.WithContext(ctx)

		c.Next()
	}
}

// RequireRole middleware для проверки роли
func RequireRole(requiredRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get(RoleKey)
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "User role not found"})
			c.Abort()
			return
		}

		role := userRole.(string)

		// Проверяем иерархию ролей
		roleHierarchy := map[string]int{
			"admin":    3,
			"operator": 2,
			"viewer":   1,
		}

		userLevel, userOk := roleHierarchy[role]
		requiredLevel, requiredOk := roleHierarchy[requiredRole]

		if !userOk || !requiredOk || userLevel < requiredLevel {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			c.Abort()
			return
		}

		c.Next()
	}
}

// GetUserIDFromContext получает ID пользователя из контекста
func GetUserIDFromContext(c *gin.Context) (uuid.UUID, error) {
	userID, exists := c.Get(UserIDKey)
	if !exists {
		return uuid.Nil, gin.Error{
			Err:  http.ErrNoCookie,
			Type: gin.ErrorTypePrivate,
		}
	}

	return userID.(uuid.UUID), nil
}

// GetUsernameFromContext получает имя пользователя из контекста
func GetUsernameFromContext(c *gin.Context) (string, error) {
	username, exists := c.Get(UsernameKey)
	if !exists {
		return "", gin.Error{
			Err:  http.ErrNoCookie,
			Type: gin.ErrorTypePrivate,
		}
	}

	return username.(string), nil
}

// GetRoleFromContext получает роль пользователя из контекста
func GetRoleFromContext(c *gin.Context) (string, error) {
	role, exists := c.Get(RoleKey)
	if !exists {
		return "", gin.Error{
			Err:  http.ErrNoCookie,
			Type: gin.ErrorTypePrivate,
		}
	}

	return role.(string), nil
}
