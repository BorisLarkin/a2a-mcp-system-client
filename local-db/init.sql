-- ============================================
-- ЛОКАЛЬНАЯ БАЗА ДАННЫХ ДИСПЕТЧЕРСКОЙ
-- Хранит данные сотрудников, настройки, кэш тикетов
-- ============================================

-- 1. СОТРУДНИКИ ДИСПЕТЧЕРСКОЙ
CREATE TABLE сотрудники (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    внешний_id UUID,  -- ID в облачной системе (если синхронизируется)
    
    -- Учетные данные
    логин VARCHAR(100) UNIQUE NOT NULL,
    хеш_пароля VARCHAR(255) NOT NULL,
    email VARCHAR(255) UNIQUE,
    телефон VARCHAR(50),
    
    -- Роль и права
    роль VARCHAR(50) NOT NULL DEFAULT 'оператор',
        -- 'администратор', 'супервайзер', 'оператор', 'аналитик'
    
    права JSONB DEFAULT '{}',
        -- {
        --   "can_manage_users": false,
        --   "can_view_reports": true,
        --   "can_escalate": true,
        --   "can_edit_config": false
        -- }
    
    -- Контакты для уведомлений
    telegram_chat_id BIGINT,
    email_notifications BOOLEAN DEFAULT true,
    telegram_notifications BOOLEAN DEFAULT true,
    
    -- Статус
    активен BOOLEAN DEFAULT true,
    последний_вход TIMESTAMP,
    создан TIMESTAMP DEFAULT NOW(),
    обновлён TIMESTAMP DEFAULT NOW(),
    
    INDEX idx_логин (логин),
    INDEX idx_активен (активен),
    INDEX idx_роль (роль)
);

-- 2. НАСТРОЙКИ ДИСПЕТЧЕРСКОЙ (локальные настройки)
CREATE TABLE настройки_диспетчерской (
    диспетчерская_id UUID PRIMARY KEY,  -- ID из облачной системы
    внешнее_название VARCHAR(255) NOT NULL,
    
    -- КОНФИГУРАЦИЯ AI (кэш из облака + локальные правки)
    стиль_общения VARCHAR(50) DEFAULT 'balanced',
        -- 'formal', 'friendly', 'technical', 'balanced'
    
    порог_уверенности DECIMAL(3,2) DEFAULT 0.70,
    интернет_поиск BOOLEAN DEFAULT false,
    
    -- ЛОКАЛЬНЫЕ НАСТРОЙКИ (не в облаке)
    автоответ_при_оффлайн BOOLEAN DEFAULT true,
    текст_автоответа TEXT DEFAULT 'Мы получили ваше обращение. Ответим в ближайшее время.',
    
    уведомлять_операторов BOOLEAN DEFAULT true,
    порог_для_уведомления DECIMAL(3,2) DEFAULT 0.30,
        -- если уверенность AI ниже этого значения → уведомляем оператора
    
    эскалация_на_почту VARCHAR(255),
    эскалация_на_телефон VARCHAR(50),
    
    -- Расписание работы
    время_начала_работы TIME DEFAULT '09:00',
    время_окончания_работы TIME DEFAULT '18:00',
    выходные_days INTEGER[] DEFAULT '{6,0}',  -- суббота, воскресенье
    
    -- Кэшированные данные из облака
    облачный_контекст TEXT,  -- контекст компании из облака
    облачный_промпт TEXT,    -- system prompt из облака
    
    синхронизировано TIMESTAMP,
    версия_конфигурации INTEGER DEFAULT 1,
    
    создано TIMESTAMP DEFAULT NOW(),
    обновлено TIMESTAMP DEFAULT NOW()
);

