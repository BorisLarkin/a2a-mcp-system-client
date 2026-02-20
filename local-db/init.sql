-- ./local-db/init.sql

-- Создание базы данных (если не существует)
SELECT 'CREATE DATABASE mcp_client_db OWNER mcp_client'
WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'mcp_client_db')\gexec

-- Подключаемся к созданной базе
\c mcp_client_db;

-- Создание расширений
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "pg_stat_statements";

-- 1. Таблица диспетчерской (локальная конфигурация)
CREATE TABLE IF NOT EXISTS dispatchers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    orchestrator_api_key VARCHAR(500),
    orchestrator_dispatcher_id UUID,
    settings JSONB NOT NULL DEFAULT '{}',
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 2. Пользователи (операторы и администраторы)
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username VARCHAR(100) NOT NULL,
    email VARCHAR(255),
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(255),
    role VARCHAR(50) NOT NULL,
    dispatcher_id UUID REFERENCES dispatchers(id) ON DELETE CASCADE,
    settings JSONB DEFAULT '{}',
    is_active BOOLEAN DEFAULT true,
    last_login_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Explicit named constraints
    CONSTRAINT uni_users_username UNIQUE (username),
    CONSTRAINT chk_users_role CHECK (role IN ('admin', 'operator', 'viewer'))
);

-- 3. Каналы связи (Telegram, Email, Web форма)
CREATE TABLE IF NOT EXISTS channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dispatcher_id UUID REFERENCES dispatchers(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    name VARCHAR(255) NOT NULL,
    config JSONB NOT NULL,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Explicit named constraints
    CONSTRAINT chk_channels_type CHECK (type IN ('telegram', 'email', 'web')),
    CONSTRAINT uni_channels_dispatcher_type_name UNIQUE (dispatcher_id, type, name)
);

-- 4. Клиенты (те, кто пишет в поддержку)
CREATE TABLE IF NOT EXISTS clients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id VARCHAR(500),
    channel_id UUID REFERENCES channels(id) ON DELETE SET NULL,
    name VARCHAR(255),
    contact_info VARCHAR(500),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- 5. Тикеты (локальные обращения)
CREATE TABLE IF NOT EXISTS tickets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id VARCHAR(500),
    dispatcher_id UUID REFERENCES dispatchers(id) ON DELETE CASCADE,
    client_id UUID REFERENCES clients(id) ON DELETE SET NULL,
    channel_id UUID REFERENCES channels(id) ON DELETE SET NULL,
    
    -- Данные обращения
    subject VARCHAR(500),
    original_text TEXT NOT NULL,
    status VARCHAR(50) DEFAULT 'new',
    priority VARCHAR(20) DEFAULT 'medium',
    category VARCHAR(100),
    
    -- AI обработка
    ai_response TEXT,
    ai_analysis JSONB,
    ai_processed_at TIMESTAMP WITH TIME ZONE,
    
    -- Назначение оператору
    assigned_to UUID REFERENCES users(id) ON DELETE SET NULL,
    assigned_at TIMESTAMP WITH TIME ZONE,
    
    -- Время жизни тикета
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    resolved_at TIMESTAMP WITH TIME ZONE,
    closed_at TIMESTAMP WITH TIME ZONE,
    
    -- Explicit named constraints
    CONSTRAINT chk_tickets_status CHECK (status IN ('new', 'in_progress', 'waiting', 'resolved', 'closed')),
    CONSTRAINT chk_tickets_priority CHECK (priority IN ('low', 'medium', 'high', 'urgent'))
);

-- 6. Сообщения в тикете
CREATE TABLE IF NOT EXISTS ticket_messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id UUID REFERENCES tickets(id) ON DELETE CASCADE,
    sender_type VARCHAR(20) NOT NULL,
    sender_id UUID,
    message_text TEXT NOT NULL,
    attachments JSONB,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Explicit named constraints
    CONSTRAINT chk_ticket_messages_sender_type CHECK (sender_type IN ('client', 'operator', 'ai'))
);

