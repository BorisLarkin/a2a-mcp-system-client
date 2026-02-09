// ./local-proxy/api/middleware/recovery.go
package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Логирование паники
				log.Error().
					Interface("error", err).
					Str("stack", string(debug.Stack())).
					Str("path", c.Request.URL.Path).
					Str("method", c.Request.Method).
					Msg("PANIC recovered")

				// Отправка ответа клиенту
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "Internal server error",
					"message": "Something went wrong. Please try again later.",
				})

				c.Abort()
			}
		}()

		c.Next()
	}
}
