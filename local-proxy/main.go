package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

type Config struct {
	CloudAPIURL   string `json:"cloud_api_url"`
	CloudAPIKey   string `json:"cloud_api_key"`
	Port          string `json:"port"`
	DatabaseURL   string `json:"database_url"`
	SyncInterval  int    `json:"sync_interval"`
	EnableOffline bool   `json:"enable_offline"`
}

type TicketRequest struct {
	Text         string                 `json:"text"`
	DispatcherID string                 `json:"dispatcher_id"`
	Channel      string                 `json:"channel"`
	Metadata     map[string]interface{} `json:"metadata"`
}

type CloudResponse struct {
	TicketID string                 `json:"ticket_id"`
	Response string                 `json:"response"`
	Status   string                 `json:"status"`
	Metadata map[string]interface{} `json:"metadata"`
}

type LocalServer struct {
	config Config
	db     *sqlx.DB
	client *http.Client
}

// Добавляем модели для новой схемы
type Employee struct {
	ID         string `db:"id"`
	Login      string `db:"логин"`
	Role       string `db:"роль"`
	IsActive   bool   `db:"активен"`
	TelegramID *int64 `db:"telegram_chat_id"`
}

type DispatcherConfig struct {
	DispatcherID     string  `db:"диспетчерская_id"`
	Style            string  `db:"стиль_общения"`
	Confidence       float64 `db:"порог_уверенности"`
	EnableInternet   bool    `db:"интернет_поиск"`
	OfflineAutoReply bool    `db:"автоответ_при_оффлайн"`
	OfflineMessage   string  `db:"текст_автоответа"`
	NotifyThreshold  float64 `db:"порог_для_уведомления"`
}

type CachedTicket struct {
	ID           string    `db:"id"`
	DispatcherID string    `db:"диспетчерская_id"`
	CloudID      *string   `db:"облачный_id"`
	SourceText   string    `db:"исходный_текст"`
	AIResponse   *string   `db:"ответ_ai"`
	AIAnalysis   []byte    `db:"анализ_ai"`
	Status       string    `db:"статус"`
	SyncStatus   string    `db:"синхронизация_статус"`
	CreatedAt    time.Time `db:"получено_в"`
	ChannelID    *string   `db:"канал_id"`
	Metadata     []byte    `db:"метаданные"`
}

// Обновляем handleCreateTicket для работы с сотрудниками
func (s *LocalServer) handleCreateTicket(c *gin.Context) {
	var req TicketRequest

	// ... валидация ...

	// 1. Получаем настройки диспетчерской
	var config DispatcherConfig
	err := s.db.Get(&config,
		"SELECT * FROM настройки_диспетчерской WHERE диспетчерская_id = $1",
		req.DispatcherID)

	// 2. Проверяем, нужна ли эскалация к оператору
	needsEscalation := false
	if req.Metadata["confidence"] != nil {
		confidence := req.Metadata["confidence"].(float64)
		if confidence < config.NotifyThreshold {
			needsEscalation = true
		}
	}

	// 3. Если облако доступно и уверенность высокая - отправляем в облако
	if s.checkCloudAvailability() && !needsEscalation {
		cloudResp, err := s.sendToCloud(req)
		if err == nil {
			// Сохраняем ответ AI
			ticketID := s.saveAITicket(req, cloudResp)

			// Проверяем, не нужно ли всё равно уведомить оператора
			if cloudResp.Metadata["confidence"].(float64) < config.NotifyThreshold {
				s.escalateToOperator(ticketID, req.DispatcherID)
			}

			c.JSON(200, cloudResp)
			return
		}
	}

	// 4. Эскалация оператору или офлайн-режим
	if needsEscalation || config.OfflineAutoReply {
		ticketID := s.createLocalTicket(req)

		if needsEscalation {
			// Назначаем оператору
			operatorID := s.assignToOperator(ticketID, req.DispatcherID)

			c.JSON(200, gin.H{
				"ticket_id":         ticketID,
				"status":            "escalated_to_operator",
				"operator_assigned": operatorID,
				"message":           "Обращение передано оператору",
			})
		} else {
			// Офлайн-автоответ
			c.JSON(200, gin.H{
				"ticket_id": ticketID,
				"status":    "offline_pending",
				"response":  config.OfflineMessage,
				"offline":   true,
			})
		}
		return
	}

	c.JSON(503, gin.H{"error": "Service unavailable"})
}

// Функция назначения оператору
func (s *LocalServer) assignToOperator(ticketID, dispatcherID string) string {
	// Находим свободного оператора
	var operatorID string
	err := s.db.Get(&operatorID, `
        SELECT id FROM сотрудники 
        WHERE диспетчерская_id = $1 
          AND роль = 'оператор' 
          AND активен = true
        ORDER BY RANDOM()
        LIMIT 1
    `, dispatcherID)

	if err == nil {
		// Назначаем тикет
		s.db.Exec(`
            UPDATE кэш_тикетов 
            SET оператор_id = $1, 
                статус = 'ожидает_оператора'
            WHERE id = $2
        `, operatorID, ticketID)

		// Отправляем уведомление оператору
		go s.notifyOperator(operatorID, ticketID)
	}

	return operatorID
}

