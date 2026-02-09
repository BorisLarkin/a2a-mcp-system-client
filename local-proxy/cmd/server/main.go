// ./local-proxy/cmd/server/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"local-proxy/api/middleware"
	"local-proxy/internal/auth"
	"local-proxy/internal/config"
	"local-proxy/internal/db"
	"local-proxy/internal/queue"
	"local-proxy/internal/websocket"
)

func main() {
	// Загрузка конфигурации
	cfg, err := config.LoadConfig("configs/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Инициализация базы данных
	dsn := db.BuildDSN(cfg.Database)
	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	// Автомиграции (в продакшене использовать отдельные миграции)
	if err := db.RunMigrations(gormDB); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// Инициализация Redis
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Host + ":" + cfg.Redis.Port,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// Проверка подключения Redis
	ctx := context.Background()
	if _, err := redisClient.Ping(ctx).Result(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// Инициализация менеджера WebSocket
	wsManager := websocket.NewManager()

	// Инициализация менеджера аутентификации
	authManager := auth.NewManager(cfg.Auth.JWTSecret, cfg.Auth.AccessTokenExpiry)

	// Инициализация очереди тикетов
	ticketQueue := queue.NewTicketQueue(gormDB, redisClient)

	// Инициализация роутера Gin
	router := gin.Default()

	// Глобальные middleware
	router.Use(middleware.CORS())
	router.Use(middleware.Recovery())
	router.Use(middleware.Logger())

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
			"time":   time.Now().Unix(),
		})
	})

	// API v1 группа
	apiV1 := router.Group("/api/v1")
	{
		// Публичные эндпоинты (без аутентификации)
		authGroup := apiV1.Group("/auth")
		{
			authHandler := v1.NewAuthHandler(authManager, gormDB)
			authGroup.POST("/login", authHandler.Login)
			authGroup.POST("/refresh", authHandler.RefreshToken)
		}

		// Защищенные эндпоинты (требуется JWT)
		protected := apiV1.Group("")
		protected.Use(middleware.JWTAuth(authManager))
		{
			// Тикеты
			ticketHandler := v1.NewTicketHandler(gormDB, ticketQueue, wsManager, cfg)
			protected.GET("/tickets", ticketHandler.ListTickets)
			protected.GET("/tickets/:id", ticketHandler.GetTicket)
			protected.POST("/tickets", ticketHandler.CreateTicket)
			protected.PUT("/tickets/:id", ticketHandler.UpdateTicket)
			protected.POST("/tickets/:id/messages", ticketHandler.AddMessage)

			// Очередь
			queueHandler := v1.NewQueueHandler(ticketQueue, gormDB)
			protected.GET("/queue", queueHandler.GetQueue)
			protected.POST("/queue/:id/assign", queueHandler.AssignTicket)
			protected.POST("/queue/:id/unassign", queueHandler.UnassignTicket)

			// WebSocket для реальных обновлений
			protected.GET("/ws", func(c *gin.Context) {
				userID := c.GetString("user_id")
				wsManager.ServeWS(c.Writer, c.Request, userID)
			})

			// Админские эндпоинты
			adminHandler := v1.NewAdminHandler(gormDB, cfg)
			adminGroup := protected.Group("/admin")
			adminGroup.Use(middleware.RequireRole("admin"))
			{
				adminGroup.GET("/settings", adminHandler.GetSettings)
				adminGroup.PUT("/settings", adminHandler.UpdateSettings)
				adminGroup.GET("/users", adminHandler.ListUsers)
				adminGroup.POST("/users", adminHandler.CreateUser)
				adminGroup.PUT("/users/:id", adminHandler.UpdateUser)
				adminGroup.GET("/channels", adminHandler.ListChannels)
				adminGroup.POST("/channels", adminHandler.CreateChannel)
				adminGroup.PUT("/channels/:id", adminHandler.UpdateChannel)
				adminGroup.GET("/analytics", adminHandler.GetAnalytics)
			}

			// Внешний вызов оркестратора
			orchestratorHandler := v1.NewOrchestratorHandler(gormDB, cfg, redisClient)
			protected.POST("/orchestrator/classify", orchestratorHandler.Classify)
			protected.POST("/orchestrator/generate", orchestratorHandler.GenerateResponse)
		}

		// Webhook для Telegram (без JWT, но с секретом)
		webhookGroup := apiV1.Group("/webhook")
		webhookGroup.Use(middleware.TelegramWebhookAuth())
		{
			telegramHandler := v1.NewTelegramHandler(gormDB, ticketQueue, cfg, wsManager)
			webhookGroup.POST("/telegram", telegramHandler.HandleWebhook)
		}
	}

	// Статические файлы для веб-интерфейса
	router.Static("/static", "./web-dashboard/build/static")
	router.StaticFile("/", "./web-dashboard/build/index.html")
	router.StaticFile("/favicon.ico", "./web-dashboard/build/favicon.ico")
	router.NoRoute(func(c *gin.Context) {
		c.File("./web-dashboard/build/index.html")
	})

	// Запуск сервера с graceful shutdown
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	go func() {
		log.Printf("Server starting on port %s", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	// Закрытие соединений
	wsManager.Shutdown()
	redisClient.Close()

	log.Println("Server exited properly")
}
