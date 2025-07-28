package onec

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
)

// OneCService сервис для работы с 1С
type OneCService struct {
	client  *OneCClient
	config  *IntegrationConfig
	enabled bool
}

// IntegrationConfig расширенная конфигурация интеграции
type IntegrationConfig struct {
	OneCConfig
	Enabled  bool                          `json:"enabled"`
	AutoSend bool                          `json:"auto_send"`
	Mapping  map[string]DocumentTypeConfig `json:"mapping"`
}

// DocumentTypeConfig конфигурация для конкретного типа документа
type DocumentTypeConfig struct {
	TargetObject string            `json:"target_object"`
	Fields       map[string]string `json:"fields"`
}

// IntegrationResult результат интеграции
type IntegrationResult struct {
	Success      bool   `json:"success"`
	DocumentID   string `json:"document_id"`
	OneCRef      string `json:"onec_ref,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// NewOneCService создает новый сервис интеграции с 1С
func NewOneCService(configPath string) (*OneCService, error) {
	// Читаем конфигурацию
	config, err := loadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("ошибка загрузки конфигурации: %w", err)
	}

	// Создаем клиент если интеграция включена
	var client *OneCClient
	if config.Enabled {
		client = NewOneCClient(config.OneCConfig)

		// Проверяем соединение
		if err := client.TestConnection(); err != nil {
			log.Printf("WARNING: Не удалось подключиться к 1С: %v", err)
			// Не возвращаем ошибку, просто отключаем интеграцию
			config.Enabled = false
		} else {
			log.Println("INFO: Подключение к 1С успешно установлено")
		}
	}

	return &OneCService{
		client:  client,
		config:  config,
		enabled: config.Enabled,
	}, nil
}

// ProcessN8nResponse обрабатывает ответ от n8n и отправляет в 1С
func (s *OneCService) ProcessN8nResponse(n8nData map[string]interface{}) (*IntegrationResult, error) {
	result := &IntegrationResult{
		Success: false,
	}

	// Если интеграция отключена
	if !s.enabled {
		log.Println("INFO: Интеграция с 1С отключена, пропускаем отправку")
		result.Success = true
		result.ErrorMessage = "Интеграция отключена"
		return result, nil
	}

	// Парсим данные от n8n
	docData := ParseN8nResponse(n8nData)
	result.DocumentID = docData.ID

	// Проверяем есть ли маппинг для данного типа документа
	if typeConfig, exists := s.config.Mapping[docData.DocumentType]; exists {
		// Применяем маппинг полей
		docData = s.applyFieldMapping(docData, typeConfig)
		log.Printf("INFO: Применен маппинг для типа документа: %s", docData.DocumentType)
	} else {
		log.Printf("WARNING: Маппинг для типа документа '%s' не найден, используем данные как есть", docData.DocumentType)
	}

	// Если автоотправка отключена, просто сохраняем данные
	if !s.config.AutoSend {
		log.Printf("INFO: Данные подготовлены для отправки в 1С, но автоотправка отключена")
		result.Success = true
		result.ErrorMessage = "Данные подготовлены, автоотправка отключена"
		return result, nil
	}

	// Отправляем в 1С
	response, err := s.client.SendDocument(docData)
	if err != nil {
		log.Printf("ERROR: Ошибка отправки в 1С: %v", err)
		result.ErrorMessage = err.Error()
		return result, err
	}

	// Обрабатываем ответ от 1С
	if response.Success {
		log.Printf("INFO: Документ успешно создан в 1С: %s", response.Reference)
		result.Success = true
		result.OneCRef = response.Reference
	} else {
		log.Printf("ERROR: 1С отклонила документ: %s", response.Error)
		result.ErrorMessage = response.Error
	}

	return result, nil
}

// applyFieldMapping применяет маппинг полей согласно конфигурации
func (s *OneCService) applyFieldMapping(docData DocumentData, typeConfig DocumentTypeConfig) DocumentData {
	mappedFields := make(map[string]interface{})

	// Применяем маппинг полей
	for sourceField, targetField := range typeConfig.Fields {
		if value, exists := docData.Fields[sourceField]; exists {
			mappedFields[targetField] = value
			log.Printf("DEBUG: Маппинг: %s -> %s = %v", sourceField, targetField, value)
		}
	}

	// Добавляем немаппированные поля
	for key, value := range docData.Fields {
		if _, mapped := typeConfig.Fields[key]; !mapped {
			mappedFields[key] = value
		}
	}

	// Добавляем информацию о целевом объекте 1С
	mappedFields["_target_object"] = typeConfig.TargetObject

	docData.Fields = mappedFields
	return docData
}

// SendManually ручная отправка документа в 1С
func (s *OneCService) SendManually(documentID string, n8nData map[string]interface{}) (*IntegrationResult, error) {
	if !s.enabled {
		return &IntegrationResult{
			Success:      false,
			DocumentID:   documentID,
			ErrorMessage: "Интеграция с 1С отключена",
		}, fmt.Errorf("интеграция отключена")
	}

	// Временно включаем автоотправку для ручной отправки
	originalAutoSend := s.config.AutoSend
	s.config.AutoSend = true
	defer func() {
		s.config.AutoSend = originalAutoSend
	}()

	return s.ProcessN8nResponse(n8nData)
}

// IsEnabled проверяет включена ли интеграция
func (s *OneCService) IsEnabled() bool {
	return s.enabled
}

// GetStatus возвращает статус интеграции
func (s *OneCService) GetStatus() map[string]interface{} {
	status := map[string]interface{}{
		"enabled":   s.enabled,
		"auto_send": s.config.AutoSend,
	}

	if s.enabled && s.client != nil {
		if err := s.client.TestConnection(); err != nil {
			status["connection"] = "error"
			status["error"] = err.Error()
		} else {
			status["connection"] = "ok"
		}
	} else {
		status["connection"] = "disabled"
	}

	return status
}

// loadConfig загружает конфигурацию из файла и переменных окружения
func loadConfig(configPath string) (*IntegrationConfig, error) {
	var config IntegrationConfig

	// Сначала пытаемся загрузить из файла
	if _, err := os.Stat(configPath); !os.IsNotExist(err) {
		data, err := os.ReadFile(configPath)
		if err != nil {
			log.Printf("WARNING: Не удалось прочитать файл конфигурации %s: %v", configPath, err)
		} else {
			if err := json.Unmarshal(data, &config); err != nil {
				log.Printf("WARNING: Ошибка парсинга конфигурации %s: %v", configPath, err)
			} else {
				log.Printf("INFO: Конфигурация загружена из файла: %s", configPath)
			}
		}
	} else {
		log.Printf("INFO: Файл конфигурации %s не найден", configPath)
	}

	// Переопределяем настройки из переменных окружения (приоритет)
	overrideFromEnv(&config)

	// Если базовые настройки не установлены, используем значения по умолчанию
	setDefaults(&config)

	return &config, nil
}

// overrideFromEnv переопределяет настройки из переменных окружения
func overrideFromEnv(config *IntegrationConfig) {
	// Основные настройки 1С
	if baseURL := os.Getenv("ONEC_BASE_URL"); baseURL != "" {
		config.BaseURL = baseURL
		log.Printf("INFO: ONEC_BASE_URL установлен из ENV: %s", baseURL)
	}

	if username := os.Getenv("ONEC_USERNAME"); username != "" {
		config.Username = username
		log.Printf("INFO: ONEC_USERNAME установлен из ENV: %s", username)
	}

	if password := os.Getenv("ONEC_PASSWORD"); password != "" {
		config.Password = password
		log.Printf("INFO: ONEC_PASSWORD установлен из ENV: [скрыт]")
	}

	if timeoutStr := os.Getenv("ONEC_TIMEOUT"); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil {
			config.Timeout = timeout
			log.Printf("INFO: ONEC_TIMEOUT установлен из ENV: %d", timeout)
		} else {
			log.Printf("WARNING: Неверный формат ONEC_TIMEOUT: %s", timeoutStr)
		}
	}

	// Флаги включения/отключения
	if enabledStr := os.Getenv("ONEC_ENABLED"); enabledStr != "" {
		if enabled, err := strconv.ParseBool(enabledStr); err == nil {
			config.Enabled = enabled
			log.Printf("INFO: ONEC_ENABLED установлен из ENV: %t", enabled)
		} else {
			log.Printf("WARNING: Неверный формат ONEC_ENABLED: %s", enabledStr)
		}
	}

	if autoSendStr := os.Getenv("ONEC_AUTO_SEND"); autoSendStr != "" {
		if autoSend, err := strconv.ParseBool(autoSendStr); err == nil {
			config.AutoSend = autoSend
			log.Printf("INFO: ONEC_AUTO_SEND установлен из ENV: %t", autoSend)
		} else {
			log.Printf("WARNING: Неверный формат ONEC_AUTO_SEND: %s", autoSendStr)
		}
	}
}

// setDefaults устанавливает значения по умолчанию для не заданных параметров
func setDefaults(config *IntegrationConfig) {
	if config.Timeout == 0 {
		config.Timeout = 30
	}

	if config.Mapping == nil {
		config.Mapping = make(map[string]DocumentTypeConfig)
	}

	// Если базовые настройки для подключения не заданы, отключаем интеграцию
	if config.BaseURL == "" || config.Username == "" || config.Password == "" {
		if config.Enabled {
			log.Printf("WARNING: Не заданы базовые настройки подключения к 1С, отключаем интеграцию")
			config.Enabled = false
		}
	}
}
