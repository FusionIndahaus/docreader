# 🤖 Document AI - Smart Document Processing

Современное веб-приложение для интеллектуальной обработки документов с помощью искусственного интеллекта. Загружайте PDF файлы или изображения, задавайте вопросы о содержании и получайте детальный анализ.

## ✨ Возможности

- 📄 **Обработка документов**: PDF, JPG, PNG до 50 МБ
- 🤖 **AI-анализ**: Извлечение данных, анализ содержания, ответы на вопросы
- 🎨 **Современный UI**: Адаптивный дизайн с drag&drop загрузкой
- ⚡ **Быстрая обработка**: Результаты через 1-3 минуты
- 🔄 **Автообновление**: Результаты обновляются автоматически
- 🌐 **API интеграция**: Работает с n8n workflows

## 🚀 Быстрый старт

### Локальный запуск

```bash
# Клонируем репозиторий
git clone https://github.com/FusionIndahaus/docreader.git
cd document-ai

# Запускаем приложение
go run .

# Открываем браузер
open http://localhost:8080
```

### Конфигурация

Отредактируйте константы в `main.go`:

```go
const (
    // URL вашего n8n webhook
    n8nWebhookURL = "https://your-n8n.instance/webhook/your-id"
    serverPort    = "8080"
    maxFileSize   = 50 << 20 // 50MB
)
```

## 🔧 API Endpoints

### `POST /upload`
Загрузка документа на обработку

**Параметры:**
- `message` (string) - Описание задачи
- `file` (multipart) - Файл документа

**Ответ:**
```json
{
  "status": "success",
  "message": "Документ отправлен на обработку"
}
```

### `GET /results`
Получение результатов обработки

**Ответ:**
```json
{
  "status": "success",
  "data": [
    {
      "id": "res_1234567890",
      "text": "Результат анализа...",
      "timestamp": "2024-01-15T10:30:00Z",
      "status": "completed"
    }
  ]
}
```

### `POST /webhook`
Webhook для получения результатов от n8n

### `GET /health`
Проверка состояния сервиса

## Деплой на сервер

### Простой способ (скрипт)

```bash
# Собираем для Linux
GOOS=linux GOARCH=amd64 go build -o document-ai .

# Загружаем на сервер
scp document-ai static/ user@server:/opt/document-ai/

# Запускаем
ssh user@server "cd /opt/document-ai && ./document-ai"
```

### С Docker

```bash
# Собираем образ
docker build -t document-ai .

# Запускаем
docker run -d -p 8080:8080 document-ai
```

### С GitHub Actions

Проект включает готовую настройку CI/CD для автоматического деплоя при пуше в main ветку.

## 🔗 Интеграция с n8n

1. Создайте workflow в n8n
2. Добавьте HTTP Request node с вашим endpoint
3. Настройте обработку файлов и текста
4. Отправляйте результаты обратно через webhook:

```json
{
  "text": "Ваш результат анализа",
  "status": "completed"
}
```

## 🛠️ Технологический стек

- **Backend**: Go 1.21, стандартная библиотека
- **Frontend**: Vanilla JavaScript, современный CSS
- **Интеграция**: n8n workflows, REST API
- **Деплой**: Docker, systemd, Nginx
