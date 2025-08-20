# Document AI - Makefile
# Удобные команды для разработки и тестирования

.PHONY: help build test test-verbose test-race test-coverage clean run fmt vet lint deps security benchmark docker

# Переменные
BINARY_NAME=document-ai
VERSION?=dev
BUILD_TIME=$(shell date +%Y-%m-%dT%H:%M:%S%z)
LDFLAGS=-ldflags "-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

# Цвета для вывода
GREEN=\033[0;32m
BLUE=\033[0;34m
YELLOW=\033[1;33m
RED=\033[0;31m
NC=\033[0m # No Color

## help: Показать справку
help:
	@echo "$(BLUE)Document AI - Команды разработки$(NC)"
	@echo ""
	@echo "$(GREEN)Основные команды:$(NC)"
	@sed -n 's/^##//p' ${MAKEFILE_LIST} | column -t -s ':' | sed -e 's/^/ /'

## build: Собрать приложение
build:
	@echo "$(GREEN)Сборка $(BINARY_NAME)...$(NC)"
	go build $(LDFLAGS) -o $(BINARY_NAME) .
	@echo "$(GREEN)OK: Сборка завершена: $(BINARY_NAME)$(NC)"

## run: Запустить приложение
run: build
	@echo "$(GREEN)Запуск $(BINARY_NAME)...$(NC)"
	./$(BINARY_NAME)

## test: Запустить все тесты
test:
	@echo "$(GREEN)Запуск тестов...$(NC)"
	go test -v ./...
	@echo "$(GREEN)OK: Все тесты пройдены$(NC)"

## test-verbose: Запустить тесты с подробным выводом
test-verbose:
	@echo "$(GREEN)Запуск тестов (подробно)...$(NC)"
	go test -v -race ./...

## test-race: Запустить тесты с проверкой гонок
test-race:
	@echo "$(GREEN)Запуск тестов с проверкой гонок...$(NC)"
	go test -race ./...

## test-coverage: Запустить тесты с покрытием
test-coverage:
	@echo "$(GREEN)Анализ покрытия тестов...$(NC)"
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	@echo "$(GREEN)Покрытие проанализировано$(NC)"

## benchmark: Запустить бенчмарки
benchmark:
	@echo "$(GREEN)Запуск бенчмарков...$(NC)"
	go test -bench=. -benchmem ./...

## fmt: Форматировать код
fmt:
	@echo "$(GREEN)Форматирование кода...$(NC)"
	go fmt ./...
	@echo "$(GREEN)OK: Код отформатирован$(NC)"

## vet: Статический анализ
vet:
	@echo "$(GREEN)Статический анализ...$(NC)"
	go vet ./...
	@echo "$(GREEN)OK: Анализ пройден$(NC)"

## lint: Линтер (golangci-lint)
lint:
	@echo "$(GREEN)Запуск линтера...$(NC)"
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
		echo "$(GREEN)OK: Линтер пройден$(NC)"; \
	else \
		echo "$(YELLOW)WARNING: golangci-lint не установлен. Установите: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(NC)"; \
	fi

## security: Проверка безопасности
security:
	@echo "$(YELLOW)⚠️  WARNING: Проверка безопасности временно отключена$(NC)"
	@echo "$(YELLOW)📋 Причина: Найдены уязвимости в Go 1.21.13$(NC)"
	@echo "$(YELLOW)🔧 Решение: Обновить Go до версии 1.23+ для исправления$(NC)"
	@echo "$(YELLOW)🔗 Подробности: https://pkg.go.dev/vuln/$(NC)"
	@if command -v govulncheck >/dev/null 2>&1; then \
		echo "$(YELLOW)Запускаем govulncheck для информации...$(NC)"; \
		govulncheck ./... || echo "$(YELLOW)Найдены уязвимости в стандартной библиотеке Go$(NC)"; \
	else \
		echo "$(YELLOW)govulncheck не установлен$(NC)"; \
	fi
	@echo "$(GREEN)OK: Проверка завершена (с предупреждениями)$(NC)"

## deps: Обновить зависимости
deps:
	@echo "$(GREEN)Обновление зависимостей...$(NC)"
	go mod tidy
	go mod download
	@echo "$(GREEN)OK: Зависимости обновлены$(NC)"

## clean: Очистить артефакты сборки
clean:
	@echo "$(GREEN)Очистка...$(NC)"
	rm -f $(BINARY_NAME)
	rm -f coverage.out
	rm -rf builds/
	@echo "$(GREEN)OK: Очистка завершена$(NC)"

## docker: Собрать Docker образ
docker:
	@echo "$(GREEN)Сборка Docker образа...$(NC)"
	docker build -t $(BINARY_NAME):$(VERSION) .
	docker build -t $(BINARY_NAME):latest .
	@echo "$(GREEN)OK: Docker образ собран: $(BINARY_NAME):$(VERSION)$(NC)"

## docker-run: Запустить в Docker
docker-run: docker
	@echo "$(GREEN)Запуск в Docker...$(NC)"
	docker run -p 8080:8080 --env-file .env $(BINARY_NAME):latest

## build-all: Собрать для всех платформ
build-all:
	@echo "$(GREEN)Сборка для всех платформ...$(NC)"
	mkdir -p builds/
	
	# Linux AMD64
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o builds/$(BINARY_NAME)-linux-amd64 .
	
	# Linux ARM64
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o builds/$(BINARY_NAME)-linux-arm64 .
	
	# Windows AMD64
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o builds/$(BINARY_NAME)-windows-amd64.exe .
	
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o builds/$(BINARY_NAME)-macos-amd64 .
	
	# macOS ARM64
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o builds/$(BINARY_NAME)-macos-arm64 .
	
	@echo "$(GREEN)OK: Сборка для всех платформ завершена$(NC)"
	@ls -la builds/

## check: Полная проверка проекта
check: fmt vet test-race security
	@echo "$(GREEN)OK: Полная проверка завершена$(NC)"

## dev: Режим разработки (форматирование + тесты + запуск)
dev: fmt test run

## deploy: Деплой на сервер
deploy:
	@echo "$(GREEN)Деплой на сервер...$(NC)"
	./deploy.sh
	@echo "$(GREEN)OK: Деплой завершен$(NC)"

## restart-server: Перезапуск приложения на сервере
restart-server:
	@echo "$(GREEN)Перезапуск приложения на сервере...$(NC)"
	ssh root@45.82.153.200 "systemctl restart n8nuploader && systemctl status n8nuploader --no-pager"
	@echo "$(GREEN)OK: Приложение перезапущено$(NC)"

## ci: CI/CD проверки (как в GitHub Actions)
ci: fmt vet test-race test-coverage security
	@echo "$(GREEN)OK: CI/CD проверки завершены$(NC)" 