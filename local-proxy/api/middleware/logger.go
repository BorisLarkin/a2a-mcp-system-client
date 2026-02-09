// ./local-proxy/api/middleware/logger.go
package middleware

import (
	"bytes"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type responseBodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w responseBodyWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		// Чтение тела запроса для логирования
		var requestBody []byte
		if c.Request.Body != nil {
			requestBody, _ = io.ReadAll(c.Request.Body)
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// Создание writer для захвата ответа
		w := &responseBodyWriter{
			body:           bytes.NewBufferString(""),
			ResponseWriter: c.Writer,
		}
		c.Writer = w

		c.Next()

		duration := time.Since(start)

		// Логирование
		event := log.Info()

		// Основные поля
		event.
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Str("query", c.Request.URL.RawQuery).
			Str("ip", c.ClientIP()).
			Str("user_agent", c.Request.UserAgent()).
			Int("status", c.Writer.Status()).
			Dur("duration", duration).
			Str("duration_human", duration.String())

		// Пользовательские поля
		if userID, exists := c.Get("user_id"); exists {
			event.Str("user_id", userID.(uuid.UUID).String())
		}

		if username, exists := c.Get("username"); exists {
			event.Str("username", username.(string))
		}

		// Логирование тела запроса (только для не-GET методов и небольших тел)
		if len(requestBody) > 0 && len(requestBody) < 1024 && c.Request.Method != "GET" {
			event.RawJSON("request_body", requestBody)
		}

		// Логирование ошибок
		if len(c.Errors) > 0 {
			event.Strs("errors", c.Errors.Errors())
		}

		// Логирование тела ответа для ошибок
		if c.Writer.Status() >= 400 && w.body.Len() > 0 && w.body.Len() < 1024 {
			event.Str("response_body", w.body.String())
		}

		// Определяем уровень логирования
		if c.Writer.Status() >= 500 {
			event = log.Error()
		} else if c.Writer.Status() >= 400 {
			event = log.Warn()
		}

		event.Msg("HTTP request")
	}
}
