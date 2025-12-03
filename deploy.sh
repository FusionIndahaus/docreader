#!/bin/bash

# Скрипт для деплоя Go приложения на сервер

# Переменные (настройте под ваш сервер)
SERVER_USER="root"
SERVER_HOST="45.82.153.200"
APP_NAME="autoaccounter"
APP_DIR="/opt/$APP_NAME"
SERVICE_FILE="/etc/systemd/system/$APP_NAME.service"

# Опционально: пароль для SSH. Если задан SSH_PASS и установлен sshpass, используем его.
SSH_PASS_CMD=""
if [ -n "$SSH_PASS" ] && command -v sshpass >/dev/null 2>&1; then
  SSH_PASS_CMD="sshpass -p $SSH_PASS"
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
tar -czf $APP_NAME.compose.tar.gz \
  Dockerfile \
  docker-compose.yml \
  nginx.conf \
  README.md \
  Makefile \
  .env \
  go.mod \
  go.sum \
  *.go \
  migrations/ \
  docs/ \
  static/

echo "⬆️ Загрузка compose на сервер..."
$SCP_CMD $APP_NAME.compose.tar.gz $SERVER_USER@$SERVER_HOST:/opt/

echo "🔧 Разворачиваем compose на сервере..."
$SSH_CMD $SERVER_USER@$SERVER_HOST << 'EOF'
    set -e
    APP_NAME="autoaccounter"
    APP_DIR="/opt/$APP_NAME"
    mkdir -p "${APP_DIR}"
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
    docker compose pull || true
    docker compose build --no-cache
    docker compose up -d

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