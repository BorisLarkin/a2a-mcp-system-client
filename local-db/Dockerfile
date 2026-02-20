# ~/git/a2a-mcp-system-client/local-db/Dockerfile
FROM postgres:15-alpine

# Копируем init.sql
COPY init.sql /docker-entrypoint-initdb.d/

# Копируем конфигурацию аутентификации
COPY config/pg_hba.conf /etc/postgresql/pg_hba.conf

# Копируем основную конфигурацию (если есть)
COPY postgresql.conf /etc/postgresql/postgresql.conf

# Устанавливаем правильные права
RUN chmod 644 /etc/postgresql/pg_hba.conf

# Команда запуска с указанием конфигурации
CMD ["postgres", "-c", "config_file=/etc/postgresql/postgresql.conf", "-c", "hba_file=/etc/postgresql/pg_hba.conf"]