-- 3. КАНАЛЫ КЛИЕНТСКОГО ВЗАИМОДЕЙСТВИЯ
CREATE TABLE каналы (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    диспетчерская_id UUID NOT NULL,
    
    тип VARCHAR(50) NOT NULL,
        -- 'telegram_bot', 'telegram_group', 'email', 'web_form', 'api'
    
    -- Конфигурация канала
    настройки JSONB NOT NULL,
        -- Для Telegram: {"bot_token": "...", "webhook_url": "...", "group_id": ...}
        -- Для Email: {"smtp_server": "...", "login": "...", "password": "...", "imap": "..."}
        -- Для Web: {"form_url": "...", "secret_key": "...", "webhook": "..."}
        -- Для API: {"api_key": "...", "endpoint": "..."}
    
    -- Статус
    активен BOOLEAN DEFAULT true,
    проверен BOOLEAN DEFAULT false,  -- успешно ли подключен
    
    -- Статистика
    сообщений_получено INTEGER DEFAULT 0,
    сообщений_отправлено INTEGER DEFAULT 0,
    последнее_сообщение TIMESTAMP,
    
    создан TIMESTAMP DEFAULT NOW(),
    
    INDEX idx_диспетчерская (диспетчерская_id),
    INDEX idx_тип (тип),
    INDEX idx_активен (активен),
    UNIQUE(диспетчерская_id, тип)
);

-- 4. КЭШ ТИКЕТОВ (основная рабочая таблица)
CREATE TABLE кэш_тикетов (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    
    -- Связи
    диспетчерская_id UUID NOT NULL,
    облачный_id UUID,          -- ID в облачной системе (после синхронизации)
    локальный_id VARCHAR(255), -- ID во внутренней системе клиента
    
    -- Источник
    канал_id UUID REFERENCES каналы(id),
    исходный_текст TEXT NOT NULL,
    метаданные JSONB DEFAULT '{}',
        -- {
        --   "user_id": "123",
        --   "username": "@ivan",
        --   "chat_id": 123456,
        --   "message_id": 456,
        --   "attachments": ["photo.jpg"],
        --   "raw_data": {...}
        -- }
    
    -- Ответ от AI (из облака)
    ответ_ai TEXT,
    анализ_ai JSONB,
        -- {
        --   "category": "техническая",
        --   "confidence": 0.87,
        --   "entities": ["интернет", "роутер"],
        --   "sources": ["rag", "internet"],
        --   "agents_used": ["classifier", "researcher", "generator"]
        -- }
    
    -- Обработка оператором (если потребовалось)
    оператор_id UUID REFERENCES сотрудники(id),
    ответ_оператора TEXT,
    комментарий_оператора TEXT,
    
    -- Статусы
    статус VARCHAR(50) DEFAULT 'получено',
        -- 'получено', 'в_обработке_ai', 'обработано_ai', 
        -- 'ожидает_оператора', 'обработано_оператором', 'закрыто'
    
    флаги JSONB DEFAULT '{}',
        -- {
        --   "требует_эскалации": false,
        --   "срочное": false,
        --   "повторное": false,
        --   "жалоба": false
        -- }
    
    -- Синхронизация с облаком
    синхронизация_статус VARCHAR(20) DEFAULT 'ожидает',
        -- 'ожидает', 'синхронизировано', 'ошибка', 'не_требуется'
    
    синхронизация_попытки INTEGER DEFAULT 0,
    синхронизация_ошибка TEXT,
    
    -- Временные метки
    получено_в TIMESTAMP DEFAULT NOW(),
    обработано_ai_в TIMESTAMP,
    взято_в_работу TIMESTAMP,
    обработано_оператором_в TIMESTAMP,
    закрыто_в TIMESTAMP,
    
    создано TIMESTAMP DEFAULT NOW(),
    
    INDEX idx_диспетчерская_статус (диспетчерская_id, статус),
    INDEX idx_синхронизация (синхронизация_статус),
    INDEX idx_получено (получено_в DESC),
    INDEX idx_канал (канал_id),
    INDEX idx_оператор (оператор_id),
    UNIQUE(диспетчерская_id, облачный_id) WHERE облачный_id IS NOT NULL
);

