#!/bin/bash

# Скрипт для деплоя Go приложения на сервер
set -euo pipefail

# Переменные (настройте под ваш сервер)
SERVER_USER="root"
SERVER_HOST="45.82.153.200"
APP_NAME="autoaccounter"
APP_DIR="/opt/$APP_NAME"
SERVICE_FILE="/etc/systemd/system/$APP_NAME.service"

# Опционально: пароль для SSH. Если задан SSH_PASS и установлен sshpass, используем его.
SSH_PASS_CMD=""
if [ -n "${SSH_PASS:-}" ] && command -v sshpass >/dev/null 2>&1; then
  SSH_PASS_CMD="sshpass -p ${SSH_PASS}"
fi

SCP_CMD="scp"
SSH_CMD="ssh"
if [ -n "$SSH_PASS_CMD" ]; then
  SCP_CMD="$SSH_PASS_CMD scp -o StrictHostKeyChecking=no"
  SSH_CMD="$SSH_PASS_CMD ssh -o StrictHostKeyChecking=no"
fi

echo "🚀 Начинаем деплой приложения $APP_NAME (Docker Compose)..."

# Формируем архив с compose-окружением и исходниками
echo "📁 Подготовка архива compose..."
tar --exclude "$APP_NAME.compose.tar.gz" -czf $APP_NAME.compose.tar.gz \
  Dockerfile \
  docker-compose.yml \
  nginx.conf \
  README.md \
  Makefile \
  .env \
  go.mod \
  go.sum \
  *.go \
  internal/ \
  migrations/ \
  docs/ \
  static/

echo "⬆️ Загрузка compose на сервер..."
$SCP_CMD $APP_NAME.compose.tar.gz $SERVER_USER@$SERVER_HOST:/opt/

echo "🔧 Разворачиваем compose на сервере..."
$SSH_CMD $SERVER_USER@$SERVER_HOST << 'EOF'
    set -euo pipefail
    APP_NAME="autoaccounter"
    APP_DIR="/opt/$APP_NAME"
    mkdir -p "${APP_DIR}"
    # Полная очистка каталога приложения, чтобы не осталось легаси-файлов
    rm -rf "${APP_DIR:?}/"*
    cd /opt
    tar -xzf $APP_NAME.compose.tar.gz -C "${APP_DIR}" || true

    # Остановим старый systemd-сервис (если был ранее)
    systemctl stop n8nuploader 2>/dev/null || true
    systemctl disable n8nuploader 2>/dev/null || true
    rm -f /etc/systemd/system/n8nuploader.service 2>/dev/null || true
    systemctl daemon-reload || true

    # Остановим и отключим системный nginx, чтобы освободить 80/443
    if systemctl list-unit-files | grep -q '^nginx.service'; then
        systemctl stop nginx 2>/dev/null || true
        systemctl disable nginx 2>/dev/null || true
    fi

    # Установка Docker и Docker Compose (если нет)
    if ! command -v docker >/dev/null 2>&1; then
        apt-get update -y
        apt-get install -y ca-certificates curl gnupg lsb-release
        install -m 0755 -d /etc/apt/keyrings
        curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
        chmod a+r /etc/apt/keyrings/docker.gpg
        echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu focal stable" > /etc/apt/sources.list.d/docker.list
        apt-get update -y
        apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
        systemctl enable docker --now
    fi

    cd "${APP_DIR}"
    # Остановим предыдущие контейнеры стека, если запущены
    docker compose down || true
    # На всякий случай удалим висячий контейнер nginx-proxy
    docker rm -f nginx-proxy 2>/dev/null || true
    # Уберём возможные конфликты имён контейнеров стека
    docker rm -f app-postgres 2>/dev/null || true
    docker rm -f autoaccounter 2>/dev/null || true
    docker rm -f app-pgadmin 2>/dev/null || true
    docker rm -f n8nuploader 2>/dev/null || true
    docker compose pull || true
    docker compose build --no-cache
    docker compose up -d

    echo "🩺 Проверка доступности приложения на 8080..."
    for i in {1..60}; do
        if curl -fsS http://127.0.0.1:8080/health >/dev/null 2>&1; then
            echo "✅ Приложение отвечает на /health"
            break
        fi
        echo "⏳ Ожидание ответа приложения... [$i/60]"
        sleep 2
        if [ "$i" -eq 60 ]; then
            echo "❌ Приложение не отвечает на /health. Последние логи контейнеров:"
            docker compose ps
            echo "---- app logs ----"
            docker compose logs --no-color --tail=200 || true
            exit 1
        fi
    done

    echo "⛏  Применяем миграции..."
    # Дождёмся реальной готовности Postgres через pg_isready (до ~120 сек)
    READY=0
    for i in {1..60}; do
        if docker exec app-postgres pg_isready -U appuser -d appdb >/dev/null 2>&1; then
            READY=1
            break
        fi
        STATUS=$(docker inspect -f '{{.State.Health.Status}}' app-postgres 2>/dev/null || echo "unknown")
        echo "⏳ Ожидание готовности Postgres... [$i/60] (health=$STATUS)"
        sleep 2
    done
    if [ "$READY" -ne 1 ]; then
        echo "❌ Postgres не готов. Прерываем деплой."
        exit 1
    fi
    # Регистр миграций (идемпотентно)
    docker exec -i app-postgres psql -U appuser -d appdb -v ON_ERROR_STOP=1 \
        -c "CREATE TABLE IF NOT EXISTS schema_migrations (filename text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now());"
    set -o pipefail
    # Накатываем новые миграции по имени файла (в алфавитном порядке)
    for f in $(find migrations -maxdepth 1 -type f -name '*.sql' | sort); do
        bn=\$(basename "\$f")
        applied=$(docker exec -i app-postgres psql -U appuser -d appdb -t -A -c "SELECT 1 FROM schema_migrations WHERE filename='${bn}'" || true)
        if [ "$applied" != "1" ]; then
            echo "➡️  Применяем миграцию: $bn"
            TMP_FILE=\$(mktemp)
            # Извлечём только блок '-- +goose Up' по '-- +goose Down'; если маркеров нет — применим весь файл
            awk 'BEGIN{p=0} /^-- \\+goose Up/{p=1; next} /^-- \\+goose Down/{p=0} {if(p) print}' "$f" > "\$TMP_FILE"
            if [ -s "\$TMP_FILE" ]; then
                cat "\$TMP_FILE" | docker exec -i app-postgres psql -U appuser -d appdb -v ON_ERROR_STOP=1
            else
                cat "$f" | docker exec -i app-postgres psql -U appuser -d appdb -v ON_ERROR_STOP=1
            fi
            rm -f "\$TMP_FILE"
            # Зафиксируем применение
            docker exec -i app-postgres psql -U appuser -d appdb -c "INSERT INTO schema_migrations(filename) VALUES ('${bn}');"
        else
            echo "✔️  Уже применена: $bn"
        fi
    done

    if command -v ufw >/dev/null 2>&1; then
        ufw allow 80/tcp || true
        ufw allow 443/tcp || true
        ufw allow 8081/tcp || true
        ufw allow 444/tcp || true
        ufw allow 8080/tcp || true
        ufw allow 5050/tcp || true
        ufw allow 5432/tcp || true
    fi
EOF

echo "✅ Деплой через Docker Compose завершен!" 