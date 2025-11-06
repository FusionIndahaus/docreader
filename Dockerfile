# Используем официальный образ Go для сборки
FROM golang:1.21-alpine AS builder

# Устанавливаем рабочую директорию
WORKDIR /app

# Копируем go.mod и go.sum (если есть)
COPY go.mod go.sum ./

# Загружаем зависимости
RUN go mod download

# Копируем исходный код целиком
COPY . ./

# Собираем приложение
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -a -installsuffix cgo -o main .

# Используем минимальный образ для production
FROM alpine:latest

# Устанавливаем CA сертификаты и утилиты для извлечения текста/OCR
# poppler-utils -> pdftotext; tesseract-ocr + языковые пакеты
RUN apk --no-cache add \
    ca-certificates \
    poppler-utils \
    tesseract-ocr \
    tesseract-ocr-data-eng \
    tesseract-ocr-data-rus

# Создаем непривилегированного пользователя
RUN addgroup -g 1001 appgroup && adduser -D -u 1001 -G appgroup appuser

# Создаем рабочую директорию
WORKDIR /app

# Копируем скомпилированное приложение из builder стадии
COPY --from=builder /app/main .

# Копируем статические файлы
COPY static/ ./static/

# Меняем владельца файлов
RUN chown -R appuser:appgroup /app

# Переключаемся на непривилегированного пользователя
USER appuser

# Значения по умолчанию для OCR языков
ENV OCR_LANGS=rus+eng

# Открываем порт
EXPOSE 8080

# Запускаем приложение
CMD ["./main"] 