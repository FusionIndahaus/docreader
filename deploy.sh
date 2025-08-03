#!/bin/bash

# Скрипт для деплоя Go приложения на сервер

# Переменные (настройте под ваш сервер)
SERVER_USER="root"
SERVER_HOST="45.82.153.200"
APP_NAME="n8nuploader"
APP_DIR="/opt/$APP_NAME"

echo "🚀 Начинаем деплой приложения $APP_NAME..."

# 1. Собираем приложение для Linux
echo "📦 Сборка приложения..."
GOOS=linux GOARCH=amd64 go build -o $APP_NAME main.go

# 2. Создаем архив с приложением
echo "📁 Создание архива..."
tar -czf $APP_NAME.tar.gz $APP_NAME static/ README.md

# 3. Копируем на сервер
echo "⬆️ Загрузка на сервер..."
scp $APP_NAME.tar.gz $SERVER_USER@$SERVER_HOST:/tmp/

# 4. Разворачиваем на сервере
echo "🔧 Установка на сервере..."
ssh $SERVER_USER@$SERVER_HOST << EOF
    # Останавливаем старую версию (если есть)
    sudo systemctl stop $APP_NAME || true
    
    # Создаем директорию приложения
    sudo mkdir -p $APP_DIR
    
    # Извлекаем архив
    cd /tmp
    tar -xzf $APP_NAME.tar.gz
    
    # Копируем файлы
    sudo cp -r $APP_NAME static/ README.md $APP_DIR/
    sudo chown -R $SERVER_USER:$SERVER_USER $APP_DIR
    sudo chmod +x $APP_DIR/$APP_NAME
    
    # Перезапускаем сервис
    sudo systemctl start $APP_NAME
    sudo systemctl status $APP_NAME --no-pager
    
    # Очищаем временные файлы
    rm -f /tmp/$APP_NAME.tar.gz
EOF

echo "✅ Деплой завершен!"
echo "🔗 Настройте systemd сервис и nginx на сервере"

# Очищаем локальные временные файлы
rm -f $APP_NAME $APP_NAME.tar.gz 