package services

import (
	"local-proxy/internal/db"
	"log"
	"time"

	"gorm.io/gorm"
)

type EscalationService struct {
	db       *gorm.DB
	interval time.Duration
	timeout  time.Duration
}

func NewEscalationService(database *gorm.DB, checkInterval, escalationTimeout time.Duration) *EscalationService {
	return &EscalationService{
		db:       database,
		interval: checkInterval,
		timeout:  escalationTimeout,
	}
}

func (s *EscalationService) Start() {
	log.Printf("Escalation service started (check every %v, timeout %v)", s.interval, s.timeout)
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for range ticker.C {
		s.checkAndEscalate()
	}
}

func (s *EscalationService) checkAndEscalate() {
	cutoff := time.Now().Add(-s.timeout)

	var tickets []db.Ticket
	s.db.Where("status = 'waiting_for_feedback' AND ai_processed_at < ?", cutoff).
		Find(&tickets)

	for _, t := range tickets {
		log.Printf("Auto-escalating ticket %s (waiting_for_feedback since %v)", t.ID, t.AIProcessedAt)
		s.db.Model(&t).Updates(map[string]interface{}{
			"status":          "waiting",
			"feedback_status": "escalate_timeout",
			"escalated_at":    time.Now(),
		})

		// Добавляем в очередь
		s.db.Create(&db.TicketQueue{
			TicketID:      t.ID,
			DispatcherID:  t.DispatcherID,
			PriorityScore: 5,
			QueuedAt:      time.Now(),
		})
	}

	if len(tickets) > 0 {
		log.Printf("Auto-escalated %d tickets", len(tickets))
	}
}
