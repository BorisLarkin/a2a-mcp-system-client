# ./web-dashboard/Dockerfile
# Этап 1: Сборка
FROM node:18-alpine AS builder

WORKDIR /app

# Копируем файлы зависимостей
COPY package*.json ./
COPY yarn.lock* ./

# Устанавливаем зависимости
RUN npm ci

# Копируем исходный код
COPY . .

# Собираем приложение
RUN npm run build

# Этап 2: Сервер
FROM nginx:alpine

# Копируем собранные файлы из этапа сборки
COPY --from=builder /app/dist /usr/share/nginx/html
# ИЛИ для create-react-app:
# COPY --from=builder /app/build /usr/share/nginx/html

# Копируем конфигурацию nginx
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Порт
EXPOSE 80

# Запуск nginx
CMD ["nginx", "-g", "daemon off;"]