// Уведомление оператора
func (s *LocalServer) notifyOperator(operatorID, ticketID string) {
	var operator Employee
	err := s.db.Get(&operator,
		"SELECT * FROM сотрудники WHERE id = $1", operatorID)

	if err != nil || operator.TelegramID == nil {
		return
	}

	var ticket CachedTicket
	s.db.Get(&ticket,
		"SELECT исходный_текст, получено_в FROM кэш_тикетов WHERE id = $1",
		ticketID)

	message := fmt.Sprintf(
		"📩 Новое обращение #%s\n\n%s\n\nВремя: %s",
		ticketID[:8],
		truncate(ticket.SourceText, 200),
		ticket.CreatedAt.Format("15:04"),
	)

	// Отправляем в Telegram
	if operator.TelegramID != nil {
		s.sendTelegramNotification(*operator.TelegramID, message)
	}
}

func main() {
	// Загрузка конфигурации
	config := loadConfig()

	// Подключение к БД
	db, err := sqlx.Connect("postgres", config.DatabaseURL)
	if err != nil {
		log.Fatal("Database connection failed:", err)
	}
	defer db.Close()

	// Создание сервера
	server := &LocalServer{
		config: config,
		db:     db,
		client: &http.Client{Timeout: 30 * time.Second},
	}

	// Запуск фоновых задач
	go server.startSyncWorker()
	go server.startHealthChecker()

	// Настройка роутера
	r := gin.Default()

	// Маршруты API
	api := r.Group("/api/v1")
	{
		api.POST("/tickets", server.handleCreateTicket)
		api.GET("/tickets/:id", server.handleGetTicket)
		api.GET("/health", server.handleHealthCheck)
		api.GET("/sync", server.handleManualSync)
		api.POST("/webhook/cloud", server.handleCloudWebhook)
	}

	// Статические файлы для веб-интерфейса
	r.Static("/dashboard", "./web-dashboard/frontend/dist")

	log.Printf("Starting local proxy server on port %s", config.Port)
	if err := r.Run(":" + config.Port); err != nil {
		log.Fatal("Server failed:", err)
	}
}

func (s *LocalServer) handleCreateTicket(c *gin.Context) {
	var req TicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request"})
		return
	}

	// 1. Проверяем доступность облака
	if s.checkCloudAvailability() {
		// 2. Отправляем в облако
		cloudResp, err := s.sendToCloud(req)
		if err == nil {
			// 3. Сохраняем в кэш
			s.saveToCache(req, cloudResp)
			c.JSON(200, cloudResp)
			return
		}
	}

	// 4. Если облако недоступно - офлайн-режим
	if s.config.EnableOffline {
		offlineResp := s.handleOffline(req)
		c.JSON(200, offlineResp)
		return
	}

	c.JSON(503, gin.H{"error": "Service unavailable"})
}

func (s *LocalServer) sendToCloud(req TicketRequest) (*CloudResponse, error) {
	payload, _ := json.Marshal(req)

	httpReq, _ := http.NewRequest("POST",
		s.config.CloudAPIURL+"/api/v1/tickets",
		bytes.NewBuffer(payload))

	httpReq.Header.Set("X-API-Key", s.config.CloudAPIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var cloudResp CloudResponse
	if err := json.NewDecoder(resp.Body).Decode(&cloudResp); err != nil {
		return nil, err
	}

	return &cloudResp, nil
}

func (s *LocalServer) handleOffline(req TicketRequest) *CloudResponse {
	// Генерируем локальный ID
	localID := "offline-" + time.Now().Format("20060102-150405")

	// Сохраняем в БД как ожидающий синхронизации
	_, err := s.db.Exec(`
		INSERT INTO ticket_cache 
		(dispatcher_id, cloud_ticket_id, source_text, channel, metadata, sync_status)
		VALUES ($1, $2, $3, $4, $5, 'pending')
	`, req.DispatcherID, localID, req.Text, req.Channel, req.Metadata)

	if err != nil {
		log.Println("Failed to save offline ticket:", err)
	}

	// Получаем офлайн-шаблон
	var template string
	s.db.Get(&template, `
		SELECT offline_response_template 
		FROM local_settings 
		WHERE dispatcher_id = $1
	`, req.DispatcherID)

	if template == "" {
		template = "Мы получили ваше сообщение. Ответим при восстановлении связи."
	}

	return &CloudResponse{
		TicketID: localID,
		Response: template,
		Status:   "offline_pending",
		Metadata: map[string]interface{}{
			"offline":  true,
			"local_id": localID,
		},
	}
}

func (s *LocalServer) startSyncWorker() {
	ticker := time.NewTicker(time.Duration(s.config.SyncInterval) * time.Minute)

	for range ticker.C {
		s.syncPendingTickets()
	}
}

func (s *LocalServer) syncPendingTickets() {
	// Получаем тикеты, ожидающие синхронизации
	var pendingTickets []struct {
		ID           string
		DispatcherID string
		SourceText   string
		Channel      string
		Metadata     string
	}

	s.db.Select(&pendingTickets, `
		SELECT id, dispatcher_id, source_text, channel, metadata
		FROM ticket_cache 
		WHERE sync_status = 'pending' 
		LIMIT 10
	`)

	for _, ticket := range pendingTickets {
		req := TicketRequest{
			Text:         ticket.SourceText,
			DispatcherID: ticket.DispatcherID,
			Channel:      ticket.Channel,
			Metadata:     json.RawMessage(ticket.Metadata),
		}

		// Пытаемся отправить в облако
		if resp, err := s.sendToCloud(req); err == nil {
			// Обновляем статус
			s.db.Exec(`
				UPDATE ticket_cache 
				SET cloud_ticket_id = $1, 
				    response_text = $2,
				    ai_analysis = $3,
				    sync_status = 'synced',
				    responded_at = NOW()
				WHERE id = $4
			`, resp.TicketID, resp.Response, resp.Metadata, ticket.ID)
		}
	}
}
