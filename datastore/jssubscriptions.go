package datastore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
)

const (
	SubscriptionsNamespace = "/_system/ds-subscriptions"
)

type jsSubscription struct {
	ID               string      `json:"id"`
	Script           string      `json:"script"`
	ExecutionTimeout int64       `json:"execution_timeout"` // milliseconds
	EnableNetworking bool        `json:"enable_networking"`
	EnableLogging    bool        `json:"enable_logging"`
	EventFilters     []EventType `json:"event_filters"`
	StrictMode       bool        `json:"strict_mode"`
	CreatedAt        time.Time   `json:"created_at"`
	UpdatedAt        time.Time   `json:"updated_at"`
}

func (s *datastorage) CreateJSSubscription(ctx context.Context, id, script string, config *JSSubscriberConfig) error {

	if id == "" {
		return fmt.Errorf("subscription ID cannot be empty")
	}

	if config == nil {
		config = &JSSubscriberConfig{
			ID:               id,
			Script:           script,
			ExecutionTimeout: 5 * time.Second,
			EnableNetworking: true,
			EnableLogging:    true,
			StrictMode:       false,
		}
	} else {
		config.ID = id
		config.Script = script
	}

	subscriber, err := NewJSSubscriber(config)
	if err != nil {
		return fmt.Errorf("failed to create JS subscriber: %w", err)
	}

	savedSub := jsSubscription{
		ID:               id,
		Script:           script,
		ExecutionTimeout: config.ExecutionTimeout.Milliseconds(),
		EnableNetworking: config.EnableNetworking,
		EnableLogging:    config.EnableLogging,
		EventFilters:     config.EventFilters,
		StrictMode:       config.StrictMode,
		CreatedAt:        time.Now(),
	}

	data, err := json.Marshal(savedSub)
	if err != nil {
		return fmt.Errorf("failed to marshal subscription: %w", err)
	}

	key := ds.NewKey(SubscriptionsNamespace).ChildString(id)
	if err := s.Datastore.Put(ctx, key, data); err != nil {
		return fmt.Errorf("failed to save subscription: %w", err)
	}

	s.mu.Lock()
	if existing, exists := s.subscribers[id]; exists {
		if chSub, ok := existing.(*ChannelSubscriber); ok {
			chSub.Close()
		}
		delete(s.subscribers, id)
	}

	s.subscribers[id] = subscriber
	s.mu.Unlock()

	return nil
}

// RemoveJSSubscription removes a JS subscription
func (s *datastorage) RemoveJSSubscription(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("subscription ID cannot be empty")
	}

	// Remove from memory
	s.mu.Lock()
	if _, exists := s.subscribers[id]; exists {
		delete(s.subscribers, id)
	}
	s.mu.Unlock()

	// Remove from datastore
	key := ds.NewKey(SubscriptionsNamespace).ChildString(id)
	if err := s.Datastore.Delete(ctx, key); err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}

	return nil
}

func (s *datastorage) ListJSSubscriptions(ctx context.Context) ([]jsSubscription, error) {

	q := query.Query{
		Prefix: SubscriptionsNamespace,
	}

	results, err := s.Datastore.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("failed to query subscriptions: %w", err)
	}
	defer results.Close()

	var subscriptions []jsSubscription

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case result, ok := <-results.Next():
			if !ok {
				return subscriptions, nil
			}
			if result.Error != nil {
				return nil, result.Error
			}
			var savedSub jsSubscription
			if err := json.Unmarshal(result.Value, &savedSub); err != nil {
				// Skip invalid entries
				continue
			}

			subscriptions = append(subscriptions, savedSub)
		}
	}
}

func (s *datastorage) loadJSSubscriptions(ctx context.Context) error {

	subscriptions, err := s.ListJSSubscriptions(ctx)
	if err != nil {
		return fmt.Errorf("failed to list subscriptions: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	loadedCount := 0
	for _, savedSub := range subscriptions {
		config := &JSSubscriberConfig{
			ID:               savedSub.ID,
			Script:           savedSub.Script,
			ExecutionTimeout: time.Duration(savedSub.ExecutionTimeout) * time.Millisecond,
			EnableNetworking: savedSub.EnableNetworking,
			EnableLogging:    savedSub.EnableLogging,
			EventFilters:     savedSub.EventFilters,
			StrictMode:       savedSub.StrictMode,
		}

		jsSubscriber, err := NewJSSubscriber(config)
		if err != nil {
			// Skip invalid subscriptions but continue loading others
			log.Printf("failed to recreate subscription %s: %v", savedSub.ID, err)
			continue
		}

		if existing, exists := s.subscribers[savedSub.ID]; exists {
			if chSub, ok := existing.(*ChannelSubscriber); ok {
				chSub.Close()
			}
			delete(s.subscribers, savedSub.ID)
		}

		s.subscribers[savedSub.ID] = jsSubscriber

		loadedCount++
	}

	return nil
}
