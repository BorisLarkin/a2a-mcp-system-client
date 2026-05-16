package middleware

import (
	dbmodels "local-proxy/internal/db"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func APIKeyAuth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "X-API-Key header required"})
			c.Abort()
			return
		}

		// Ищем диспетчерскую с таким API-ключом (внутренний ключ)
		// Для публичных эндпоинтов используем поле api_key из dispatchers или отдельную таблицу
		// Пока проверяем, что ключ совпадает с любым активным диспетчером
		var count int64
		db.Model(&dbmodels.Dispatcher{}).Where("orchestrator_api_key = ? AND is_active = true", apiKey).Count(&count)
		if count == 0 {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			c.Abort()
			return
		}
		c.Next()
	}
}
