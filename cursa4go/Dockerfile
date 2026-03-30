FROM golang:1.23-alpine AS builder

WORKDIR /app

# Копируем файлы go.mod и go.sum
COPY go.mod go.sum ./

# Скачиваем зависимости
RUN go mod download

# Копируем весь проект
COPY . .

# ✅ Обновляем зависимости и собираем
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o main .

# Финальный образ
FROM alpine:latest

WORKDIR /app

# Копируем бинарник
COPY --from=builder /app/main .

# Копируем шаблоны и статику
COPY templates ./templates
COPY static ./static

# 🟢 ИСПРАВЛЕНО: Был 8080, теперь 5000
EXPOSE 5000

CMD ["./main"]
 #проверяю конкретно на этом файде