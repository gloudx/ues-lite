package datastore

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
	"ues-lite/js"
)

type jsEvent struct {
	Type      string
	Key       string
	Value     string
	Timestamp time.Time
	Metadata  map[string]interface{} `json:"metadata"`
}

type JSSubscriberConfig struct {
	ID               string
	Script           string
	ExecutionTimeout time.Duration
	EnableNetworking bool
	EnableLogging    bool
	CustomLibraries  map[string]interface{}
	EventFilters     []EventType
	StrictMode       bool
}

type jsSubscriber struct {
	id     string
	script string
	config *JSSubscriberConfig
	mu     sync.RWMutex
	logger *log.Logger
}

var _ Subscriber = (*jsSubscriber)(nil)

func NewJSSubscriber(config *JSSubscriberConfig) (*jsSubscriber, error) {
	if config.ID == "" {
		return nil, fmt.Errorf("subscriber ID cannot be empty")
	}

	if config.ExecutionTimeout <= 0 {
		config.ExecutionTimeout = 5 * time.Second
	}

	subscriber := &jsSubscriber{
		id:     config.ID,
		script: config.Script,
		config: config,
	}

	if config.EnableLogging {
		subscriber.logger = log.New(log.Writer(), fmt.Sprintf("[JS-%s] ", config.ID), log.LstdFlags)
	}

	return subscriber, nil
}

func NewSimpleJSSubscriber(id, script string) (*jsSubscriber, error) {
	config := &JSSubscriberConfig{
		ID:               id,
		Script:           script,
		ExecutionTimeout: 5 * time.Second,
		EnableLogging:    true,
		EnableNetworking: true,
	}
	return NewJSSubscriber(config)
}

func NewFilteredJSSubscriber(id, script string, eventTypes ...EventType) (*jsSubscriber, error) {
	config := &JSSubscriberConfig{
		ID:               id,
		Script:           script,
		ExecutionTimeout: 5 * time.Second,
		EnableLogging:    true,
		EnableNetworking: true,
		EventFilters:     eventTypes,
	}
	return NewJSSubscriber(config)
}

func (s *jsSubscriber) ID() string {
	return s.id
}

func (s *jsSubscriber) OnEvent(ctx context.Context, event Event) {
	// Проверяем фильтры событий
	if len(s.config.EventFilters) > 0 {
		found := false
		for _, filter := range s.config.EventFilters {
			if event.Type == filter {
				found = true
				break
			}
		}
		if !found {
			return
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, s.config.ExecutionTimeout)
	defer cancel()

	done := make(chan bool, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if s.config.EnableLogging && s.logger != nil {
					s.logger.Printf("Script execution panic: %v", r)
				}
			}
			done <- true
		}()
		s.executeScript(ctx, event)
	}()

	select {
	case <-done:
		// Script completed
	case <-ctx.Done():
		if s.config.EnableLogging && s.logger != nil {
			s.logger.Printf("Script execution timeout for event %s", event.Key.String())
		}
	}
}

func (s *jsSubscriber) executeScript(ctx context.Context, event Event) {
	eventType := s.eventTypeToString(event.Type)

	var valueStr string
	if event.Value != nil {
		valueStr = string(event.Value)
	}

	// Создаем JS событие
	e := jsEvent{
		Type:      eventType,
		Key:       event.Key.String(),
		Value:     valueStr,
		Timestamp: event.Timestamp,
		Metadata:  make(map[string]interface{}),
	}

	// Копируем метаданные из события, если они есть
	if event.Metadata != nil {
		for k, v := range event.Metadata {
			e.Metadata[k] = v
		}
	}

	// Для TTL событий добавляем специальные метаданные
	if event.Type == EventTTLExpired {
		e.Metadata["is_ttl_expired"] = true
		// expired_at уже должен быть в event.Metadata
	}

	if s.script != "" {
		_, err := js.Eval(ctx, s.script, map[string]any{
			"event": e,
		})
		if err != nil && s.config.EnableLogging && s.logger != nil {
			s.logger.Printf("Script execution error: %v", err)
		}
	}
}

// eventTypeToString - конвертирует тип события в строку
func (s *jsSubscriber) eventTypeToString(eventType EventType) string {
	switch eventType {
	case EventPut:
		return "put"
	case EventDelete:
		return "delete"
	case EventBatch:
		return "batch"
	case EventTTLExpired:
		return "ttl_expired"
	default:
		return "unknown"
	}
}
