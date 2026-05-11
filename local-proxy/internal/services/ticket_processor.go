package services

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"local-proxy/internal/db"
	"local-proxy/internal/messaging"
	"local-proxy/internal/orchestrator"
	"local-proxy/internal/websocket"
)

type TicketProcessor struct {
	db           *gorm.DB
	orchestrator *orchestrator.Client
	botClient    *messaging.BotClient
	wsManager    *websocket.Manager
	dispatcherID string
}

func NewTicketProcessor(database *gorm.DB, orchClient *orchestrator.Client, botClient *messaging.BotClient, wsMgr *websocket.Manager, dispID string) *TicketProcessor {
	return &TicketProcessor{
		db:           database,
		orchestrator: orchClient,
		botClient:    botClient,
		wsManager:    wsMgr,
		dispatcherID: dispID,
	}
}

// ProcessTicket отправляет обращение в оркестратор и обрабатывает ответ
func (tp *TicketProcessor) ProcessTicket(ticketID uuid.UUID) {
	var ticket db.Ticket
	if err := tp.db.Preload("Client").First(&ticket, "id = ?", ticketID).Error; err != nil {
		log.Printf("Ticket %s not found: %v", ticketID, err)
		return
	}

	// Логируем начало обращения к оркестратору
	logEntry := db.OrchestratorLog{
		DispatcherID: ticket.DispatcherID,
		TicketID:     &ticketID,
		RequestType:  "process-ticket",
		RequestData:  db.JSONB{"text": ticket.OriginalText, "dispatcher_id": tp.dispatcherID},
	}
	start := time.Now()

	// Вызов оркестратора
	resp, err := tp.orchestrator.ProcessTicket(ticket.OriginalText, tp.dispatcherID)
	durationMs := int(time.Since(start).Milliseconds())

	if err != nil {
		logEntry.StatusCode = 503
		logEntry.ErrorMessage = err.Error()
		logEntry.DurationMs = durationMs
		tp.db.Create(&logEntry)
		log.Printf("Orchestrator call failed: %v", err)
		return
	}

	logEntry.StatusCode = 200
	respJSON, _ := json.Marshal(resp)
	logEntry.ResponseData = db.JSONB{"raw": string(respJSON)}
	logEntry.DurationMs = durationMs
	tp.db.Create(&logEntry)

	// Сохраняем результат в тикет
	ticket.AIAnalysis = db.JSONB{
		"classification": resp.Classification,
		"plan":           resp.Plan,
		"execution_log":  resp.ExecutionLog,
		"suggested_team": resp.SuggestedTeam,
	}
	ticket.OrchestratorTicketID = &resp.TicketID
	now := time.Now()
	ticket.AIProcessedAt = &now

	// Извлекаем финальный ответ и confidence
	finalResponse := extractFinalResponse(resp.Classification)
	confidence := extractConfidence(resp.Classification)
	ticket.AIResponse = finalResponse
	ticket.Category = extractCategory(resp.Classification)
	tp.db.Save(&ticket)

	threshold := 0.7
	if confidence < threshold {
		// Эскалация
		ticket.Status = "waiting"
		now := time.Now()
		ticket.EscalatedAt = &now
		tp.db.Save(&ticket)

		// Помещаем в очередь
		tp.addToQueue(ticket)

		// Уведомляем операторов
		tp.wsManager.Broadcast("new_escalated", map[string]interface{}{
			"ticket_id": ticket.ID.String(),
			"text":      truncate(ticket.OriginalText, 100),
			"category":  ticket.Category,
		})
	} else {
		// Автоответ
		ticket.Status = "waiting_for_feedback"
		tp.db.Save(&ticket)

		// Отправляем ответ клиенту через бота
		if ticket.ClientID != nil {
			var client db.Client
			if err := tp.db.First(&client, "id = ?", *ticket.ClientID).Error; err == nil {
				chatID := parseChatID(client.ExternalID)
				if chatID != 0 {
					// Сохраняем AI-сообщение
					aiMsg := db.TicketMessage{
						TicketID:    ticket.ID,
						SenderType:  "ai",
						MessageText: finalResponse,
					}
					tp.db.Create(&aiMsg)

					// Отправляем через бот-клиент (будет добавлен позже)
					if tp.botClient != nil {
						err := tp.botClient.SendMessage(chatID, finalResponse, ticket.ID.String())
						if err != nil {
							log.Printf("Failed to send AI response to client: %v", err)
						}
					}
				}
			}
		}
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func extractCategory(classification map[string]interface{}) string {
	if classification == nil {
		return ""
	}
	if cat, ok := classification["category"].(string); ok {
		return cat
	}
	if cat, ok := classification["predicted_class"].(string); ok {
		return cat
	}
	return "общий_вопрос"
}

func (tp *TicketProcessor) addToQueue(ticket db.Ticket) {
	pq := db.TicketQueue{
		TicketID:      ticket.ID,
		DispatcherID:  ticket.DispatcherID,
		PriorityScore: calculatePriority(ticket),
		QueuedAt:      time.Now(),
	}
	tp.db.Create(&pq)
}

func extractFinalResponse(classification map[string]interface{}) string {
	if classification == nil {
		return ""
	}
	if resp, ok := classification["generated_response"].(string); ok && resp != "" {
		return resp
	}
	return ""
}

func extractConfidence(classification map[string]interface{}) float64 {
	if classification == nil {
		return 0
	}
	if conf, ok := classification["confidence"].(float64); ok {
		return conf
	}
	return 0
}

func calculatePriority(ticket db.Ticket) int {
	// Простейший приоритет: учёт категории
	if ticket.AIAnalysis != nil {
		if cat, _ := ticket.AIAnalysis["category"].(string); cat == "техническая" {
			return 5
		}
	}
	return 3
}

func parseChatID(externalID string) int64 {
	var id int64
	fmt.Sscanf(externalID, "%d", &id)
	return id
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
