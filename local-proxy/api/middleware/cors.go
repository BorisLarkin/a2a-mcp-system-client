// ./local-proxy/api/middleware/cors.go
package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	cors "github.com/rs/cors/wrapper/gin"
)

func CORS() gin.HandlerFunc {
	return cors.New(cors.Options{
		AllowedOrigins: []string{
			"http://localhost:3000",   // React dev server
			"http://localhost:8080",   // Local proxy
			"https://your-domain.com", // Production domain
		},
		AllowedMethods: []string{
			"GET",
			"POST",
			"PUT",
			"PATCH",
			"DELETE",
			"OPTIONS",
		},
		AllowedHeaders: []string{
			"Origin",
			"Content-Type",
			"Content-Length",
			"Accept-Encoding",
			"Authorization",
			"X-CSRF-Token",
			"X-API-Key",
			"X-Requested-With",
		},
		ExposedHeaders: []string{
			"Content-Length",
			"X-Total-Count",
		},
		AllowCredentials: true,
		MaxAge:           12 * int(time.Hour.Seconds()),
	})
}
