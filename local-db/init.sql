-- ./local-db/init.sql

-- 1. Таблица диспетчерской (локальная конфигурация)
CREATE TABLE dispatchers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    orchestrator_api_key VARCHAR(500), -- ключ для SaaS API
    orchestrator_dispatcher_id UUID, -- ID в SaaS системе
    settings JSONB NOT NULL DEFAULT '{}',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- 2. Пользователи (операторы и администраторы)
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(100) UNIQUE NOT NULL,
    email VARCHAR(255),
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(255),
    role VARCHAR(50) NOT NULL, -- 'admin', 'operator', 'viewer'
    dispatcher_id UUID REFERENCES dispatchers(id) ON DELETE CASCADE,
    settings JSONB DEFAULT '{}',
    is_active BOOLEAN DEFAULT true,
    last_login_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- 3. Каналы связи (Telegram, Email, Web форма)
CREATE TABLE channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dispatcher_id UUID REFERENCES dispatchers(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL, -- 'telegram', 'email', 'web'
    name VARCHAR(255) NOT NULL,
    config JSONB NOT NULL, -- { "bot_token": "...", "chat_id": "...", "webhook_secret": "..." }
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    
    UNIQUE(dispatcher_id, type, name)
);

-- 4. Клиенты (те, кто пишет в поддержку)
CREATE TABLE clients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id VARCHAR(500), -- ID в канале (telegram_chat_id, email)
    channel_id UUID REFERENCES channels(id) ON DELETE SET NULL,
    name VARCHAR(255),
    contact_info VARCHAR(500),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    
    INDEX idx_external_id (external_id),
    INDEX idx_channel (channel_id)
);

-- 5. Тикеты (локальные обращения)
CREATE TABLE tickets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id VARCHAR(500), -- ID тикета в SaaS системе
    dispatcher_id UUID REFERENCES dispatchers(id) ON DELETE CASCADE,
    client_id UUID REFERENCES clients(id) ON DELETE SET NULL,
    channel_id UUID REFERENCES channels(id) ON DELETE SET NULL,
    
    -- Данные обращения
    subject VARCHAR(500),
    original_text TEXT NOT NULL,
    status VARCHAR(50) DEFAULT 'new', -- 'new', 'in_progress', 'waiting', 'resolved', 'closed'
    priority VARCHAR(20) DEFAULT 'medium', -- 'low', 'medium', 'high', 'urgent'
    category VARCHAR(100), -- определит AI классификатор
    
    -- AI обработка
    ai_response TEXT,
    ai_analysis JSONB, -- { "category": "...", "confidence": 0.9, "entities": [], "suggested_solution": "..." }
    ai_processed_at TIMESTAMP,
    
    -- Назначение оператору
    assigned_to UUID REFERENCES users(id) ON DELETE SET NULL,
    assigned_at TIMESTAMP,
    
    -- Время жизни тикета
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    resolved_at TIMESTAMP,
    closed_at TIMESTAMP,
    
    -- Индексы для быстрого поиска
    INDEX idx_dispatcher_status (dispatcher_id, status),
    INDEX idx_assigned_status (assigned_to, status),
    INDEX idx_created (created_at DESC),
    INDEX idx_priority (priority)
);

-- 6. Сообщения в тикете
CREATE TABLE ticket_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id UUID REFERENCES tickets(id) ON DELETE CASCADE,
    sender_type VARCHAR(20) NOT NULL, -- 'client', 'operator', 'ai'
    sender_id UUID, -- user_id или client_id
    message_text TEXT NOT NULL,
    attachments JSONB, -- [{"type": "image", "url": "...", "filename": "..."}]
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW(),
    
    INDEX idx_ticket (ticket_id, created_at)
);

-- 7. Очередь тикетов (для операторов)
CREATE TABLE ticket_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id UUID UNIQUE REFERENCES tickets(id) ON DELETE CASCADE,
    dispatcher_id UUID REFERENCES dispatchers(id) ON DELETE CASCADE,
    priority_score INTEGER DEFAULT 0, -- вычисляемый приоритет
    queued_at TIMESTAMP DEFAULT NOW(),
    assigned_at TIMESTAMP,
    
    INDEX idx_dispatcher_priority (dispatcher_id, priority_score DESC),
    INDEX idx_queued (queued_at)
);

-- 8. Настройки AI для диспетчерской
CREATE TABLE ai_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dispatcher_id UUID UNIQUE REFERENCES dispatchers(id) ON DELETE CASCADE,
    enabled BOOLEAN DEFAULT true,
    auto_respond BOOLEAN DEFAULT false,
    confidence_threshold DECIMAL(3,2) DEFAULT 0.7,
    use_internet_search BOOLEAN DEFAULT false,
    communication_style VARCHAR(50) DEFAULT 'balanced',
    system_context TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

-- 9. История взаимодействий с оркестратором
CREATE TABLE orchestrator_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dispatcher_id UUID REFERENCES dispatchers(id) ON DELETE CASCADE,
    ticket_id UUID REFERENCES tickets(id) ON DELETE SET NULL,
    request_type VARCHAR(50) NOT NULL, -- 'classify', 'generate', 'research'
    request_data JSONB NOT NULL,
    response_data JSONB,
    status_code INTEGER,
    duration_ms INTEGER,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    
    INDEX idx_ticket (ticket_id),
    INDEX idx_created (created_at DESC)
);

-- Триггер для обновления updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Применяем триггер к таблицам
CREATE TRIGGER update_dispatchers_updated_at BEFORE UPDATE ON dispatchers
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_channels_updated_at BEFORE UPDATE ON channels
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_clients_updated_at BEFORE UPDATE ON clients
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_tickets_updated_at BEFORE UPDATE ON tickets
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_ai_settings_updated_at BEFORE UPDATE ON ai_settings
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();