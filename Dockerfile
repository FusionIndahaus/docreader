FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . ./

# Сборка
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -a -installsuffix cgo -o main .

# Используем минимальный образ для production
FROM alpine:latest

RUN apk --no-cache add \
    ca-certificates \
    poppler-utils \
    tesseract-ocr \
    tesseract-ocr-data-eng \
    tesseract-ocr-data-rus \
    wget

RUN addgroup -g 1001 appgroup && adduser -D -u 1001 -G appgroup appuser

WORKDIR /app

COPY --from=builder /app/main .

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