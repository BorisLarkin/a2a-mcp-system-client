// ./local-proxy/internal/queue/ticket_queue.go
package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"local-proxy/internal/db"
)

type PriorityScore struct {
	BaseScore     int // Базовый приоритет (1-100)
	UrgentScore   int // Срочность (0-50)
	WaitingTime   int // Время ожидания (1 за каждую минуту)
	ClientValue   int // Ценность клиента (0-30)
	CategoryScore int // Важность категории (0-20)
}

type TicketQueue struct {
	db    *gorm.DB
	redis *redis.Client
}

func NewTicketQueue(db *gorm.DB, redis *redis.Client) *TicketQueue {
	return &TicketQueue{
		db:    db,
		redis: redis,
	}
}

// AddTicket добавляет тикет в очередь
func (q *TicketQueue) AddTicket(ctx context.Context, ticketID uuid.UUID, dispatcherID uuid.UUID) error {
	// Вычисляем приоритетный балл
	score, err := q.calculatePriorityScore(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("failed to calculate priority score: %w", err)
	}

	// Добавляем в Redis сортированный набор
	key := fmt.Sprintf("queue:dispatcher:%s", dispatcherID.String())
	member := ticketID.String()

	if err := q.redis.ZAdd(ctx, key, redis.Z{
		Score:  float64(score),
		Member: member,
	}).Err(); err != nil {
		return fmt.Errorf("failed to add to redis queue: %w", err)
	}

	// Сохраняем в PostgreSQL для персистентности
	queueEntry := db.TicketQueue{
		TicketID:      ticketID,
		DispatcherID:  dispatcherID,
		PriorityScore: score,
		QueuedAt:      time.Now(),
	}

	if err := q.db.Create(&queueEntry).Error; err != nil {
		// Если не удалось сохранить в БД, удаляем из Redis
		q.redis.ZRem(ctx, key, member)
		return fmt.Errorf("failed to save queue entry: %w", err)
	}

	// Публикуем событие обновления очереди
	q.publishQueueUpdate(ctx, dispatcherID)

	return nil
}

// GetNextTicket возвращает следующий тикет из очереди
func (q *TicketQueue) GetNextTicket(ctx context.Context, dispatcherID uuid.UUID) (*uuid.UUID, error) {
	key := fmt.Sprintf("queue:dispatcher:%s", dispatcherID.String())

	// Используем ZPOPMAX для атомарного получения элемента с наивысшим баллом
	result, err := q.redis.ZPopMax(ctx, key, 1).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to pop from queue: %w", err)
	}

	if len(result) == 0 {
		return nil, nil // Очередь пуста
	}

	ticketID, err := uuid.Parse(result[0].Member.(string))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ticket id: %w", err)
	}

	// Обновляем запись в БД
	if err := q.db.Model(&db.TicketQueue{}).
		Where("ticket_id = ? AND dispatcher_id = ?", ticketID, dispatcherID).
		Update("assigned_at", time.Now()).Error; err != nil {
		// Если не удалось обновить БД, возвращаем тикет обратно в очередь
		q.redis.ZAdd(ctx, key, redis.Z{
			Score:  result[0].Score,
			Member: result[0].Member,
		})
		return nil, fmt.Errorf("failed to update queue entry: %w", err)
	}

	return &ticketID, nil
}

