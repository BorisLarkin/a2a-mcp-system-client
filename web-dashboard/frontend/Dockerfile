# ./web-dashboard/frontend/Dockerfile
FROM node:20-alpine AS builder

WORKDIR /app

# Копируем package.json
COPY package*.json ./

# Очищаем кеш npm и устанавливаем зависимости
RUN npm cache clean --force && \
    npm install

# Копируем конфигурационные файлы
COPY vite.config.js ./
COPY index.html ./

# Копируем исходный код
COPY src ./src
COPY public ./public

# Собираем приложение с дополнительной информацией
RUN echo "Building application..." && \
    npm run build && \
    echo "Build complete. Contents of /app:" && \
    ls -la /app && \
    echo "Contents of /app/dist:" && \
    ls -la /app/dist || echo "No dist directory"

# Этап 2: Сервер
FROM nginx:alpine

# Удаляем дефолтные файлы
RUN rm -rf /usr/share/nginx/html/*

# Копируем собранные файлы
COPY --from=builder /app/dist /usr/share/nginx/html

# Создаем конфигурацию nginx
RUN echo 'server { \
    listen 80; \
    root /usr/share/nginx/html; \
    index index.html; \
    location / { \
        try_files $uri $uri/ /index.html; \
    } \
    location /api/ { \
        proxy_pass http://local-proxy:8080; \
        proxy_http_version 1.1; \
        proxy_set_header Upgrade $http_upgrade; \
        proxy_set_header Connection "upgrade"; \
        proxy_set_header Host $host; \
    } \
}' > /etc/nginx/conf.d/default.conf

# Проверяем что файлы скопировались
RUN echo "Contents of nginx html directory:" && \
    ls -la /usr/share/nginx/html/

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]