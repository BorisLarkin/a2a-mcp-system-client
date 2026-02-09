/*
func (s *LocalServer) setupRoutes() {
	// ... существующие маршруты ...

	// Аутентификация
	r.POST("/api/v1/auth/login", s.handleLogin)
	r.POST("/api/v1/auth/logout", s.handleLogout)
	r.POST("/api/v1/auth/refresh", s.handleRefreshToken)

	// Управление операторами
	r.GET("/api/v1/operators", s.authMiddleware, s.handleGetOperators)
	r.POST("/api/v1/operators", s.authMiddleware, s.handleCreateOperator)
	r.PUT("/api/v1/operators/:id", s.authMiddleware, s.handleUpdateOperator)

	// Работа с очередью
	r.GET("/api/v1/queue", s.authMiddleware, s.handleGetQueue)
	r.POST("/api/v1/tickets/:id/take", s.authMiddleware, s.handleTakeTicket)
	r.POST("/api/v1/tickets/:id/respond", s.authMiddleware, s.handleRespondToTicket)

	// Настройки
	r.GET("/api/v1/settings", s.authMiddleware, s.handleGetSettings)
	r.POST("/api/v1/settings", s.authMiddleware, s.handleUpdateSettings)

	// Шаблоны ответов
	r.GET("/api/v1/templates", s.handleGetTemplates)
	r.POST("/api/v1/templates", s.authMiddleware, s.handleCreateTemplate)

	// Отчеты
	r.GET("/api/v1/reports/daily", s.authMiddleware, s.handleDailyReport)
	r.GET("/api/v1/reports/operators", s.authMiddleware, s.handleOperatorsReport)
}
*/