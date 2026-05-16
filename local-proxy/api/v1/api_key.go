package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"local-proxy/internal/config"
	"local-proxy/internal/db"
)

type ApiKeyHandler struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewApiKeyHandler(database *gorm.DB, cfg *config.Config) *ApiKeyHandler {
	return &ApiKeyHandler{db: database, cfg: cfg}
}

func (h *ApiKeyHandler) GetBotKey(c *gin.Context) {
	// Проверяем внутренний секретный ключ
	secret := c.GetHeader("X-Setup-Secret")
	if secret != "internal_setup_secret_123" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	var dispatcher db.Dispatcher
	if err := h.db.Where("is_active = ?", true).First(&dispatcher).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No active dispatcher"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"api_key":       dispatcher.OrchestratorAPIKey,
		"dispatcher_id": dispatcher.ID.String(),
	})
}
