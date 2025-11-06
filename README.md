## Document AI

Короче, это сервис, который принимает файл + текст задачки, обрабатывает запрос через Qwen (OpenRouter), а ответы складывает обратно. Пока что без 1С, без боли, только то, что нужно.

### Что умеем
- PDF/JPG/PNG до 50 МБ — норм.
- Вызов Qwen через OpenRouter — результат публикуется в реальном времени через SSE.
- UI на ванильном JS — загрузка, статус, список ответов.

### Как запустить
```bash
go run .
# открой http://localhost:8080
```

Docker:
```bash
docker build -t document-ai .
docker run -it --rm -p 8080:8080 document-ai
```

Makefile (если лень помнить команды):
```bash
make build   # go build -o document-ai .
make test    # go test ./...
make run     # собрать и стартануть
```

### Конфиг (env'ы)
- `SERVER_PORT` — порт HTTP (дефолт 8080)
- `MAX_FILE_SIZE_MB` — лимит загрузки (дефолт 50)
- `MAX_RESPONSES` — сколько результатов держать в памяти (дефолт 20)
- `STATIC_DIR` — откуда отдаём фронт (дефолт `static`)
- `OPENROUTER_API_KEY` — API-ключ OpenRouter (обязательно)
- `OPENROUTER_BASE_URL` — базовый URL (дефолт `https://openrouter.ai/api/v1`)
- `QWEN_MODEL` — модель (дефолт `qwen/qwen3-235b-a22b-2507`)
- `SITE_URL`, `SITE_TITLE` — опциональные заголовки для рейтингов OpenRouter
- `DATABASE_URL` — строка подключения к Postgres
  - Compose: `postgres://appuser:apppass@postgres:5432/appdb?sslmode=disable`
  - Local:   `postgres://appuser:apppass@localhost:5432/appdb?sslmode=disable`
- `ADMIN_EMAIL`, `ADMIN_PASSWORD` — bootstrap-учётка для входа в админку
- `SESSION_SECRET` — секрет подписи сессий (замените на случайный)

Грузится в `config.go`. Логика в `handlers.go`. Ничего лишнего.

### Архитектура (ленивая, но аккуратная)
```
main.go           # точка входа: initEnvVariables -> setupRoutes -> startServer
config.go         # читаем env'ы, выставляем глобальные настройки
types.go          # структуры: запрос/ответ/модель результата
state.go          # простая память + мьютекс (ничего rocket science)
routes.go         # регистрируем эндпоинты
handlers.go       # прикладная логика и утилиты (Qwen/OpenRouter, JSON, валидация)
static/           # фронтенд (css/js/html)
```

### API (минимализм)
- `POST /upload`
  - form-data: `message` (string), `file` (pdf/jpg/png)
  - ответ: `{ status: "success", message: "..." }`

- `POST /webhook` (обратная совместимость)
  - остаётся для старых интеграций; новые запросы идут напрямую в Qwen

- `GET /results`
  - список последних результатов (с лимитом по `MAX_RESPONSES`)

- `GET /health`
  - просто говорит, что всё живо

### Сборка/деплой
Локально:
```bash
go build -ldflags="-s -w" -o document-ai .
./document-ai
```

Скрипт деплоя: `deploy.sh` — деплой через Docker Compose (приложение + Nginx + Postgres + PgAdmin).
Артефакты: `Dockerfile`, `docker-compose.yml`, `migrations/`.

После деплоя примените миграции (один раз):
```bash
goose -dir ./migrations postgres "postgres://appuser:apppass@45.82.153.200:5432/appdb?sslmode=disable" up
```

PgAdmin: `http://SERVER:5050/` (логин/пароль из docker-compose или .env).

### CI/CD
- GitHub Actions: тесты, сборка, отчёты. Главное — собираем весь пакет, а не один файл. Уже настроено.

### Примечания
- Admin UI находится по адресу `/static/admin/`.
- API админки: `/admin/login`, `/admin/logout`, `/admin/customers*`, `/admin/subscriptions/create`.
- Хотите стейт не в памяти — теперь Postgres. Таблицы создаются миграциями из `migrations/`.
- Логи: оставлены только предупреждения/ошибки. Если надо болтологию — добавьте уровень через env.
— если модель не отвечает — вернём ошибку. Ретраев нет, потому что не надо до тех пор, пока не надо.
