FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . ./

# Сборка
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o main .

# Используем минимальный образ для production
FROM alpine:3.20

RUN apk --no-cache add \
    ca-certificates \
    poppler-utils \
    tesseract-ocr \
    tesseract-ocr-data-eng \
    tesseract-ocr-data-rus \
    wget && update-ca-certificates

RUN addgroup -g 1001 appgroup && adduser -D -u 1001 -G appgroup appuser

WORKDIR /app

ENV OCR_LANGS=rus+eng \
    UPLOAD_DIR=/tmp/document-ai/uploads

COPY --from=builder /app/main .

COPY static/ ./static/

# Подготовка runtime-директорий и прав
RUN mkdir -p /app/logs "${UPLOAD_DIR}" && chown -R appuser:appgroup /app "${UPLOAD_DIR}"

# Переключаемся на непривилегированного пользователя
USER appuser

# Открываем порт
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://127.0.0.1:8080/health || exit 1

# Запускаем приложение
CMD ["./main"] 