package datastore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	ds "github.com/ipfs/go-datastore"
)

type APIServer struct {
	ds          Datastore
	enableCORS  bool
	logRequests bool
}

type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

type KeyInfo struct {
	Key         string            `json:"key"`
	Value       string            `json:"value"`
	Size        int               `json:"size"`
	ContentType string            `json:"content_type"`
	TTL         string            `json:"ttl,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

type SearchRequest struct {
	Query         string `json:"query"`
	CaseSensitive bool   `json:"case_sensitive,omitempty"`
	KeysOnly      bool   `json:"keys_only,omitempty"`
	Limit         int    `json:"limit,omitempty"`
}

type ListKeysRequest struct {
	Prefix   string `json:"prefix,omitempty"`
	KeysOnly bool   `json:"keys_only,omitempty"`
	Limit    int    `json:"limit,omitempty"`
}

type PutKeyRequest struct {
	Value string        `json:"value"`
	TTL   time.Duration `json:"ttl,omitempty"`
}

type SubscriptionRequest struct {
	ID               string   `json:"id"`
	Script           string   `json:"script"`
	ExecutionTimeout int      `json:"execution_timeout,omitempty"` // секунды
	EnableNetworking bool     `json:"enable_networking,omitempty"`
	EnableLogging    bool     `json:"enable_logging,omitempty"`
	EventFilters     []string `json:"event_filters,omitempty"`
	StrictMode       bool     `json:"strict_mode,omitempty"`
}

func (s *APIServer) setupRoutes(router *mux.Router) {
	// Middleware
	router.Use(s.corsMiddleware)
	if s.logRequests {
		router.Use(s.loggingMiddleware)
	}

	// API routes
	api := router.PathPrefix("/api").Subrouter()

	// Keys endpoints
	api.HandleFunc("/keys", s.handleListKeys).Methods("GET")
	api.HandleFunc("/keys/{key:.*}", s.handleGetKey).Methods("GET")
	api.HandleFunc("/keys/{key:.*}", s.handlePutKey).Methods("PUT")
	api.HandleFunc("/keys/{key:.*}", s.handleDeleteKey).Methods("DELETE")
	api.HandleFunc("/keys/{key:.*}/info", s.handleKeyInfo).Methods("GET")

	// Search
	api.HandleFunc("/search", s.handleSearch).Methods("POST")

	// Stats
	api.HandleFunc("/stats", s.handleStats).Methods("GET")

	// Clear
	api.HandleFunc("/clear", s.handleClear).Methods("DELETE")

	// Export/Import
	api.HandleFunc("/export", s.handleExport).Methods("POST")
	api.HandleFunc("/import", s.handleImport).Methods("POST")

	// Subscriptions
	api.HandleFunc("/subscriptions", s.handleListSubscriptions).Methods("GET")
	api.HandleFunc("/subscriptions", s.handleCreateSubscription).Methods("POST")
	api.HandleFunc("/subscriptions/{id}", s.handleDeleteSubscription).Methods("DELETE")

	// Health check
	api.HandleFunc("/health", s.handleHealth).Methods("GET")

	// API documentation
	api.HandleFunc("/docs", s.handleDocs).Methods("GET")

	// Root redirect
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/api/docs", http.StatusTemporaryRedirect)
		} else {
			s.sendErrorResponse(w, "Эндпоинт не найден", http.StatusNotFound)
		}
	})
}

func (s *APIServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.enableCORS {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *APIServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
	})
}

func (s *APIServer) sendResponse(w http.ResponseWriter, data interface{}) {
	s.sendResponseWithMessage(w, data, "", http.StatusOK)
}

func (s *APIServer) sendResponseWithMessage(w http.ResponseWriter, data interface{}, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := APIResponse{
		Success: statusCode < 400,
		Data:    data,
		Message: message,
	}

	json.NewEncoder(w).Encode(response)
}

func (s *APIServer) sendErrorResponse(w http.ResponseWriter, error string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := APIResponse{
		Success: false,
		Error:   error,
	}

	json.NewEncoder(w).Encode(response)
}

// Обработчики эндпоинтов

func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.sendResponse(w, map[string]string{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func (s *APIServer) handleListKeys(w http.ResponseWriter, r *http.Request) {
	// Параметры запроса
	prefix := r.URL.Query().Get("prefix")
	if prefix == "" {
		prefix = "/"
	}

	keysOnly := r.URL.Query().Get("keys_only") == "true"

	limit := 0
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dsPrefix := ds.NewKey(prefix)
	kvChan, errChan, err := s.ds.Iterator(ctx, dsPrefix, keysOnly)
	if err != nil {
		s.sendErrorResponse(w, fmt.Sprintf("Ошибка создания итератора: %v", err), http.StatusInternalServerError)
		return
	}

	var keys []interface{}
	count := 0

	for {
		select {
		case kv, ok := <-kvChan:
			if !ok {
				s.sendResponse(w, map[string]interface{}{
					"keys":  keys,
					"total": count,
				})
				return
			}

			count++
			if limit > 0 && count > limit {
				s.sendResponse(w, map[string]interface{}{
					"keys":      keys,
					"total":     count - 1,
					"truncated": true,
				})
				return
			}

			if keysOnly {
				keys = append(keys, kv.Key.String())
			} else {
				var contentType string
				if json.Valid(kv.Value) {
					contentType = "json"
				} else if isUTF8(kv.Value) {
					contentType = "text"
				} else {
					contentType = "binary"
				}

				keyInfo := map[string]interface{}{
					"key":          kv.Key.String(),
					"value":        string(kv.Value),
					"size":         len(kv.Value),
					"content_type": contentType,
				}
				keys = append(keys, keyInfo)
			}

		case err := <-errChan:
			if err != nil {
				s.sendErrorResponse(w, fmt.Sprintf("Ошибка итерации: %v", err), http.StatusInternalServerError)
				return
			}
		}
	}
}

func (s *APIServer) handleGetKey(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]

	if key == "" {
		s.sendErrorResponse(w, "Требуется ключ", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dsKey := ds.NewKey(key)
	data, err := s.ds.Get(ctx, dsKey)
	if err != nil {
		if err == ds.ErrNotFound {
			s.sendErrorResponse(w, "Ключ не найден", http.StatusNotFound)
		} else {
			s.sendErrorResponse(w, fmt.Sprintf("Ошибка получения ключа: %v", err), http.StatusInternalServerError)
		}
		return
	}

	// Определяем тип ответа по заголовку Accept или параметру format
	format := r.URL.Query().Get("format")
	accept := r.Header.Get("Accept")

	if format == "raw" || strings.Contains(accept, "text/plain") {
		// Возвращаем сырые данные
		w.Header().Set("Content-Type", "text/plain")
		w.Write(data)
		return
	}

	// Возвращаем JSON с метаданными
	var contentType string
	if json.Valid(data) {
		contentType = "json"
	} else if isUTF8(data) {
		contentType = "text"
	} else {
		contentType = "binary"
	}

	keyInfo := KeyInfo{
		Key:         key,
		Value:       string(data),
		Size:        len(data),
		ContentType: contentType,
	}

	s.sendResponse(w, keyInfo)
}

func (s *APIServer) handlePutKey(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]

	if key == "" {
		s.sendErrorResponse(w, "Требуется ключ", http.StatusBadRequest)
		return
	}

	// Читаем тело запроса
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.sendErrorResponse(w, "Ошибка чтения тела запроса", http.StatusBadRequest)
		return
	}

	var data []byte
	var ttl time.Duration

	// Проверяем Content-Type
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		// JSON запрос с метаданными
		var req PutKeyRequest
		if err := json.Unmarshal(body, &req); err != nil {
			s.sendErrorResponse(w, "Неверный JSON", http.StatusBadRequest)
			return
		}
		data = []byte(req.Value)
		ttl = req.TTL
	} else {
		// Сырые данные
		data = body
		// TTL из параметра запроса
		if ttlStr := r.URL.Query().Get("ttl"); ttlStr != "" {
			if t, err := time.ParseDuration(ttlStr); err == nil {
				ttl = t
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dsKey := ds.NewKey(key)

	if ttl > 0 {
		err = s.ds.PutWithTTL(ctx, dsKey, data, ttl)
	} else {
		err = s.ds.Put(ctx, dsKey, data)
	}

	if err != nil {
		s.sendErrorResponse(w, fmt.Sprintf("Ошибка сохранения ключа: %v", err), http.StatusInternalServerError)
		return
	}

	message := "Ключ сохранен"
	if ttl > 0 {
		message = fmt.Sprintf("Ключ сохранен с TTL %v", ttl)
	}

	s.sendResponseWithMessage(w, nil, message, http.StatusCreated)
}

func (s *APIServer) handleDeleteKey(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]

	if key == "" {
		s.sendErrorResponse(w, "Требуется ключ", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dsKey := ds.NewKey(key)
	err := s.ds.Delete(ctx, dsKey)
	if err != nil {
		s.sendErrorResponse(w, fmt.Sprintf("Ошибка удаления ключа: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendResponseWithMessage(w, nil, "Ключ удален", http.StatusOK)
}

func (s *APIServer) handleKeyInfo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["key"]

	if key == "" {
		s.sendErrorResponse(w, "Требуется ключ", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dsKey := ds.NewKey(key)

	// Проверяем существование
	exists, err := s.ds.Has(ctx, dsKey)
	if err != nil {
		s.sendErrorResponse(w, fmt.Sprintf("Ошибка проверки ключа: %v", err), http.StatusInternalServerError)
		return
	}

	if !exists {
		s.sendErrorResponse(w, "Ключ не найден", http.StatusNotFound)
		return
	}

	// Получаем данные
	data, err := s.ds.Get(ctx, dsKey)
	if err != nil {
		s.sendErrorResponse(w, fmt.Sprintf("Ошибка получения данных: %v", err), http.StatusInternalServerError)
		return
	}

	// Получаем TTL
	var ttlInfo string
	if expiration, err := s.ds.GetExpiration(ctx, dsKey); err == nil && !expiration.IsZero() {
		remaining := time.Until(expiration)
		if remaining > 0 {
			ttlInfo = remaining.String()
		} else {
			ttlInfo = "expired"
		}
	}

	// Определяем тип содержимого
	var contentType string
	if json.Valid(data) {
		contentType = "json"
	} else if isUTF8(data) {
		contentType = "text"
	} else {
		contentType = "binary"
	}

	keyInfo := KeyInfo{
		Key:         key,
		Value:       string(data),
		Size:        len(data),
		ContentType: contentType,
		TTL:         ttlInfo,
		Metadata:    make(map[string]string),
	}

	// Добавляем части ключа как метаданные
	parts := strings.Split(strings.Trim(key, "/"), "/")
	for i, part := range parts {
		if part != "" {
			keyInfo.Metadata[fmt.Sprintf("key_part_%d", i)] = part
		}
	}

	s.sendResponse(w, keyInfo)
}

func (s *APIServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendErrorResponse(w, "Неверный JSON", http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		s.sendErrorResponse(w, "Требуется поисковый запрос", http.StatusBadRequest)
		return
	}

	searchStr := req.Query
	if !req.CaseSensitive {
		searchStr = strings.ToLower(searchStr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	kvChan, errChan, err := s.ds.Iterator(ctx, ds.NewKey("/"), req.KeysOnly)
	if err != nil {
		s.sendErrorResponse(w, fmt.Sprintf("Ошибка создания итератора: %v", err), http.StatusInternalServerError)
		return
	}

	var results []interface{}
	found := 0
	total := 0

	for {
		select {
		case kv, ok := <-kvChan:
			if !ok {
				s.sendResponse(w, map[string]interface{}{
					"results": results,
					"found":   found,
					"total":   total,
				})
				return
			}

			total++
			keyStr := kv.Key.String()
			searchKey := keyStr

			if !req.CaseSensitive {
				searchKey = strings.ToLower(searchKey)
			}

			if strings.Contains(searchKey, searchStr) {
				found++

				if req.Limit > 0 && found > req.Limit {
					s.sendResponse(w, map[string]interface{}{
						"results":   results,
						"found":     found - 1,
						"total":     total,
						"truncated": true,
					})
					return
				}

				if req.KeysOnly {
					results = append(results, keyStr)
				} else {
					results = append(results, map[string]interface{}{
						"key":   keyStr,
						"value": string(kv.Value),
						"size":  len(kv.Value),
					})
				}
			}

		case err := <-errChan:
			if err != nil {
				s.sendErrorResponse(w, fmt.Sprintf("Ошибка итерации: %v", err), http.StatusInternalServerError)
				return
			}
		}
	}
}

func (s *APIServer) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Подсчитываем ключи
	keysChan, errChan, err := s.ds.Keys(ctx, ds.NewKey("/"))
	if err != nil {
		s.sendErrorResponse(w, fmt.Sprintf("Ошибка получения ключей: %v", err), http.StatusInternalServerError)
		return
	}

	totalKeys := 0
	for {
		select {
		case _, ok := <-keysChan:
			if !ok {
				goto countDone
			}
			totalKeys++
		case err := <-errChan:
			if err != nil {
				s.sendErrorResponse(w, fmt.Sprintf("Ошибка подсчета ключей: %v", err), http.StatusInternalServerError)
				return
			}
		}
	}

countDone:
	// Подсчитываем размер
	kvChan, errChan2, err := s.ds.Iterator(ctx, ds.NewKey("/"), false)
	if err != nil {
		s.sendErrorResponse(w, fmt.Sprintf("Ошибка создания итератора: %v", err), http.StatusInternalServerError)
		return
	}

	var totalSize int64
	for {
		select {
		case kv, ok := <-kvChan:
			if !ok {
				goto sizeDone
			}
			totalSize += int64(len(kv.Value))
		case err := <-errChan2:
			if err != nil {
				s.sendErrorResponse(w, fmt.Sprintf("Ошибка подсчета размера: %v", err), http.StatusInternalServerError)
				return
			}
		}
	}

sizeDone:
	var avgSize int64
	if totalKeys > 0 {
		avgSize = totalSize / int64(totalKeys)
	}

	stats := map[string]interface{}{
		"total_keys":         totalKeys,
		"total_size_bytes":   totalSize,
		"total_size_human":   formatBytes(totalSize),
		"average_size":       avgSize,
		"average_size_human": formatBytes(avgSize),
		"timestamp":          time.Now().Format(time.RFC3339),
	}

	s.sendResponse(w, stats)
}

func (s *APIServer) handleClear(w http.ResponseWriter, r *http.Request) {
	// Проверяем подтверждение
	if r.URL.Query().Get("confirm") != "true" {
		s.sendErrorResponse(w, "Требуется параметр ?confirm=true для подтверждения", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	err := s.ds.Clear(ctx)
	if err != nil {
		s.sendErrorResponse(w, fmt.Sprintf("Ошибка очистки: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendResponseWithMessage(w, nil, "Датастор очищен", http.StatusOK)
}

func (s *APIServer) handleListSubscriptions(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	subscriptions, err := s.ds.ListJSSubscriptions(ctx)
	if err != nil {
		s.sendErrorResponse(w, fmt.Sprintf("Ошибка получения подписок: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendResponse(w, map[string]interface{}{
		"subscriptions": subscriptions,
		"total":         len(subscriptions),
	})
}

func (s *APIServer) handleCreateSubscription(w http.ResponseWriter, r *http.Request) {
	var req SubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendErrorResponse(w, "Неверный JSON", http.StatusBadRequest)
		return
	}

	if req.ID == "" || req.Script == "" {
		s.sendErrorResponse(w, "Требуются поля ID и Script", http.StatusBadRequest)
		return
	}

	// Преобразуем строки событий в EventType
	var eventFilters []EventType
	for _, eventStr := range req.EventFilters {
		switch strings.ToLower(eventStr) {
		case "put":
			eventFilters = append(eventFilters, EventPut)
		case "delete":
			eventFilters = append(eventFilters, EventDelete)
		case "batch":
			eventFilters = append(eventFilters, EventBatch)
		}
	}

	timeout := time.Duration(req.ExecutionTimeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	config := &JSSubscriberConfig{
		ID:               req.ID,
		Script:           req.Script,
		ExecutionTimeout: timeout,
		EnableNetworking: req.EnableNetworking,
		EnableLogging:    req.EnableLogging,
		EventFilters:     eventFilters,
		StrictMode:       req.StrictMode,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := s.ds.CreateJSSubscription(ctx, req.ID, req.Script, config)
	if err != nil {
		s.sendErrorResponse(w, fmt.Sprintf("Ошибка создания подписки: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendResponseWithMessage(w, nil, "Подписка создана", http.StatusCreated)
}

func (s *APIServer) handleDeleteSubscription(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		s.sendErrorResponse(w, "Требуется ID подписки", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := s.ds.RemoveJSSubscription(ctx, id)
	if err != nil {
		s.sendErrorResponse(w, fmt.Sprintf("Ошибка удаления подписки: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendResponseWithMessage(w, nil, "Подписка удалена", http.StatusOK)
}

func (s *APIServer) handleExport(w http.ResponseWriter, r *http.Request) {
	// Упрощенная версия экспорта - возвращает JSON
	// Полная версия потребует streaming response для больших данных
	s.sendErrorResponse(w, "Экспорт через API пока не реализован - используйте CLI", http.StatusNotImplemented)
}

func (s *APIServer) handleImport(w http.ResponseWriter, r *http.Request) {
	// Упрощенная версия импорта
	s.sendErrorResponse(w, "Импорт через API пока не реализован - используйте CLI", http.StatusNotImplemented)
}

func (s *APIServer) handleDocs(w http.ResponseWriter, r *http.Request) {
	docs := `
<!DOCTYPE html>
<html>
<head>
    <title>UES Datastore API Documentation</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .endpoint { margin: 20px 0; padding: 15px; border: 1px solid #ddd; border-radius: 5px; }
        .method { display: inline-block; padding: 5px 10px; border-radius: 3px; color: white; font-weight: bold; margin-right: 10px; }
        .GET { background-color: #28a745; }
        .POST { background-color: #007bff; }
        .PUT { background-color: #ffc107; color: black; }
        .DELETE { background-color: #dc3545; }
        pre { background: #f8f9fa; padding: 10px; border-radius: 3px; overflow-x: auto; }
    </style>
</head>
<body>
    <h1>UES Datastore REST API</h1>
    
    <div class="endpoint">
        <span class="method GET">GET</span><code>/api/health</code>
        <p>Проверка состояния сервера</p>
    </div>

    <div class="endpoint">
        <span class="method GET">GET</span><code>/api/keys?prefix=/&keys_only=false&limit=100</code>
        <p>Получить список ключей</p>
    </div>

    <div class="endpoint">
        <span class="method GET">GET</span><code>/api/keys/{key}</code>
        <p>Получить значение ключа</p>
    </div>

    <div class="endpoint">
        <span class="method PUT">PUT</span><code>/api/keys/{key}?ttl=1h</code>
        <p>Установить значение ключа</p>
        <pre>Content-Type: text/plain
Тело: значение ключа</pre>
        <p>Или:</p>
        <pre>Content-Type: application/json
{"value": "значение", "ttl": "1h"}</pre>
    </div>

    <div class="endpoint">
        <span class="method DELETE">DELETE</span><code>/api/keys/{key}</code>
        <p>Удалить ключ</p>
    </div>

    <div class="endpoint">
        <span class="method GET">GET</span><code>/api/keys/{key}/info</code>
        <p>Получить информацию о ключе</p>
    </div>

    <div class="endpoint">
        <span class="method POST">POST</span><code>/api/search</code>
        <p>Поиск ключей</p>
        <pre>{"query": "поисковая строка", "case_sensitive": false, "keys_only": false, "limit": 100}</pre>
    </div>

    <div class="endpoint">
        <span class="method GET">GET</span><code>/api/stats</code>
        <p>Статистика датастора</p>
    </div>

    <div class="endpoint">
        <span class="method DELETE">DELETE</span><code>/api/clear?confirm=true</code>
        <p>Очистить весь датастор (требует confirm=true)</p>
    </div>

    <div class="endpoint">
        <span class="method GET">GET</span><code>/api/subscriptions</code>
        <p>Список подписок</p>
    </div>

    <div class="endpoint">
        <span class="method POST">POST</span><code>/api/subscriptions</code>
        <p>Создать подписку</p>
        <pre>{"id": "my-handler", "script": "console.log('Event:', event.type)", "execution_timeout": 5, "enable_logging": true}</pre>
    </div>

    <div class="endpoint">
        <span class="method DELETE">DELETE</span><code>/api/subscriptions/{id}</code>
        <p>Удалить подписку</p>
    </div>

</body>
</html>
`
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(docs))
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d Б", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cБ", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func isUTF8(data []byte) bool {
	return string(data) == strings.ToValidUTF8(string(data), "")
}
