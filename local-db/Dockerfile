# ./local-db/Dockerfile
FROM postgres:15-alpine

# Метаданные
LABEL maintainer="Your Company <support@yourcompany.com>"
LABEL description="PostgreSQL database for MCP/A2A client dispatcher system"
LABEL version="1.0.0"

# Создание директории для init скриптов
WORKDIR /docker-entrypoint-initdb.d

# Копирование init скрипта
COPY init.sql /docker-entrypoint-initdb.d/

# Копирование конфигурации PostgreSQL
COPY postgresql.conf /etc/postgresql/postgresql.conf

# Настройка прав
RUN chown -R postgres:postgres /docker-entrypoint-initdb.d \
    && chmod 644 /docker-entrypoint-initdb.d/init.sql \
    && chmod 644 /etc/postgresql/postgresql.conf

# Порт PostgreSQL
EXPOSE 5432

# Переключение на пользователя postgres
USER postgres

# Запуск PostgreSQL с кастомной конфигурацией
CMD ["postgres", "-c", "config_file=/etc/postgresql/postgresql.conf"]