// AssignTicket назначает тикет конкретному оператору
func (q *TicketQueue) AssignTicket(ctx context.Context, ticketID, userID uuid.UUID) error {
	// Удаляем из общей очереди
	var dispatcherID uuid.UUID
	var queueEntry db.TicketQueue

	if err := q.db.Where("ticket_id = ?", ticketID).First(&queueEntry).Error; err != nil {
		return fmt.Errorf("ticket not found in queue: %w", err)
	}

	dispatcherID = queueEntry.DispatcherID
	key := fmt.Sprintf("queue:dispatcher:%s", dispatcherID.String())

	// Удаляем из Redis
	if err := q.redis.ZRem(ctx, key, ticketID.String()).Err(); err != nil {
		return fmt.Errorf("failed to remove from redis: %w", err)
	}

	// Обновляем тикет
	if err := q.db.Model(&db.Ticket{}).
		Where("id = ?", ticketID).
		Updates(map[string]interface{}{
			"assigned_to": userID,
			"assigned_at": time.Now(),
			"status":      "in_progress",
			"updated_at":  time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("failed to assign ticket: %w", err)
	}

	// Обновляем запись в очереди
	queueEntry.AssignedAt = &[]time.Time{time.Now()}[0]
	if err := q.db.Save(&queueEntry).Error; err != nil {
		return fmt.Errorf("failed to update queue entry: %w", err)
	}

	// Публикуем событие
	q.publishAssignment(ctx, ticketID, userID)

	return nil
}

// UnassignTicket возвращает тикет в очередь
func (q *TicketQueue) UnassignTicket(ctx context.Context, ticketID uuid.UUID) error {
	var ticket db.Ticket
	if err := q.db.Where("id = ?", ticketID).First(&ticket).Error; err != nil {
		return fmt.Errorf("ticket not found: %w", err)
	}

	// Сбрасываем назначение
	if err := q.db.Model(&ticket).
		Updates(map[string]interface{}{
			"assigned_to": nil,
			"assigned_at": nil,
			"status":      "new",
			"updated_at":  time.Now(),
		}).Error; err != nil {
		return fmt.Errorf("failed to unassign ticket: %w", err)
	}

	// Возвращаем в очередь с пересчетом приоритета
	if err := q.AddTicket(ctx, ticketID, ticket.DispatcherID); err != nil {
		return fmt.Errorf("failed to return ticket to queue: %w", err)
	}

	// Удаляем запись о назначении в очереди
	if err := q.db.Model(&db.TicketQueue{}).
		Where("ticket_id = ?", ticketID).
		Update("assigned_at", nil).Error; err != nil {
		return fmt.Errorf("failed to update queue entry: %w", err)
	}

	q.publishUnassignment(ctx, ticketID)

	return nil
}

// GetQueue возвращает текущую очередь
func (q *TicketQueue) GetQueue(ctx context.Context, dispatcherID uuid.UUID, limit int) ([]db.TicketQueue, error) {
	key := fmt.Sprintf("queue:dispatcher:%s", dispatcherID.String())

	// Получаем из Redis
	result, err := q.redis.ZRevRangeWithScores(ctx, key, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get queue from redis: %w", err)
	}

	// Получаем полные данные из БД
	var queueEntries []db.TicketQueue
	for _, item := range result {
		ticketID, err := uuid.Parse(item.Member.(string))
		if err != nil {
			continue
		}

		var entry db.TicketQueue
		if err := q.db.
			Preload("Ticket").
			Preload("Ticket.Client").
			Preload("Ticket.Channel").
			Where("ticket_id = ? AND dispatcher_id = ?", ticketID, dispatcherID).
			First(&entry).Error; err != nil {
			continue
		}

		queueEntries = append(queueEntries, entry)
	}

	return queueEntries, nil
}

// calculatePriorityScore вычисляет приоритетный балл
func (q *TicketQueue) calculatePriorityScore(ctx context.Context, ticketID uuid.UUID) (int, error) {
	var ticket db.Ticket
	if err := q.db.Preload("Client").Where("id = ?", ticketID).First(&ticket).Error; err != nil {
		return 0, fmt.Errorf("failed to get ticket: %w", err)
	}

	score := PriorityScore{
		BaseScore:     50,
		WaitingTime:   int(time.Since(ticket.CreatedAt).Minutes()),
		CategoryScore: q.getCategoryScore(ticket.Category),
	}

	// Приоритет по срочности
	switch ticket.Priority {
	case "urgent":
		score.UrgentScore = 50
	case "high":
		score.UrgentScore = 30
	case "medium":
		score.UrgentScore = 15
	case "low":
		score.UrgentScore = 0
	}

	// Бонус для важных клиентов
	if ticket.Client.Metadata != nil {
		if vip, ok := ticket.Client.Metadata["vip"].(bool); ok && vip {
			score.ClientValue = 30
		}
	}

	totalScore := score.BaseScore +
		score.UrgentScore +
		score.WaitingTime +
		score.ClientValue +
		score.CategoryScore

	// Ограничиваем максимальный балл
	if totalScore > 1000 {
		totalScore = 1000
	}

	return totalScore, nil
}

func (q *TicketQueue) getCategoryScore(category string) int {
	scores := map[string]int{
		"техническая":  20,
		"финансовая":   15,
		"HR":           10,
		"общая":        5,
		"жалоба":       25,
		"чрезвычайная": 30,
	}

	if score, ok := scores[category]; ok {
		return score
	}
	return 5
}

func (q *TicketQueue) publishQueueUpdate(ctx context.Context, dispatcherID uuid.UUID) {
	channel := fmt.Sprintf("queue:update:%s", dispatcherID.String())
	q.redis.Publish(ctx, channel, "updated")
}

func (q *TicketQueue) publishAssignment(ctx context.Context, ticketID, userID uuid.UUID) {
	channel := fmt.Sprintf("assignment:%s", ticketID.String())
	q.redis.Publish(ctx, channel, userID.String())
}

func (q *TicketQueue) publishUnassignment(ctx context.Context, ticketID uuid.UUID) {
	channel := fmt.Sprintf("unassignment:%s", ticketID.String())
	q.redis.Publish(ctx, channel, "unassigned")
}
