#!/bin/bash
# ./scripts/start.sh

echo "🚀 Запуск клиентской части диспетчерской..."

# Проверка переменных окружения
if [ ! -f .env ]; then
    echo "⚠️  Файл .env не найден. Создаю из примера..."
    cp .env.example .env
    echo "✏️  Отредактируйте .env файл и запустите снова"
    exit 1
fi

# Загрузка переменных окружения
set -a
source .env
set +a

# Проверка обязательных переменных
required_vars=("DB_PASSWORD" "ORCHESTRATOR_URL" "TELEGRAM_BOT_TOKEN")
for var in "${required_vars[@]}"; do
    if [ -z "${!var}" ]; then
        echo "❌ Не установлена переменная: $var"
        exit 1
    fi
done

echo "📦 Сборка и запуск Docker контейнеров..."
docker-compose down
docker-compose build --no-cache
docker-compose up -d

echo "⏳ Ожидание запуска сервисов..."
sleep 10

# Проверка здоровья сервисов
echo "🔍 Проверка состояния сервисов..."

# Проверка local-proxy
if curl -f http://localhost:8080/health > /dev/null 2>&1; then
    echo "✅ Local-proxy запущен"
else
    echo "❌ Local-proxy не отвечает"
    docker-compose logs local-proxy
    exit 1
fi

# Проверка базы данных
if docker-compose exec local-db pg_isready -U dispatcher_user > /dev/null 2>&1; then
    echo "✅ База данных запущена"
else
    echo "❌ База данных не отвечает"
    exit 1
fi

echo ""
echo "🎉 Клиентская часть успешно запущена!"
echo ""
echo "📊 Доступные интерфейсы:"
echo "   • Веб-интерфейс: http://localhost:3000"
echo "   • API сервер:    http://localhost:8080"
echo "   • База данных:   localhost:5432"
echo "   • Redis:         localhost:6379"
echo ""
echo "🛠️  Управление:"
echo "   Просмотр логов:    docker-compose logs -f"
echo "   Остановка:         docker-compose down"
echo "   Перезапуск:        docker-compose restart"
echo ""
echo "📋 Следующие шаги:"
echo "   1. Откройте http://localhost:3000"
echo "   2. Войдите с учетными данными администратора"
echo "   3. Настройте каналы связи"
echo "   4. Проверьте подключение к оркестратору"