-- 5. ШАБЛОНЫ ОТВЕТОВ (локальная база шаблонов)
CREATE TABLE шаблоны_ответов (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    диспетчерская_id UUID NOT NULL,
    
    название VARCHAR(255) NOT NULL,
    категория VARCHAR(100),
        -- 'приветствие', 'тех_проблемы', 'финансы', 'жалобы', 'общее'
    
    текст TEXT NOT NULL,
    переменные TEXT[],
        -- ["{имя}", "{номер_заявки}", "{время}"]
    
    теги TEXT[],
    использование INTEGER DEFAULT 0,
    
    -- Применимость
    минимальная_уверенность DECIMAL(3,2) DEFAULT 0.0,
    каналы VARCHAR(50)[],  -- ['telegram', 'email', 'web']
    
    создано TIMESTAMP DEFAULT NOW(),
    обновлено TIMESTAMP DEFAULT NOW(),
    
    INDEX idx_диспетчерская_категория (диспетчерская_id, категория),
    INDEX idx_теги (теги)
);

-- 6. ЭСКАЛАЦИИ И НАЗНАЧЕНИЯ
CREATE TABLE эскалации (
    id UUID PRIMARY DEFAULT gen_random_uuid(),
    тикет_id UUID REFERENCES кэш_тикетов(id) ON DELETE CASCADE,
    
    от_оператора_id UUID REFERENCES сотрудники(id),
    к_оператору_id UUID REFERENCES сотрудники(id),
    
    причина VARCHAR(255),
    приоритет VARCHAR(20) DEFAULT 'normal',
        -- 'low', 'normal', 'high', 'critical'
    
    статус VARCHAR(50) DEFAULT 'назначено',
        -- 'назначено', 'принято', 'в_работе', 'завершено', 'отклонено'
    
    комментарий TEXT,
    срок_исполнения TIMESTAMP,
    
    создано TIMESTAMP DEFAULT NOW(),
    обновлено TIMESTAMP DEFAULT NOW(),
    
    INDEX idx_тикет (тикет_id),
    INDEX idx_к_оператору (к_оператору_id, статус),
    INDEX idx_срок (срок_исполнения)
);

-- 7. ЛОКАЛЬНАЯ СТАТИСТИКА И ОТЧЕТЫ
CREATE TABLE локальная_статистика (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    диспетчерская_id UUID NOT NULL,
    дата DATE NOT NULL,
    смена VARCHAR(20),  -- 'утро', 'день', 'вечер', 'ночь'
    
    -- Общая статистика
    всего_сообщений INTEGER DEFAULT 0,
    автоответов_ai INTEGER DEFAULT 0,
    обработано_операторами INTEGER DEFAULT 0,
    эскалаций INTEGER DEFAULT 0,
    
    -- Временные метрики
    среднее_время_ответа_ai INTEGER,  -- секунды
    среднее_время_ответа_оператора INTEGER,
    
    -- Качество
    процент_автоответов DECIMAL(5,2),
    удовлетворенность DECIMAL(3,2),  -- если есть оценка от клиентов
    
    -- Каналы
    по_каналам JSONB DEFAULT '{}',
        -- {"telegram": 150, "email": 45, "web": 30}
    
    создано TIMESTAMP DEFAULT NOW(),
    
    INDEX idx_дата (дата DESC),
    INDEX idx_диспетчерская_дата (диспетчерская_id, дата),
    UNIQUE(диспетчерская_id, дата, смена)
);

-- 8. ЖУРНАЛ ДЕЙСТВИЙ (аудит)
CREATE TABLE журнал_действий (
    id BIGSERIAL PRIMARY KEY,
    диспетчерская_id UUID NOT NULL,
    
    -- Кто
    пользователь_id UUID REFERENCES сотрудники(id),
    пользователь_тип VARCHAR(50),
        -- 'system', 'operator', 'admin', 'api'
    
    -- Что
    действие VARCHAR(100) NOT NULL,
        -- 'login', 'logout', 'create_ticket', 'respond', 'escalate',
        -- 'edit_config', 'add_user', 'sync_with_cloud'
    
    объект_тип VARCHAR(50),
        -- 'ticket', 'user', 'channel', 'template', 'config'
    
    объект_id UUID,
    
    -- Подробности
    параметры JSONB,
    ip_адрес INET,
    user_agent TEXT,
    
    -- Результат
    успешно BOOLEAN DEFAULT true,
    ошибка TEXT,
    
    создано TIMESTAMP DEFAULT NOW(),
    
    INDEX idx_дата_действия (создано DESC),
    INDEX idx_пользователь (пользователь_id),
    INDEX idx_действие (действие),
    INDEX idx_объект (объект_тип, объект_id)
);

