#!/bin/bash
# ./scripts/backup.sh

BACKUP_DIR="./backups"
DATE=$(date +%Y%m%d_%H%M%S)

mkdir -p $BACKUP_DIR

echo "💾 Создание резервной копии базы данных..."
docker-compose exec local-db pg_dump -U dispatcher_user dispatcher_db > "$BACKUP_DIR/dispatcher_db_$DATE.sql"

echo "📦 Архивирование резервных копий..."
tar -czf "$BACKUP_DIR/full_backup_$DATE.tar.gz" \
    ./local-db/init.sql \
    "$BACKUP_DIR/dispatcher_db_$DATE.sql" \
    .env \
    docker-compose.yml

echo "✅ Резервная копия создана: $BACKUP_DIR/full_backup_$DATE.tar.gz"