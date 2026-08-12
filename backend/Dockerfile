# Используем официальный образ Go для сборки
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Копируем файлы с зависимостями
COPY go.mod go.sum ./
RUN go mod download

# Копируем остальной код и собираем приложение
COPY . .
RUN go build -o main .

# Используем минимальный образ для запуска
FROM alpine:latest

WORKDIR /app

# Копируем собранный бинарный файл из предыдущего этапа
COPY --from=builder /app/main .

# Объявляем порт, который будет слушать приложение
EXPOSE 10000

# Запускаем приложение
CMD ["./main"]