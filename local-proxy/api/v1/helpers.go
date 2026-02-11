// ./local-proxy/api/v1/helpers.go
package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// GetUserIDFromContext получает ID пользователя из контекста
func GetUserIDFromContext(c *gin.Context) (uuid.UUID, error) {
	userID, exists := c.Get("user_id")
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
	username, exists := c.Get("username")
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
	role, exists := c.Get("role")
	if !exists {
		return "", gin.Error{
			Err:  http.ErrNoCookie,
			Type: gin.ErrorTypePrivate,
		}
	}

	return role.(string), nil
}
