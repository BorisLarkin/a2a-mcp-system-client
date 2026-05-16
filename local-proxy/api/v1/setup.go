package v1

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"local-proxy/internal/config"
	"local-proxy/internal/db"
)

type SetupHandler struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewSetupHandler(database *gorm.DB, cfg *config.Config) *SetupHandler {
	return &SetupHandler{db: database, cfg: cfg}
}

type SetupRequest struct {
	Action string `json:"action" binding:"required"` // "register" | "connect"
	// Для register
	CompanyName string `json:"company_name"`
	Email       string `json:"email"`
	// Для connect
	APIKey       string `json:"api_key"`
	DispatcherID string `json:"dispatcher_id"`
}

func (h *SetupHandler) Setup(c *gin.Context) {
	var req SetupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	switch req.Action {
	case "register":
		h.handleRegister(c, &req)
	case "connect":
		h.handleConnect(c, &req)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unknown action"})
	}
	// После успешного setup — перезапускаем сервер
	go func() {
		time.Sleep(500 * time.Millisecond)
		log.Println("Setup complete, restarting server...")
		os.Exit(0) // Docker перезапустит контейнер
	}()
}

func (h *SetupHandler) handleRegister(c *gin.Context, req *SetupRequest) {
	// Вызываем оркестратор для регистрации
	orchURL := h.cfg.Orchestrator.URL + "/api/v1/admin/dispatchers"
	body, _ := json.Marshal(map[string]string{
		"company_name": req.CompanyName,
		"email":        req.Email,
	})

	httpClient := &http.Client{Timeout: 30 * time.Second}
	httpReq, _ := http.NewRequest("POST", orchURL, bytes.NewReader(body))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Admin-Key", "super_secret_admin_key") // SaaS-ключ из конфига

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Orchestrator unreachable: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		var errResp map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&errResp)
		c.JSON(resp.StatusCode, gin.H{"error": errResp["error"]})
		return
	}

	var orchResult struct {
		DispatcherID  string `json:"dispatcher_id"`
		APIKey        string `json:"api_key"`
		AdminUsername string `json:"admin_username"`
		AdminPassword string `json:"admin_password"`
	}
	json.NewDecoder(resp.Body).Decode(&orchResult)

	// Хешируем пароль для локального хранения
	//hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(orchResult.AdminPassword), bcrypt.DefaultCost)

	// Сохраняем диспетчерскую локально
	dispID, _ := uuid.Parse(orchResult.DispatcherID)
	dispatcher := db.Dispatcher{
		ID:                       dispID,
		Name:                     req.CompanyName,
		OrchestratorAPIKey:       orchResult.APIKey,
		OrchestratorDispatcherID: &dispID,
		IsActive:                 true,
	}
	h.db.Create(&dispatcher)

	// Создаём пользователя-администратора
	adminUser := db.User{
		Username:     orchResult.AdminUsername,
		PasswordHash: orchResult.AdminPassword,
		FullName:     "Administrator",
		Role:         "admin",
		DispatcherID: dispID,
		IsActive:     true,
	}
	h.db.Create(&adminUser)

	c.JSON(http.StatusCreated, gin.H{
		"message":        "Dispatcher registered successfully",
		"admin_username": orchResult.AdminUsername,
		"admin_password": orchResult.AdminPassword,
		"api_key":        orchResult.APIKey,
		"dispatcher_id":  orchResult.DispatcherID,
	})
}

func (h *SetupHandler) handleConnect(c *gin.Context, req *SetupRequest) {
	// Валидируем ключ через оркестратор
	orchURL := h.cfg.Orchestrator.URL + "/api/v1/dispatchers/validate"
	body, _ := json.Marshal(map[string]string{
		"api_key":       req.APIKey,
		"dispatcher_id": req.DispatcherID,
	})

	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Post(orchURL, "application/json", bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "Orchestrator unreachable"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key or dispatcher ID"})
		return
	}

	dispID, _ := uuid.Parse(req.DispatcherID)

	// Проверяем, существует ли уже диспетчерская
	var existing db.Dispatcher
	if err := h.db.Where("id = ?", dispID).First(&existing).Error; err == nil {
		// Обновляем существующую
		h.db.Model(&existing).Updates(map[string]interface{}{
			"orchestrator_api_key": req.APIKey,
			"is_active":            true,
		})
	} else {
		// Создаём новую
		dispatcher := db.Dispatcher{
			ID:                       dispID,
			OrchestratorAPIKey:       req.APIKey,
			OrchestratorDispatcherID: &dispID,
			Name:                     "Imported Dispatcher",
			IsActive:                 true,
		}
		h.db.Create(&dispatcher)
	}

	// Создаём админа, если нет пользователей
	var userCount int64
	h.db.Model(&db.User{}).Count(&userCount)
	if userCount == 0 {
		adminUser := db.User{
			Username:     "admin",
			PasswordHash: "admin123",
			Role:         "admin",
			DispatcherID: dispID,
			IsActive:     true,
		}
		h.db.Create(&adminUser)
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "Connected successfully",
		"admin_username": "admin",
		"admin_password": "admin123",
	})
}