// ./local-proxy/internal/db/models.go
package db

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Dispatcher - локальная диспетчерская
type Dispatcher struct {
	ID                       uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Name                     string     `gorm:"not null"`
	OrchestratorAPIKey       string     `gorm:"type:varchar(500)"`
	OrchestratorDispatcherID *uuid.UUID `gorm:"type:uuid"`
	Settings                 JSONB      `gorm:"type:jsonb;default:'{}'"`
	IsActive                 bool       `gorm:"default:true"`
	CreatedAt                time.Time
	UpdatedAt                time.Time
}

// User - оператор или администратор
type User struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Username     string    `gorm:"type:varchar(100);uniqueIndex;not null"`
	Email        string    `gorm:"type:varchar(255)"`
	PasswordHash string    `gorm:"type:varchar(255);not null"`
	FullName     string    `gorm:"type:varchar(255)"`
	Role         string    `gorm:"type:varchar(50);not null"` // admin, operator, viewer
	DispatcherID uuid.UUID `gorm:"type:uuid;not null"`
	Dispatcher   Dispatcher
	Settings     JSONB `gorm:"type:jsonb;default:'{}'"`
	IsActive     bool  `gorm:"default:true"`
	LastLoginAt  *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Channel - канал связи (Telegram, Email, Web)
type Channel struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	DispatcherID uuid.UUID `gorm:"type:uuid;not null"`
	Dispatcher   Dispatcher
	Type         string `gorm:"type:varchar(50);not null"` // telegram, email, web
	Name         string `gorm:"type:varchar(255);not null"`
	Config       JSONB  `gorm:"type:jsonb;not null"`
	IsActive     bool   `gorm:"default:true"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Client - клиент (отправитель обращения)
type Client struct {
	ID          uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ExternalID  string     `gorm:"type:varchar(500)"` // telegram_chat_id, email
	ChannelID   *uuid.UUID `gorm:"type:uuid"`
	Channel     Channel
	Name        string `gorm:"type:varchar(255)"`
	ContactInfo string `gorm:"type:varchar(500)"`
	Metadata    JSONB  `gorm:"type:jsonb;default:'{}'"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Ticket - тикет (обращение)
type Ticket struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	ExternalID    string    `gorm:"type:varchar(500)"` // ID в SaaS системе
	DispatcherID  uuid.UUID `gorm:"type:uuid;not null"`
	Dispatcher    Dispatcher
	ClientID      *uuid.UUID `gorm:"type:uuid"`
	Client        Client
	ChannelID     *uuid.UUID `gorm:"type:uuid"`
	Channel       Channel
	Subject       string `gorm:"type:varchar(500)"`
	OriginalText  string `gorm:"type:text;not null"`
	Status        string `gorm:"type:varchar(50);default:'new'"`    // new, in_progress, waiting, resolved, closed
	Priority      string `gorm:"type:varchar(20);default:'medium'"` // low, medium, high, urgent
	Category      string `gorm:"type:varchar(100)"`
	AIResponse    string `gorm:"type:text"`
	AIAnalysis    JSONB  `gorm:"type:jsonb"`
	AIProcessedAt *time.Time
	AssignedTo    *uuid.UUID `gorm:"type:uuid"`
	AssignedUser  User       `gorm:"foreignKey:AssignedTo"`
	AssignedAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	ResolvedAt    *time.Time
	ClosedAt      *time.Time
}

// TicketMessage - сообщение в тикете
type TicketMessage struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TicketID    uuid.UUID `gorm:"type:uuid;not null"`
	Ticket      Ticket
	SenderType  string     `gorm:"type:varchar(20);not null"` // client, operator, ai
	SenderID    *uuid.UUID `gorm:"type:uuid"`
	MessageText string     `gorm:"type:text;not null"`
	Attachments JSONB      `gorm:"type:jsonb"`
	Metadata    JSONB      `gorm:"type:jsonb;default:'{}'"`
	CreatedAt   time.Time
}

// TicketQueue - очередь тикетов
type TicketQueue struct {
	ID            uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	TicketID      uuid.UUID `gorm:"type:uuid;uniqueIndex;not null"`
	Ticket        Ticket
	DispatcherID  uuid.UUID `gorm:"type:uuid;not null"`
	Dispatcher    Dispatcher
	PriorityScore int `gorm:"default:0"`
	QueuedAt      time.Time
	AssignedAt    *time.Time
}

// AISettings - настройки AI для диспетчерской
type AISettings struct {
	ID                  uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	DispatcherID        uuid.UUID `gorm:"type:uuid;uniqueIndex;not null"`
	Dispatcher          Dispatcher
	Enabled             bool    `gorm:"default:true"`
	AutoRespond         bool    `gorm:"default:false"`
	ConfidenceThreshold float64 `gorm:"type:decimal(3,2);default:0.7"`
	UseInternetSearch   bool    `gorm:"default:false"`
	CommunicationStyle  string  `gorm:"type:varchar(50);default:'balanced'"`
	SystemContext       string  `gorm:"type:text"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

// OrchestratorLog - лог вызовов к оркестратору
type OrchestratorLog struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	DispatcherID uuid.UUID `gorm:"type:uuid;not null"`
	Dispatcher   Dispatcher
	TicketID     *uuid.UUID `gorm:"type:uuid"`
	Ticket       Ticket
	RequestType  string `gorm:"type:varchar(50);not null"`
	RequestData  JSONB  `gorm:"type:jsonb;not null"`
	ResponseData JSONB  `gorm:"type:jsonb"`
	StatusCode   int
	DurationMs   int
	ErrorMessage string `gorm:"type:text"`
	CreatedAt    time.Time
}

// JSONB - тип для работы с JSONB в GORM
type JSONB map[string]interface{}

func (j JSONB) GormDataType() string {
	return "jsonb"
}

// BeforeCreate - хуки для автоматической генерации UUID
func (d *Dispatcher) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}
