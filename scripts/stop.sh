#!/bin/bash
# ./scripts/stop.sh

echo "🛑 Остановка клиентской части..."
docker-compose down

echo "✅ Все сервисы остановлены"