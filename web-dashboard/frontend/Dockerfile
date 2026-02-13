# ./web-dashboard/frontend/Dockerfile
FROM node:20-alpine AS builder

WORKDIR /app

# Копируем package.json и package-lock.json (если есть)
COPY package*.json ./

# Если нет package-lock.json, используем npm install вместо npm ci
RUN if [ -f package-lock.json ]; then \
        npm ci; \
    else \
        npm install; \
    fi

# Копируем исходный код
COPY . .

# Собираем приложение
RUN npm run build

# Этап 2: Сервер
FROM nginx:alpine

# Копируем собранные файлы
COPY --from=builder /app/dist /usr/share/nginx/html
# ИЛИ для create-react-app:
# COPY --from=builder /app/build /usr/share/nginx/html

# Копируем конфигурацию nginx
COPY nginx.conf /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["nginx", "-g", "daemon off;"]