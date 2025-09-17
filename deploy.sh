#!/bin/bash

# Скрипт для деплоя Go приложения на сервер

# Переменные (настройте под ваш сервер)
SERVER_USER="root"
SERVER_HOST="45.82.153.200"
APP_NAME="n8nuploader"
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

echo "🚀 Начинаем деплой приложения $APP_NAME..."

# 1. Собираем приложение для Linux
echo "📦 Сборка приложения..."
GOOS=linux GOARCH=amd64 go build -o $APP_NAME .

# 2. Создаем архив с приложением
echo "📁 Создание архива..."
tar -czf $APP_NAME.tar.gz $APP_NAME static/ README.md .env

# 3. Копируем на сервер (+ unit-файл)
echo "⬆️ Загрузка на сервер..."
$SCP_CMD $APP_NAME.tar.gz $SERVER_USER@$SERVER_HOST:/tmp/
$SCP_CMD n8nuploader.service $SERVER_USER@$SERVER_HOST:/tmp/
$SCP_CMD nginx-simple.conf $SERVER_USER@$SERVER_HOST:/tmp/

# 4. Разворачиваем на сервере
echo "🔧 Установка на сервере..."
$SSH_CMD $SERVER_USER@$SERVER_HOST << EOF
    # Останавливаем старую версию (если есть)
    sudo systemctl stop $APP_NAME || true
    
    # Создаем директорию приложения
    sudo mkdir -p $APP_DIR
    
    # Извлекаем архив
    cd /tmp
    tar -xzf $APP_NAME.tar.gz
    
    # Копируем файлы
    sudo cp -r $APP_NAME static/ README.md .env $APP_DIR/
    # Права для пользователя сервиса www-data
    sudo chown -R www-data:www-data $APP_DIR
    sudo chmod +x $APP_DIR/$APP_NAME
    
    # Устанавливаем unit-файл systemd
    sudo mv /tmp/n8nuploader.service $SERVICE_FILE
    sudo systemctl daemon-reload
    sudo systemctl enable $APP_NAME || true
    # Перезапускаем сервис
    sudo systemctl restart $APP_NAME || sudo systemctl start $APP_NAME
    sudo systemctl status $APP_NAME --no-pager || true

    echo "== Установка/настройка nginx (HTTP) =="
    if ! command -v nginx >/dev/null 2>&1; then
        apt-get update -y && apt-get install -y nginx
    fi
    # Размещаем простой HTTP-конфиг
    sudo mv /tmp/nginx-simple.conf /etc/nginx/conf.d/app.conf
    # Отключим дефолт, чтобы не конфликтовал (опционально)
    if [ -f /etc/nginx/sites-enabled/default ]; then sudo rm -f /etc/nginx/sites-enabled/default; fi
    nginx -t && systemctl reload nginx || systemctl restart nginx || true
    # Разрешим HTTP в firewall (если ufw включен)
    if command -v ufw >/dev/null 2>&1; then
        ufw allow 80/tcp || true
        ufw allow 8080/tcp || true
    fi
    
    # Очищаем временные файлы
    rm -f /tmp/$APP_NAME.tar.gz
EOF

echo "✅ Деплой завершен!"
echo "🔗 Сервис перезапущен через systemd. При необходимости настройте Nginx."

# Очищаем локальные временные файлы
rm -f $APP_NAME $APP_NAME.tar.gz 