-- 9. ОФФЛАЙН-ОЧЕРЕДЬ (для работы без интернета)
CREATE TABLE оффлайн_очередь (
    id BIGSERIAL PRIMARY KEY,
    диспетчерская_id UUID NOT NULL,
    
    тип_операции VARCHAR(50) NOT NULL,
        -- 'sync_ticket', 'sync_config', 'notify_operator', 'log_event'
    
    данные JSONB NOT NULL,
    приоритет INTEGER DEFAULT 0,  -- 0=normal, 1=high, 2=critical
    
    статус VARCHAR(20) DEFAULT 'pending',
        -- 'pending', 'processing', 'completed', 'failed', 'retry'
    
    попытки INTEGER DEFAULT 0,
    последняя_попытка TIMESTAMP,
    ошибка TEXT,
    
    срок_выполнения TIMESTAMP,
    создано TIMESTAMP DEFAULT NOW(),
    
    INDEX idx_статус_приоритет (статус, приоритет DESC),
    INDEX idx_срок (срок_выполнения),
    INDEX idx_создано (создано)
);

-- 10. API КЛЮЧИ ДЛЯ ВНУТРЕННЕГО ИСПОЛЬЗОВАНИЯ
CREATE TABLE api_ключи (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    диспетчерская_id UUID NOT NULL,
    
    название VARCHAR(255) NOT NULL,
    ключ VARCHAR(255) UNIQUE NOT NULL,
    
    -- Права
    разрешения JSONB DEFAULT '{"read": true, "write": false}',
        -- {
        --   "tickets": {"read": true, "write": true},
        --   "analytics": {"read": true, "write": false},
        --   "config": {"read": false, "write": false}
        -- }
    
    истекает TIMESTAMP,
    последнее_использование TIMESTAMP,
    
    -- Ограничения
    лимит_запросов_в_день INTEGER DEFAULT 1000,
    ip_ограничения CIDR[],
    
    активен BOOLEAN DEFAULT true,
    создан TIMESTAMP DEFAULT NOW(),
    
    INDEX idx_ключ (ключ),
    INDEX idx_диспетчерская (диспетчерская_id),
    INDEX idx_активен (активен)
);

-- ============================================
-- СИСТЕМНЫЕ ТАБЛИЦЫ (для работы прокси)
-- ============================================

-- 11. СЕССИИ ПОЛЬЗОВАТЕЛЕЙ (для веб-интерфейса)
CREATE TABLE сессии (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    пользователь_id UUID REFERENCES сотрудники(id) ON DELETE CASCADE,
    
    токен VARCHAR(512) UNIQUE NOT NULL,
    токен_обновления VARCHAR(512),
    
    устройство VARCHAR(255),
    браузер VARCHAR(255),
    ip_адрес INET,
    
    истекает TIMESTAMP NOT NULL,
    создана TIMESTAMP DEFAULT NOW(),
    последняя_активность TIMESTAMP DEFAULT NOW(),
    
    INDEX idx_токен (токен),
    INDEX idx_пользователь (пользователь_id),
    INDEX idx_истекает (истекает)
);

