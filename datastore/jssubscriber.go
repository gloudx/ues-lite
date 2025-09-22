package datastore

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
)

// JSEventData represents event data passed to JavaScript
type JSEventData struct {
	Type      string            `json:"type"`
	Key       string            `json:"key"`
	Value     string            `json:"value"` // base64 encoded
	Timestamp int64             `json:"timestamp"`
	Metadata  map[string]string `json:"metadata"`
}

// JSSubscriberConfig holds configuration for JS subscriber
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

// JSSubscriber executes JavaScript code on datastore events
type JSSubscriber struct {
	id         string
	script     string
	vm         *goja.Runtime
	config     JSSubscriberConfig
	mu         sync.RWMutex
	logger     *log.Logger
	httpClient *http.Client
}

var _ Subscriber = (*JSSubscriber)(nil)

// NewJSSubscriber creates a new JavaScript subscriber
func NewJSSubscriber(config JSSubscriberConfig) (*JSSubscriber, error) {

	if config.ID == "" {
		return nil, fmt.Errorf("subscriber ID cannot be empty")
	}

	if config.ExecutionTimeout <= 0 {
		config.ExecutionTimeout = 5 * time.Second
	}

	subscriber := &JSSubscriber{
		id:     config.ID,
		script: config.Script,
		config: config,
		logger: log.New(io.Discard, "", 0),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	if config.EnableLogging {
		subscriber.logger = log.New(log.Writer(), fmt.Sprintf("[JSSubscriber-%s] ", config.ID), log.LstdFlags)
	}

	if err := subscriber.initVM(); err != nil {
		return nil, fmt.Errorf("failed to initialize JS runtime: %w", err)
	}

	return subscriber, nil
}

func (js *JSSubscriber) initVM() error {
	js.mu.Lock()
	defer js.mu.Unlock()

	vm := goja.New()

	// Security: disable dangerous functions
	vm.Set("require", goja.Undefined())
	vm.Set("module", goja.Undefined())
	vm.Set("exports", goja.Undefined())
	vm.Set("global", goja.Undefined())
	vm.Set("process", goja.Undefined())

	// Add safe built-in functions
	js.setupBuiltins(vm)

	// Add custom libraries
	if js.config.CustomLibraries != nil {
		for name, lib := range js.config.CustomLibraries {
			vm.Set(name, lib)
		}
	}

	// Compile and validate script
	if js.script != "" {
		_, err := goja.Compile("user-script", js.script, js.config.StrictMode)
		if err != nil {
			return fmt.Errorf("script compilation failed: %w", err)
		}
	}

	js.vm = vm

	return nil
}

func (js *JSSubscriber) setupBuiltins(vm *goja.Runtime) {
	// Console for logging
	console := map[string]interface{}{
		"log": func(args ...interface{}) {
			if js.config.EnableLogging {
				js.logger.Println(args...)
			}
		},
		"error": func(args ...interface{}) {
			if js.config.EnableLogging {
				js.logger.Printf("ERROR: %v\n", args)
			}
		},
		"info": func(args ...interface{}) {
			if js.config.EnableLogging {
				js.logger.Printf("INFO: %v\n", args)
			}
		},
	}
	vm.Set("console", console)

	// JSON utilities
	jsonUtils := map[string]interface{}{
		"parse": func(s string) interface{} {
			var result interface{}
			if err := json.Unmarshal([]byte(s), &result); err != nil {
				panic(vm.NewTypeError("Invalid JSON: " + err.Error()))
			}
			return result
		},
		"stringify": func(obj interface{}) string {
			bytes, err := json.Marshal(obj)
			if err != nil {
				panic(vm.NewTypeError("Cannot stringify object: " + err.Error()))
			}
			return string(bytes)
		},
	}

	vm.Set("JSON", jsonUtils)

	// String utilities
	stringUtils := map[string]interface{}{
		"contains": func(s, substr string) bool {
			return strings.Contains(s, substr)
		},
		"hasPrefix": func(s, prefix string) bool {
			return strings.HasPrefix(s, prefix)
		},
		"hasSuffix": func(s, suffix string) bool {
			return strings.HasSuffix(s, suffix)
		},
		"split": func(s, sep string) []string {
			return strings.Split(s, sep)
		},
		"join": func(elems []string, sep string) string {
			return strings.Join(elems, sep)
		},
		"trim": func(s string) string {
			return strings.TrimSpace(s)
		},
		"toLower": func(s string) string {
			return strings.ToLower(s)
		},
		"toUpper": func(s string) string {
			return strings.ToUpper(s)
		},
	}
	vm.Set("Strings", stringUtils)

	// Crypto utilities
	cryptoUtils := map[string]interface{}{
		"md5": func(data string) string {
			hash := md5.Sum([]byte(data))
			return hex.EncodeToString(hash[:])
		},
		"sha256": func(data string) string {
			hash := sha256.Sum256([]byte(data))
			return hex.EncodeToString(hash[:])
		},
	}
	vm.Set("Crypto", cryptoUtils)

	// Time utilities
	timeUtils := map[string]interface{}{
		"now": func() int64 {
			return time.Now().Unix()
		},
		"nowMillis": func() int64 {
			return time.Now().UnixMilli()
		},
		"format": func(timestamp int64, layout string) string {
			return time.Unix(timestamp, 0).Format(layout)
		},
		"parse": func(value, layout string) int64 {
			t, err := time.Parse(layout, value)
			if err != nil {
				panic(vm.NewTypeError("Invalid time format: " + err.Error()))
			}
			return t.Unix()
		},
	}
	vm.Set("Time", timeUtils)

	// HTTP utilities (if networking enabled)
	if js.config.EnableNetworking {
		httpUtils := map[string]interface{}{
			"get":  js.httpGet,
			"post": js.httpPost,
		}
		vm.Set("HTTP", httpUtils)
	}
}

func (js *JSSubscriber) httpGet(url string) map[string]interface{} {
	resp, err := js.httpClient.Get(url)
	if err != nil {
		return map[string]interface{}{
			"error":  err.Error(),
			"status": 0,
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return map[string]interface{}{
			"error":  err.Error(),
			"status": resp.StatusCode,
		}
	}

	return map[string]interface{}{
		"status":  resp.StatusCode,
		"body":    string(body),
		"headers": resp.Header,
	}
}

func (js *JSSubscriber) httpPost(url string, data interface{}) map[string]interface{} {
	var body io.Reader
	contentType := "application/json"

	switch v := data.(type) {
	case string:
		body = strings.NewReader(v)
		contentType = "text/plain"
	default:
		jsonData, err := json.Marshal(v)
		if err != nil {
			return map[string]interface{}{
				"error":  "Failed to marshal data: " + err.Error(),
				"status": 0,
			}
		}
		body = strings.NewReader(string(jsonData))
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return map[string]interface{}{
			"error":  err.Error(),
			"status": 0,
		}
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := js.httpClient.Do(req)
	if err != nil {
		return map[string]interface{}{
			"error":  err.Error(),
			"status": 0,
		}
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return map[string]interface{}{
			"error":  err.Error(),
			"status": resp.StatusCode,
		}
	}

	return map[string]interface{}{
		"status":  resp.StatusCode,
		"body":    string(responseBody),
		"headers": resp.Header,
	}
}

func (js *JSSubscriber) ID() string {
	return js.id
}

func (js *JSSubscriber) OnEvent(event Event) {
	// Check if subscriber is closed
	js.mu.RLock()
	if js.vm == nil {
		js.mu.RUnlock()
		return
	}
	js.mu.RUnlock()

	// Check event filters
	if len(js.config.EventFilters) > 0 {
		found := false
		for _, filter := range js.config.EventFilters {
			if event.Type == filter {
				found = true
				break
			}
		}
		if !found {
			return
		}
	}

	// Execute script with timeout
	ctx, cancel := context.WithTimeout(context.Background(), js.config.ExecutionTimeout)
	defer cancel()

	done := make(chan bool, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if js.config.EnableLogging {
					js.logger.Printf("Script execution panic: %v", r)
				}
			}
			done <- true
		}()

		js.executeScript(event)
	}()

	select {
	case <-done:
		// Script completed
	case <-ctx.Done():
		if js.config.EnableLogging {
			js.logger.Printf("Script execution timeout for event %s", event.Key.String())
		}
	}
}
func (js *JSSubscriber) executeScript(event Event) {

	js.mu.RLock()
	defer js.mu.RUnlock()

	// Double check VM is available
	if js.vm == nil {
		if js.config.EnableLogging {
			js.logger.Printf("VM is nil, skipping event for subscriber %s", js.id)
		}
		return
	}

	// Prepare event data for JavaScript
	eventType := "unknown"
	switch event.Type {
	case EventPut:
		eventType = "put"
	case EventDelete:
		eventType = "delete"
	case EventBatch:
		eventType = "batch"
	}

	// Convert value to base64 for safe transport
	var valueStr string
	if event.Value != nil {
		valueStr = string(event.Value)
	}

	jsEvent := JSEventData{
		Type:      eventType,
		Key:       event.Key.String(),
		Value:     valueStr,
		Timestamp: event.Timestamp.Unix(),
		Metadata:  make(map[string]string),
	}

	// Add key parts as metadata
	parts := strings.Split(strings.Trim(event.Key.String(), "/"), "/")
	for i, part := range parts {
		if part != "" {
			jsEvent.Metadata[fmt.Sprintf("key_part_%d", i)] = part
		}
	}

	// Set event data in VM
	js.vm.Set("event", jsEvent)

	// Execute main handler function if it exists
	// handlerFunc := js.vm.Get("onEvent")

	// if handlerFunc != nil && !goja.IsUndefined(handlerFunc) && !goja.IsNull(handlerFunc) {
	// 	if callable, ok := goja.AssertFunction(handlerFunc); ok {
	// 		_, err := callable(goja.Undefined(), js.vm.ToValue(jsEvent))
	// 		if err != nil && js.config.EnableLogging {
	// 			js.logger.Printf("Handler function error: %v", err)
	// 		}
	// 	}
	// } else {
	// Execute the entire script if no handler function

	if js.script != "" {
		_, err := js.vm.RunString(js.script)
		if err != nil && js.config.EnableLogging {
			js.logger.Printf("Script execution error: %v", err)
		}
	}
	// }
}

// UpdateScript safely updates the JavaScript code
func (js *JSSubscriber) UpdateScript(newScript string) error {

	js.mu.Lock()
	defer js.mu.Unlock()

	// Don't update if VM is closed
	if js.vm == nil {
		return fmt.Errorf("subscriber is closed")
	}

	// Test compilation
	_, err := goja.Compile("user-script", newScript, js.config.StrictMode)
	if err != nil {
		return fmt.Errorf("script compilation failed: %w", err)
	}

	js.script = newScript
	return nil
}

// GetScript returns current script
func (js *JSSubscriber) GetScript() string {
	js.mu.RLock()
	defer js.mu.RUnlock()
	return js.script
}

// Close cleans up resources
func (js *JSSubscriber) Close() error {

	js.mu.Lock()
	defer js.mu.Unlock()

	if js.vm != nil {
		js.vm.ClearInterrupt()
		js.vm = nil
	}

	return nil
}

// Helper function to create JS subscriber with common settings
func NewSimpleJSSubscriber(id, script string) (*JSSubscriber, error) {
	config := JSSubscriberConfig{
		ID:               id,
		Script:           script,
		ExecutionTimeout: 5 * time.Second,
		EnableLogging:    true,
		EnableNetworking: true,
	}

	return NewJSSubscriber(config)
}

// Helper function to create JS subscriber for specific event types
func NewFilteredJSSubscriber(id, script string, eventTypes ...EventType) (*JSSubscriber, error) {
	config := JSSubscriberConfig{
		ID:               id,
		Script:           script,
		ExecutionTimeout: 5 * time.Second,
		EnableLogging:    true,
		EnableNetworking: true,
		EventFilters:     eventTypes,
	}

	return NewJSSubscriber(config)
}
