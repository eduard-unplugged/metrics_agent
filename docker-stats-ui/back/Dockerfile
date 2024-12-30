# ==========================
# 1) Стадия сборки (builder)
# ==========================
FROM golang:1.23-alpine AS builder

WORKDIR /app

# Копируем файлы go.mod/go.sum, чтобы сначала скачать зависимости
COPY . ./
RUN go mod download

# Копируем весь код бэкенда
COPY . .

# Собираем Go-приложение
RUN go build -o /main

# ========================
# 2) Стадия запуск (final)
# ========================
FROM alpine:3.18

# Копируем бинарник из builder-стадии
COPY --from=builder /main /usr/local/bin/app

# Открываем порт 3000 (где слушает наш бэкенд)
EXPOSE 3000

# По умолчанию запускаем
CMD ["app"]