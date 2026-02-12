#!/bin/bash
# ~/git/a2a-mcp-system-client/start.sh

echo "🚀 Запуск клиентской части системы поддержки"

# Установка цветов для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Функция проверки статуса
check_status() {
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✅ $1${NC}"
    else
        echo -e "${RED}❌ $1${NC}"
        exit 1
    fi
}

# 1. Проверка переменных окружения
echo "📁 Проверка .env файлов..."
if [ ! -f .env ]; then
    cp .env.example .env
    echo -e "${YELLOW}⚠️  Создан .env файл. Отредактируйте его перед запуском!${NC}"
    exit 1
fi

# 2. Загрузка переменных
set -a
source .env
set +a

# 3. Запуск Docker контейнеров
echo "🐳 Запуск PostgreSQL и Redis..."
docker compose -f docker-compose.full.yml up -d postgres redis
sleep 5
check_status "Базы данных запущены"

# 4. Инициализация БД
echo "🗄️  Инициализация базы данных..."
docker exec -i mcp-client-postgres psql -U mcp_client -d mcp_client_db < local-db/init.sql
check_status "База данных инициализирована"

# 5. Создание тестового пользователя
echo "👤 Создание тестового администратора..."
docker exec -i mcp-client-postgres psql -U mcp_client -d mcp_client_db << EOF
INSERT INTO dispatchers (id, name, orchestrator_api_key, settings, created_at, updated_at)
SELECT '11111111-1111-1111-1111-111111111111', 'Тестовая диспетчерская', 'sk_test', '{}'::jsonb, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM dispatchers WHERE id = '11111111-1111-1111-1111-111111111111');

INSERT INTO users (id, username, email, full_name, role, password_hash, dispatcher_id, is_active, created_at, updated_at)
SELECT '22222222-2222-2222-2222-222222222222', 'admin', 'admin@example.com', 'Administrator', 'admin', '\$2a\$10\$N9qo8uLOickgx2ZMRZoMy.MrZ7QKxQXK3XQ5Q5Q5Q5Q5Q5Q5Q5Q5', '11111111-1111-1111-1111-111111111111', true, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM users WHERE username = 'admin');
EOF
check_status "Администратор создан"

# 6. Запуск local-proxy в Docker
echo "🔄 Запуск local-proxy..."
docker compose -f docker-compose.full.yml up -d local-proxy
sleep 3
check_status "Local-proxy запущен"

# 7. Проверка работоспособности
echo "🔍 Проверка API..."
curl -s http://localhost:8080/health | grep -q "ok"
check_status "API доступен"

echo ""
echo -e "${GREEN}🎉 Система успешно запущена!${NC}"
echo ""
echo "📊 Доступные сервисы:"
echo "   • API:          http://localhost:8080"
echo "   • Health check: http://localhost:8080/health"
echo "   • PostgreSQL:   localhost:5432"
echo "   • Redis:        localhost:6379"
echo "   • Redis Admin:  http://localhost:8081 (если включен)"
echo ""
echo "🔑 Тестовый доступ:"
echo "   • Логин:    admin"
echo "   • Пароль:   admin123"
echo ""
echo "📋 Логи: docker compose -f docker-compose.full.yml logs -f"