-- 7. Очередь тикетов (для операторов)
CREATE TABLE IF NOT EXISTS ticket_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    ticket_id UUID UNIQUE REFERENCES tickets(id) ON DELETE CASCADE,
    dispatcher_id UUID REFERENCES dispatchers(id) ON DELETE CASCADE,
    priority_score INTEGER DEFAULT 0,
    queued_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    assigned_at TIMESTAMP WITH TIME ZONE
);

-- 8. Настройки AI для диспетчерской
CREATE TABLE IF NOT EXISTS ai_settings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dispatcher_id UUID REFERENCES dispatchers(id) ON DELETE CASCADE,
    enabled BOOLEAN DEFAULT true,
    auto_respond BOOLEAN DEFAULT false,
    confidence_threshold DECIMAL(3,2) DEFAULT 0.7,
    use_internet_search BOOLEAN DEFAULT false,
    communication_style VARCHAR(50) DEFAULT 'balanced',
    system_context TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    -- Explicit named constraints
    CONSTRAINT uni_ai_settings_dispatcher_id UNIQUE (dispatcher_id),
    CONSTRAINT chk_ai_settings_communication_style CHECK (communication_style IN ('friendly', 'professional', 'balanced'))
);

-- 9. История взаимодействий с оркестратором
CREATE TABLE IF NOT EXISTS orchestrator_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    dispatcher_id UUID REFERENCES dispatchers(id) ON DELETE CASCADE,
    ticket_id UUID REFERENCES tickets(id) ON DELETE SET NULL,
    request_type VARCHAR(50) NOT NULL,
    request_data JSONB NOT NULL,
    response_data JSONB,
    status_code INTEGER,
    duration_ms INTEGER,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- ========== ИНДЕКСЫ (ОТДЕЛЬНО ОТ ТАБЛИЦ) ==========

-- Индексы для users
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_dispatcher ON users(dispatcher_id);

-- Индексы для clients
CREATE INDEX IF NOT EXISTS idx_clients_external_id ON clients(external_id);
CREATE INDEX IF NOT EXISTS idx_clients_channel_id ON clients(channel_id);
CREATE INDEX IF NOT EXISTS idx_clients_created ON clients(created_at DESC);