-- 12. КЭШ КОНФИГУРАЦИИ ОБЛАКА
CREATE TABLE кэш_конфигурации (
    диспетчерская_id UUID PRIMARY KEY,
    
    -- Данные из облака
    облачная_конфигурация JSONB NOT NULL,
        -- Полный ответ от облачного API при запросе конфигурации
    
    версия INTEGER NOT NULL,
    хеш_конфигурации VARCHAR(64),  -- для проверки изменений
    
    -- Метаданные
    загружено_из_облака TIMESTAMP,
    следующая_проверка TIMESTAMP,
    
    создано TIMESTAMP DEFAULT NOW(),
    обновлено TIMESTAMP DEFAULT NOW()
);

-- ============================================
-- ТРИГГЕРЫ И ФУНКЦИИ
-- ============================================

-- Автоматическое обновление timestamp
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.обновлено = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Применение триггеров к таблицам
DO $$ 
DECLARE 
    tbl text;
BEGIN 
    FOR tbl IN 
        SELECT table_name 
        FROM information_schema.tables 
        WHERE table_schema = 'public' 
        AND table_name IN (
            'сотрудники', 
            'настройки_диспетчерской',
            'шаблоны_ответов',
            'кэш_конфигурации'
        )
    LOOP
        EXECUTE format('
            DROP TRIGGER IF EXISTS update_%s_timestamp ON %s;
            CREATE TRIGGER update_%s_timestamp
            BEFORE UPDATE ON %s
            FOR EACH ROW EXECUTE FUNCTION update_updated_at();
        ', tbl, tbl, tbl, tbl);
    END LOOP;
END $$;

-- Функция для автоматической категоризации по времени суток
CREATE OR REPLACE FUNCTION определить_смену(время TIME)
RETURNS VARCHAR(20) AS $$
BEGIN
    RETURN CASE
        WHEN время BETWEEN TIME '00:00' AND TIME '08:00' THEN 'ночь'
        WHEN время BETWEEN TIME '08:00' AND TIME '16:00' THEN 'утро'
        WHEN время BETWEEN TIME '16:00' AND TIME '00:00' THEN 'вечер'
        ELSE 'день'
    END;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Индекс для быстрого поиска тикетов по статусу и времени
CREATE INDEX idx_тикеты_для_операторов 
ON кэш_тикетов (статус, получено_в) 
WHERE статус IN ('ожидает_оператора', 'в_обработке_ai');

-- Представление для дашборда
CREATE VIEW дашборд_статистика AS
SELECT 
    д.дата,
    д.диспетчерская_id,
    д.всего_сообщений,
    д.автоответов_ai,
    д.обработано_операторами,
    д.эскалаций,
    д.среднее_время_ответа_ai,
    д.среднее_время_ответа_оператора,
    ROUND(д.автоответов_ai::DECIMAL / NULLIF(д.всего_сообщений, 0) * 100, 2) as процент_автоответов
FROM локальная_статистика д
ORDER BY д.дата DESC;

-- Представление для операторов
CREATE VIEW очередь_операторов AS
SELECT 
    т.id,
    т.диспетчерская_id,
    т.исходный_текст,
    т.получено_в,
    т.статус,
    т.оператор_id,
    к.тип as канал_тип,
    с.логин as оператор_логин,
    -- Приоритет: срочное + давно ожидает
    CASE 
        WHEN т.флаgs->>'срочное' = 'true' THEN 100
        ELSE 0 
    END + 
    EXTRACT(EPOCH FROM (NOW() - т.получено_в)) / 3600 as приоритет_баллы
FROM кэш_тикетов т
LEFT JOIN каналы к ON т.канал_id = к.id
LEFT JOIN сотрудники с ON т.оператор_id = с.id
WHERE т.статус IN ('ожидает_оператора', 'в_обработке_ai')
ORDER BY приоритет_баллы DESC;

COMMENT ON TABLE сотрудники IS 'Сотрудники диспетчерской с правами доступа';
COMMENT ON TABLE настройки_диспетчерской IS 'Локальные настройки + кэш конфигурации из облака';
COMMENT ON TABLE кэш_тикетов IS 'Основная таблица кэшированных тикетов';
COMMENT ON TABLE шаблоны_ответов IS 'Локальная база шаблонов ответов для операторов';