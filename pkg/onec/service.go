package onec

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
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

// loadConfig загружает конфигурацию из файла
func loadConfig(configPath string) (*IntegrationConfig, error) {
	// Проверяем существует ли файл конфигурации
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Если конфигурации нет, возвращаем конфигурацию по умолчанию
		log.Printf("WARNING: Файл конфигурации %s не найден, используем настройки по умолчанию", configPath)
		return &IntegrationConfig{
			Enabled:  false,
			AutoSend: false,
			Mapping:  make(map[string]DocumentTypeConfig),
		}, nil
	}

	// Читаем файл
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать файл конфигурации: %w", err)
	}

	// Парсим JSON
	var config IntegrationConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("ошибка парсинга конфигурации: %w", err)
	}

	return &config, nil
}