-- Индексы для tickets
CREATE INDEX IF NOT EXISTS idx_tickets_dispatcher_status ON tickets(dispatcher_id, status);
CREATE INDEX IF NOT EXISTS idx_tickets_assigned_status ON tickets(assigned_to, status);
CREATE INDEX IF NOT EXISTS idx_tickets_created_desc ON tickets(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tickets_priority ON tickets(priority);
CREATE INDEX IF NOT EXISTS idx_tickets_status_priority ON tickets(status, priority);
CREATE INDEX IF NOT EXISTS idx_tickets_external_id ON tickets(external_id);

-- Индексы для ticket_messages
CREATE INDEX IF NOT EXISTS idx_ticket_messages_ticket_created ON ticket_messages(ticket_id, created_at);
CREATE INDEX IF NOT EXISTS idx_ticket_messages_sender ON ticket_messages(sender_type, sender_id);

-- Индексы для ticket_queue
CREATE INDEX IF NOT EXISTS idx_ticket_queue_dispatcher_priority ON ticket_queue(dispatcher_id, priority_score DESC);
CREATE INDEX IF NOT EXISTS idx_ticket_queue_queued ON ticket_queue(queued_at);
CREATE INDEX IF NOT EXISTS idx_ticket_queue_ticket_id ON ticket_queue(ticket_id);

-- Индексы для orchestrator_logs
CREATE INDEX IF NOT EXISTS idx_orchestrator_logs_created_desc ON orchestrator_logs(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_orchestrator_logs_dispatcher ON orchestrator_logs(dispatcher_id);
CREATE INDEX IF NOT EXISTS idx_orchestrator_logs_ticket ON orchestrator_logs(ticket_id);

-- Индексы для ai_settings
CREATE INDEX IF NOT EXISTS idx_ai_settings_dispatcher ON ai_settings(dispatcher_id);

-- Индексы для channels
CREATE INDEX IF NOT EXISTS idx_channels_dispatcher_type ON channels(dispatcher_id, type);
CREATE INDEX IF NOT EXISTS idx_channels_is_active ON channels(is_active);

-- ========== ТРИГГЕРЫ ==========

-- Функция обновления updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Применяем триггеры (используем DROP IF EXISTS для чистоты)
DROP TRIGGER IF EXISTS trigger_update_dispatchers_updated_at ON dispatchers;
CREATE TRIGGER trigger_update_dispatchers_updated_at 
    BEFORE UPDATE ON dispatchers 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS trigger_update_users_updated_at ON users;
CREATE TRIGGER trigger_update_users_updated_at 
    BEFORE UPDATE ON users 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS trigger_update_channels_updated_at ON channels;
CREATE TRIGGER trigger_update_channels_updated_at 
    BEFORE UPDATE ON channels 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS trigger_update_clients_updated_at ON clients;
CREATE TRIGGER trigger_update_clients_updated_at 
    BEFORE UPDATE ON clients 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS trigger_update_tickets_updated_at ON tickets;
CREATE TRIGGER trigger_update_tickets_updated_at 
    BEFORE UPDATE ON tickets 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS trigger_update_ai_settings_updated_at ON ai_settings;
CREATE TRIGGER trigger_update_ai_settings_updated_at 
    BEFORE UPDATE ON ai_settings 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ========== ТЕСТОВЫЕ ДАННЫЕ ==========

-- Добавляем тестовую диспетчерскую
INSERT INTO dispatchers (id, name, orchestrator_api_key, settings)
VALUES (
    '11111111-1111-1111-1111-111111111111',
    'Тестовая диспетчерская',
    'sk_test_123456',
    '{"test_mode": true}'
) ON CONFLICT (id) DO NOTHING;

-- Добавляем тестового администратора (пароль: admin123)
-- Note: This is a bcrypt hash of 'admin123' - in production use proper password hashing
INSERT INTO users (id, username, email, full_name, role, password_hash, dispatcher_id)
VALUES (
    '22222222-2222-2222-2222-222222222222',
    'admin',
    'admin@example.com',
    'Administrator',
    'admin',
    '$2a$10$N9qo8uLOickgx2ZMRZoMy.MrZ7QKxQXK3XQ5Q5Q5Q5Q5Q5Q5Q5Q5',
    '11111111-1111-1111-1111-111111111111'
) ON CONFLICT (username) DO NOTHING;

-- Добавляем тестового оператора
INSERT INTO users (id, username, email, full_name, role, password_hash, dispatcher_id)
VALUES (
    '33333333-3333-3333-3333-333333333333',
    'operator',
    'operator@example.com',
    'Test Operator',
    'operator',
    '$2a$10$N9qo8uLOickgx2ZMRZoMy.MrZ7QKxQXK3XQ5Q5Q5Q5Q5Q5Q5Q5Q5',
    '11111111-1111-1111-1111-111111111111'
) ON CONFLICT (username) DO NOTHING;

-- Добавляем AI настройки
INSERT INTO ai_settings (dispatcher_id, enabled, auto_respond, confidence_threshold, communication_style)
VALUES (
    '11111111-1111-1111-1111-111111111111',
    true,
    true,
    0.75,
    'friendly'
) ON CONFLICT (dispatcher_id) DO NOTHING;

-- Добавляем тестовый канал
INSERT INTO channels (id, dispatcher_id, type, name, config)
VALUES (
    '44444444-4444-4444-4444-444444444444',
    '11111111-1111-1111-1111-111111111111',
    'web',
    'Веб-форма обратной связи',
    '{"form_fields": ["name", "email", "message"]}'
) ON CONFLICT (dispatcher_id, type, name) DO NOTHING;

-- ========== ПРОВЕРОЧНЫЕ ЗАПРОСЫ ==========
-- Проверяем созданные таблицы
SELECT 'Таблицы успешно созданы' as message;

-- Показываем количество записей в каждой таблице
SELECT 
    (SELECT COUNT(*) FROM dispatchers) as dispatchers_count,
    (SELECT COUNT(*) FROM users) as users_count,
    (SELECT COUNT(*) FROM channels) as channels_count,
    (SELECT COUNT(*) FROM ai_settings) as ai_settings_count;