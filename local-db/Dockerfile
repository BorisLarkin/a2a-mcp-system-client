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

# Копирование дополнительных скриптов (если есть)
#COPY migrations/*.sql /docker-entrypoint-initdb.d/migrations/

# Копирование конфигурации PostgreSQL
COPY postgresql.conf /etc/postgresql/postgresql.conf

# Настройка прав
RUN chown -R postgres:postgres /docker-entrypoint-initdb.d \
    && chmod 644 /docker-entrypoint-initdb.d/init.sql

# Оптимизация PostgreSQL для разработки
RUN echo "shared_buffers = 256MB" >> /etc/postgresql/postgresql.conf \
    && echo "effective_cache_size = 768MB" >> /etc/postgresql/postgresql.conf \
    && echo "maintenance_work_mem = 64MB" >> /etc/postgresql/postgresql.conf \
    && echo "checkpoint_completion_target = 0.9" >> /etc/postgresql/postgresql.conf \
    && echo "wal_buffers = 16MB" >> /etc/postgresql/postgresql.conf \
    && echo "default_statistics_target = 100" >> /etc/postgresql/postgresql.conf \
    && echo "random_page_cost = 1.1" >> /etc/postgresql/postgresql.conf \
    && echo "effective_io_concurrency = 200" >> /etc/postgresql/postgresql.conf \
    && echo "work_mem = 16MB" >> /etc/postgresql/postgresql.conf \
    && echo "min_wal_size = 1GB" >> /etc/postgresql/postgresql.conf \
    && echo "max_wal_size = 4GB" >> /etc/postgresql/postgresql.conf \
    && echo "max_connections = 200" >> /etc/postgresql/postgresql.conf \
    && echo "listen_addresses = '*'" >> /etc/postgresql/postgresql.conf

# Добавление расширений для полнотекстового поиска
RUN echo "shared_preload_libraries = 'pg_stat_statements, pgvecto.rs'" >> /etc/postgresql/postgresql.conf

# Порт PostgreSQL
EXPOSE 5432

# Переключение на пользователя postgres
USER postgres

# Запуск PostgreSQL с кастомной конфигурацией
CMD ["postgres", "-c", "config_file=/etc/postgresql/postgresql.conf"]