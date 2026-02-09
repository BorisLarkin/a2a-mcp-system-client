// ./local-proxy/api/middleware/telegram_auth.go
package middleware

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// TelegramWebhookAuth middleware для проверки webhook от Telegram
func TelegramWebhookAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Проверка заголовка X-Telegram-Bot-Api-Secret-Token
		secretToken := c.GetHeader("X-Telegram-Bot-Api-Secret-Token")
		expectedToken := "your-telegram-webhook-secret" // Должен совпадать с настройками бота

		if secretToken != expectedToken {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid Telegram webhook token",
			})
			c.Abort()
			return
		}

		// Проверка HMAC подписи (опционально)
		if signature := c.GetHeader("X-Hub-Signature-256"); signature != "" {
			body, err := c.GetRawData()
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to read body"})
				c.Abort()
				return
			}

			// Восстанавливаем body для последующих обработчиков
			c.Request.Body = io.NopCloser(bytes.NewBuffer(body))

			// Проверяем HMAC
			if !verifyHMAC(body, signature, expectedToken) {
				c.JSON(http.StatusUnauthorized, gin.H{
					"error": "Invalid HMAC signature",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

func verifyHMAC(data []byte, signature, secret string) bool {
	parts := strings.Split(signature, "=")
	if len(parts) != 2 || parts[0] != "sha256" {
		return false
	}

	expectedMAC, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(data)
	actualMAC := mac.Sum(nil)

	return hmac.Equal(actualMAC, expectedMAC)
}
