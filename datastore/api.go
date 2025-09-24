package datastore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	ds "github.com/ipfs/go-datastore"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
)

// Config конфигурация сервера
type Config struct {
	Host                 string        `json:"host"`
	Port                 int           `json:"port"`
	EnableCORS           bool          `json:"enable_cors"`
	EnableMetrics        bool          `json:"enable_metrics"`
	EnableAuth           bool          `json:"enable_auth"`
	AuthToken            string        `json:"auth_token"`
	LogRequests          bool          `json:"log_requests"`
	RequestTimeout       time.Duration `json:"request_timeout"`
	ReadTimeout          time.Duration `json:"read_timeout"`
	WriteTimeout         time.Duration `json:"write_timeout"`
	IdleTimeout          time.Duration `json:"idle_timeout"`
	ShutdownTimeout      time.Duration `json:"shutdown_timeout"`
	MaxRequestSize       int64         `json:"max_request_size"`
	RateLimitRPS         float64       `json:"rate_limit_rps"`
	RateLimitBurst       int           `json:"rate_limit_burst"`
	EnableCompression    bool          `json:"enable_compression"`
	EnableStructuredLogs bool          `json:"enable_structured_logs"`
}

// DefaultConfig возвращает конфигурацию по умолчанию
func DefaultConfig() *Config {
	return &Config{
		Host:                 "localhost",
		Port:                 8080,
		EnableCORS:           true,
		EnableMetrics:        true,
		EnableAuth:           false,
		LogRequests:          true,
		RequestTimeout:       30 * time.Second,
		ReadTimeout:          30 * time.Second,
		WriteTimeout:         30 * time.Second,
		IdleTimeout:          60 * time.Second,
		ShutdownTimeout:      30 * time.Second,
		MaxRequestSize:       32 << 20, // 32MB
		RateLimitRPS:         100,
		RateLimitBurst:       200,
		EnableCompression:    true,
		EnableStructuredLogs: true,
	}
}

// APIServer полноценный API сервер
type APIServer struct {
	ds       Datastore
	config   *Config
	server   *http.Server
	logger   *log.Logger
	metrics  *Metrics
	limiter  *rate.Limiter
	shutdown chan os.Signal
	wg       sync.WaitGroup
}

// Metrics метрики Prometheus
type Metrics struct {
	RequestsTotal       prometheus.Counter
	RequestDuration     prometheus.Histogram
	ActiveConnections   prometheus.Gauge
	ErrorsTotal         prometheus.Counter
	DatastoreOperations prometheus.CounterVec
	DatastoreSize       prometheus.Gauge
	DatastoreKeys       prometheus.Gauge
}

func NewMetrics() *Metrics {
	return &Metrics{
		RequestsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "datastore_api_requests_total",
			Help: "Общее количество HTTP запросов",
		}),
		RequestDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name: "datastore_api_request_duration_seconds",
			Help: "Продолжительность HTTP запросов",
		}),
		ActiveConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "datastore_api_active_connections",
			Help: "Количество активных соединений",
		}),
		ErrorsTotal: promauto.NewCounter(prometheus.CounterOpts{
			Name: "datastore_api_errors_total",
			Help: "Общее количество ошибок",
		}),
		DatastoreOperations: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "datastore_operations_total",
				Help: "Операции с датастором по типам",
			},
			[]string{"operation", "status"},
		),
		DatastoreSize: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "datastore_size_bytes",
			Help: "Размер датастора в байтах",
		}),
		DatastoreKeys: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "datastore_keys_total",
			Help: "Общее количество ключей в датасторе",
		}),
	}
}

// APIResponse стандартный ответ API
type APIResponse struct {
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Message   string      `json:"message,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

// NewAPIServer создает новый API сервер
func NewAPIServer(ds Datastore, config *Config) *APIServer {
	if config == nil {
		config = DefaultConfig()
	}

	server := &APIServer{
		ds:       ds,
		config:   config,
		logger:   log.New(os.Stdout, "[API] ", log.LstdFlags|log.Lshortfile),
		shutdown: make(chan os.Signal, 1),
	}

	if config.EnableMetrics {
		server.metrics = NewMetrics()
	}

	if config.RateLimitRPS > 0 {
		server.limiter = rate.NewLimiter(rate.Limit(config.RateLimitRPS), config.RateLimitBurst)
	}

	return server
}

// Start запускает сервер
func (s *APIServer) Start(ctx context.Context) error {
	router := mux.NewRouter()
	s.setupRoutes(router)

	s.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.config.Host, s.config.Port),
		Handler:      router,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
		IdleTimeout:  s.config.IdleTimeout,
	}

	signal.Notify(s.shutdown, os.Interrupt, syscall.SIGTERM)

	// Запускаем сервер в отдельной горутине
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.logger.Printf("Сервер запущен на %s", s.server.Addr)

		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Printf("Ошибка сервера: %v", err)
		}
	}()

	// Запускаем обновление метрик
	if s.metrics != nil {
		s.wg.Add(1)
		go s.metricsUpdater(ctx)
	}

	// Ожидаем сигнал завершения
	<-s.shutdown
	return s.gracefulShutdown()
}

// gracefulShutdown graceful shutdown сервера
func (s *APIServer) gracefulShutdown() error {
	s.logger.Println("Начинается graceful shutdown...")

	ctx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.Printf("Ошибка при shutdown: %v", err)
		return err
	}

	s.wg.Wait()
	s.logger.Println("Сервер остановлен")
	return nil
}

// setupRoutes настраивает маршруты
func (s *APIServer) setupRoutes(router *mux.Router) {
	// Middleware
	router.Use(s.recoveryMiddleware)
	router.Use(s.requestIDMiddleware)
	router.Use(s.corsMiddleware)
	router.Use(s.authMiddleware)
	router.Use(s.rateLimitMiddleware)
	router.Use(s.metricsMiddleware)
	router.Use(s.compressionMiddleware)

	if s.config.LogRequests {
		router.Use(s.loggingMiddleware)
	}

	// API routes
	api := router.PathPrefix("/api/v1").Subrouter()

	// Basic operations
	api.HandleFunc("/health", s.handleHealth).Methods("GET")
	api.HandleFunc("/keys", s.handleListKeys).Methods("GET")
	api.HandleFunc("/keys/{key:.*}", s.handleGetKey).Methods("GET")
	api.HandleFunc("/keys/{key:.*}", s.handlePutKey).Methods("PUT", "POST")
	api.HandleFunc("/keys/{key:.*}", s.handleDeleteKey).Methods("DELETE")
	api.HandleFunc("/keys/{key:.*}/info", s.handleKeyInfo).Methods("GET")
	api.HandleFunc("/search", s.handleSearch).Methods("POST")
	api.HandleFunc("/stats", s.handleStats).Methods("GET")
	api.HandleFunc("/clear", s.handleClear).Methods("DELETE")

	// JQ queries
	api.HandleFunc("/query", s.handleJQQuery).Methods("POST")
	api.HandleFunc("/query/aggregate", s.handleJQAggregate).Methods("POST")
	api.HandleFunc("/keys/{key:.*}/query", s.handleJQSingle).Methods("POST")

	// Views
	api.HandleFunc("/views", s.handleListViews).Methods("GET")
	api.HandleFunc("/views", s.handleCreateView).Methods("POST")
	api.HandleFunc("/views/{id}", s.handleGetView).Methods("GET")
	api.HandleFunc("/views/{id}", s.handleUpdateView).Methods("PUT")
	api.HandleFunc("/views/{id}", s.handleDeleteView).Methods("DELETE")
	api.HandleFunc("/views/{id}/execute", s.handleExecuteView).Methods("POST")
	api.HandleFunc("/views/{id}/refresh", s.handleRefreshView).Methods("POST")
	api.HandleFunc("/views/refresh", s.handleRefreshAllViews).Methods("POST")
	api.HandleFunc("/views/{id}/stats", s.handleViewStats).Methods("GET")

	// Transform operations
	api.HandleFunc("/transform", s.handleTransform).Methods("POST")
	api.HandleFunc("/transform/jq", s.handleTransformJQ).Methods("POST")
	api.HandleFunc("/transform/patch", s.handleTransformPatch).Methods("POST")

	// TTL operations
	api.HandleFunc("/ttl/stats", s.handleTTLStats).Methods("GET")
	api.HandleFunc("/ttl/keys", s.handleTTLKeys).Methods("GET")
	api.HandleFunc("/ttl/cleanup", s.handleTTLCleanup).Methods("DELETE")
	api.HandleFunc("/ttl/{key:.*}/extend", s.handleExtendTTL).Methods("POST")
	api.HandleFunc("/ttl/{key:.*}/refresh", s.handleRefreshTTL).Methods("POST")

	// Streaming
	api.HandleFunc("/stream", s.handleStream).Methods("GET")
	api.HandleFunc("/stream/events", s.handleStreamEvents).Methods("GET")
	api.HandleFunc("/stream/json", s.handleStreamJSON).Methods("GET")
	api.HandleFunc("/stream/jsonl", s.handleStreamJSONL).Methods("GET")
	api.HandleFunc("/stream/csv", s.handleStreamCSV).Methods("GET")
	api.HandleFunc("/stream/sse", s.handleStreamSSE).Methods("GET")

	// Batch operations
	api.HandleFunc("/batch", s.handleBatch).Methods("POST")

	// Subscriptions
	api.HandleFunc("/subscriptions", s.handleListSubscriptions).Methods("GET")
	api.HandleFunc("/subscriptions", s.handleCreateSubscription).Methods("POST")
	api.HandleFunc("/subscriptions/{id}", s.handleDeleteSubscription).Methods("DELETE")

	// Export/Import
	api.HandleFunc("/export", s.handleExport).Methods("GET")
	api.HandleFunc("/import", s.handleImport).Methods("POST")

	// System operations
	api.HandleFunc("/system/mode", s.handleSetMode).Methods("POST")
	api.HandleFunc("/system/gc", s.handleGC).Methods("POST")

	// Metrics endpoint
	if s.metrics != nil {
		router.Handle("/metrics", promhttp.Handler()).Methods("GET")
	}

	// Documentation
	api.HandleFunc("/docs", s.handleDocs).Methods("GET")

	// Root redirect
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/api/v1/docs", http.StatusTemporaryRedirect)
		} else {
			s.sendErrorResponse(w, r, "Эндпоинт не найден", http.StatusNotFound)
		}
	})
}

// Middleware

func (s *APIServer) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				s.logger.Printf("PANIC: %v", err)
				if s.metrics != nil {
					s.metrics.ErrorsTotal.Inc()
				}
				s.sendErrorResponse(w, r, "Внутренняя ошибка сервера", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *APIServer) requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		ctx := context.WithValue(r.Context(), "request_id", requestID)
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *APIServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.config.EnableCORS {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")

			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *APIServer) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.config.EnableAuth {
			// Skip auth for health check and metrics
			if r.URL.Path == "/api/v1/health" || r.URL.Path == "/metrics" {
				next.ServeHTTP(w, r)
				return
			}

			auth := r.Header.Get("Authorization")
			if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
				s.sendErrorResponse(w, r, "Требуется авторизация", http.StatusUnauthorized)
				return
			}

			token := strings.TrimPrefix(auth, "Bearer ")
			if token != s.config.AuthToken {
				s.sendErrorResponse(w, r, "Недействительный токен", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *APIServer) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.limiter != nil {
			if !s.limiter.Allow() {
				s.sendErrorResponse(w, r, "Превышен лимит запросов", http.StatusTooManyRequests)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *APIServer) metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.metrics != nil {
			start := time.Now()
			s.metrics.RequestsTotal.Inc()
			s.metrics.ActiveConnections.Inc()

			defer func() {
				s.metrics.ActiveConnections.Dec()
				s.metrics.RequestDuration.Observe(time.Since(start).Seconds())
			}()
		}
		next.ServeHTTP(w, r)
	})
}

func (s *APIServer) compressionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.config.EnableCompression {
			// Простая реализация сжатия
			if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				w.Header().Set("Content-Encoding", "gzip")
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *APIServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Создаем wrapper для ResponseWriter чтобы захватить статус
		wrapper := &responseWrapper{ResponseWriter: w, status: 200}

		next.ServeHTTP(wrapper, r)

		duration := time.Since(start)
		requestID := r.Context().Value("request_id")

		if s.config.EnableStructuredLogs {
			s.logger.Printf(`{"method":"%s","path":"%s","status":%d,"duration":"%v","request_id":"%v","remote_addr":"%s"}`,
				r.Method, r.URL.Path, wrapper.status, duration, requestID, r.RemoteAddr)
		} else {
			s.logger.Printf("%s %s %d %v %v", r.Method, r.URL.Path, wrapper.status, duration, requestID)
		}
	})
}

type responseWrapper struct {
	http.ResponseWriter
	status int
}

func (rw *responseWrapper) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Response helpers

func (s *APIServer) sendResponse(w http.ResponseWriter, r *http.Request, data interface{}) {
	s.sendResponseWithMessage(w, r, data, "", http.StatusOK)
}

func (s *APIServer) sendResponseWithMessage(w http.ResponseWriter, r *http.Request, data interface{}, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	requestID := r.Context().Value("request_id")
	response := APIResponse{
		Success:   statusCode < 400,
		Data:      data,
		Message:   message,
		RequestID: fmt.Sprintf("%v", requestID),
		Timestamp: time.Now(),
	}

	json.NewEncoder(w).Encode(response)
}

func (s *APIServer) sendErrorResponse(w http.ResponseWriter, r *http.Request, error string, statusCode int) {
	if s.metrics != nil {
		s.metrics.ErrorsTotal.Inc()
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	requestID := r.Context().Value("request_id")
	response := APIResponse{
		Success:   false,
		Error:     error,
		RequestID: fmt.Sprintf("%v", requestID),
		Timestamp: time.Now(),
	}

	json.NewEncoder(w).Encode(response)
}

// Request helpers

func (s *APIServer) parseJSONBody(r *http.Request, target interface{}) error {
	if r.ContentLength > s.config.MaxRequestSize {
		return fmt.Errorf("тело запроса слишком большое")
	}

	body := http.MaxBytesReader(nil, r.Body, s.config.MaxRequestSize)
	return json.NewDecoder(body).Decode(target)
}

func (s *APIServer) getContextWithTimeout(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), s.config.RequestTimeout)
}

// Handlers implementation

func (s *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	// Проверяем состояние датастора
	testKey := ds.NewKey("/_health_check")
	testValue := []byte("test")

	if err := s.ds.Put(ctx, testKey, testValue); err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка записи в датастор: %v", err), http.StatusServiceUnavailable)
		return
	}

	if err := s.ds.Delete(ctx, testKey); err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка удаления из датастора: %v", err), http.StatusServiceUnavailable)
		return
	}

	s.sendResponse(w, r, map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"version":   "1.0.0",
		"uptime":    time.Since(time.Now()).String(), // В реальности нужно сохранить время запуска
	})
}

func (s *APIServer) handleListKeys(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

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

	if s.metrics != nil {
		s.metrics.DatastoreOperations.WithLabelValues("list_keys", "started").Inc()
	}

	dsPrefix := ds.NewKey(prefix)
	kvChan, errChan, err := s.ds.Iterator(ctx, dsPrefix, keysOnly)
	if err != nil {
		if s.metrics != nil {
			s.metrics.DatastoreOperations.WithLabelValues("list_keys", "error").Inc()
		}
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка создания итератора: %v", err), http.StatusInternalServerError)
		return
	}

	var keys []interface{}
	count := 0

	for {
		select {
		case kv, ok := <-kvChan:
			if !ok {
				if s.metrics != nil {
					s.metrics.DatastoreOperations.WithLabelValues("list_keys", "success").Inc()
				}
				s.sendResponse(w, r, map[string]interface{}{
					"keys":  keys,
					"total": count,
				})
				return
			}

			count++
			if limit > 0 && count > limit {
				if s.metrics != nil {
					s.metrics.DatastoreOperations.WithLabelValues("list_keys", "success").Inc()
				}
				s.sendResponse(w, r, map[string]interface{}{
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
				if s.metrics != nil {
					s.metrics.DatastoreOperations.WithLabelValues("list_keys", "error").Inc()
				}
				s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка итерации: %v", err), http.StatusInternalServerError)
				return
			}
		case <-ctx.Done():
			if s.metrics != nil {
				s.metrics.DatastoreOperations.WithLabelValues("list_keys", "timeout").Inc()
			}
			s.sendErrorResponse(w, r, "Таймаут запроса", http.StatusRequestTimeout)
			return
		}
	}
}

func (s *APIServer) handleGetKey(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	vars := mux.Vars(r)
	key := vars["key"]

	if key == "" {
		s.sendErrorResponse(w, r, "Требуется ключ", http.StatusBadRequest)
		return
	}

	if s.metrics != nil {
		s.metrics.DatastoreOperations.WithLabelValues("get", "started").Inc()
	}

	dsKey := ds.NewKey(key)
	data, err := s.ds.Get(ctx, dsKey)
	if err != nil {
		if s.metrics != nil {
			s.metrics.DatastoreOperations.WithLabelValues("get", "error").Inc()
		}
		if err == ds.ErrNotFound {
			s.sendErrorResponse(w, r, "Ключ не найден", http.StatusNotFound)
		} else {
			s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка получения ключа: %v", err), http.StatusInternalServerError)
		}
		return
	}

	if s.metrics != nil {
		s.metrics.DatastoreOperations.WithLabelValues("get", "success").Inc()
	}

	format := r.URL.Query().Get("format")
	accept := r.Header.Get("Accept")

	if format == "raw" || strings.Contains(accept, "text/plain") {
		w.Header().Set("Content-Type", "text/plain")
		w.Write(data)
		return
	}

	var contentType string
	if json.Valid(data) {
		contentType = "json"
	} else if isUTF8(data) {
		contentType = "text"
	} else {
		contentType = "binary"
	}

	keyInfo := map[string]interface{}{
		"key":          key,
		"value":        string(data),
		"size":         len(data),
		"content_type": contentType,
	}

	s.sendResponse(w, r, keyInfo)
}

func (s *APIServer) handlePutKey(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	vars := mux.Vars(r)
	key := vars["key"]

	if key == "" {
		s.sendErrorResponse(w, r, "Требуется ключ", http.StatusBadRequest)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, s.config.MaxRequestSize))
	if err != nil {
		s.sendErrorResponse(w, r, "Ошибка чтения тела запроса", http.StatusBadRequest)
		return
	}

	if s.metrics != nil {
		s.metrics.DatastoreOperations.WithLabelValues("put", "started").Inc()
	}

	var data []byte
	var ttl time.Duration

	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var req struct {
			Value string        `json:"value"`
			TTL   time.Duration `json:"ttl,omitempty"`
		}
		if err := json.Unmarshal(body, &req); err != nil {
			s.sendErrorResponse(w, r, "Неверный JSON", http.StatusBadRequest)
			return
		}
		data = []byte(req.Value)
		ttl = req.TTL
	} else {
		data = body
		if ttlStr := r.URL.Query().Get("ttl"); ttlStr != "" {
			if t, err := time.ParseDuration(ttlStr); err == nil {
				ttl = t
			}
		}
	}

	dsKey := ds.NewKey(key)

	if ttl > 0 {
		err = s.ds.PutWithTTL(ctx, dsKey, data, ttl)
	} else {
		err = s.ds.Put(ctx, dsKey, data)
	}

	if err != nil {
		if s.metrics != nil {
			s.metrics.DatastoreOperations.WithLabelValues("put", "error").Inc()
		}
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка сохранения ключа: %v", err), http.StatusInternalServerError)
		return
	}

	if s.metrics != nil {
		s.metrics.DatastoreOperations.WithLabelValues("put", "success").Inc()
	}

	message := "Ключ сохранен"
	if ttl > 0 {
		message = fmt.Sprintf("Ключ сохранен с TTL %v", ttl)
	}

	s.sendResponseWithMessage(w, r, nil, message, http.StatusCreated)
}

func (s *APIServer) handleDeleteKey(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	vars := mux.Vars(r)
	key := vars["key"]

	if key == "" {
		s.sendErrorResponse(w, r, "Требуется ключ", http.StatusBadRequest)
		return
	}

	if s.metrics != nil {
		s.metrics.DatastoreOperations.WithLabelValues("delete", "started").Inc()
	}

	dsKey := ds.NewKey(key)
	err := s.ds.Delete(ctx, dsKey)
	if err != nil {
		if s.metrics != nil {
			s.metrics.DatastoreOperations.WithLabelValues("delete", "error").Inc()
		}
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка удаления ключа: %v", err), http.StatusInternalServerError)
		return
	}

	if s.metrics != nil {
		s.metrics.DatastoreOperations.WithLabelValues("delete", "success").Inc()
	}

	s.sendResponseWithMessage(w, r, nil, "Ключ удален", http.StatusOK)
}

func (s *APIServer) handleKeyInfo(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	vars := mux.Vars(r)
	key := vars["key"]

	if key == "" {
		s.sendErrorResponse(w, r, "Требуется ключ", http.StatusBadRequest)
		return
	}

	dsKey := ds.NewKey(key)

	exists, err := s.ds.Has(ctx, dsKey)
	if err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка проверки ключа: %v", err), http.StatusInternalServerError)
		return
	}

	if !exists {
		s.sendErrorResponse(w, r, "Ключ не найден", http.StatusNotFound)
		return
	}

	data, err := s.ds.Get(ctx, dsKey)
	if err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка получения данных: %v", err), http.StatusInternalServerError)
		return
	}

	var ttlInfo string
	if expiration, err := s.ds.GetExpiration(ctx, dsKey); err == nil && !expiration.IsZero() {
		remaining := time.Until(expiration)
		if remaining > 0 {
			ttlInfo = remaining.String()
		} else {
			ttlInfo = "expired"
		}
	}

	var contentType string
	if json.Valid(data) {
		contentType = "json"
	} else if isUTF8(data) {
		contentType = "text"
	} else {
		contentType = "binary"
	}

	keyInfo := map[string]interface{}{
		"key":          key,
		"value":        string(data),
		"size":         len(data),
		"content_type": contentType,
		"ttl":          ttlInfo,
		"metadata":     make(map[string]string),
	}

	parts := strings.Split(strings.Trim(key, "/"), "/")
	metadata := make(map[string]string)
	for i, part := range parts {
		if part != "" {
			metadata[fmt.Sprintf("key_part_%d", i)] = part
		}
	}
	keyInfo["metadata"] = metadata

	s.sendResponse(w, r, keyInfo)
}

func (s *APIServer) handleSearch(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	var req struct {
		Query         string `json:"query"`
		CaseSensitive bool   `json:"case_sensitive,omitempty"`
		KeysOnly      bool   `json:"keys_only,omitempty"`
		Limit         int    `json:"limit,omitempty"`
	}

	if err := s.parseJSONBody(r, &req); err != nil {
		s.sendErrorResponse(w, r, "Неверный JSON", http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		s.sendErrorResponse(w, r, "Требуется поисковый запрос", http.StatusBadRequest)
		return
	}

	searchStr := req.Query
	if !req.CaseSensitive {
		searchStr = strings.ToLower(searchStr)
	}

	kvChan, errChan, err := s.ds.Iterator(ctx, ds.NewKey("/"), req.KeysOnly)
	if err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка создания итератора: %v", err), http.StatusInternalServerError)
		return
	}

	var results []interface{}
	found := 0
	total := 0

	for {
		select {
		case kv, ok := <-kvChan:
			if !ok {
				s.sendResponse(w, r, map[string]interface{}{
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
					s.sendResponse(w, r, map[string]interface{}{
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
				s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка итерации: %v", err), http.StatusInternalServerError)
				return
			}
		case <-ctx.Done():
			s.sendErrorResponse(w, r, "Таймаут запроса", http.StatusRequestTimeout)
			return
		}
	}
}

func (s *APIServer) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	keysChan, errChan, err := s.ds.Keys(ctx, ds.NewKey("/"))
	if err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка получения ключей: %v", err), http.StatusInternalServerError)
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
				s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка подсчета ключей: %v", err), http.StatusInternalServerError)
				return
			}
		case <-ctx.Done():
			s.sendErrorResponse(w, r, "Таймаут запроса", http.StatusRequestTimeout)
			return
		}
	}

countDone:
	kvChan, errChan2, err := s.ds.Iterator(ctx, ds.NewKey("/"), false)
	if err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка создания итератора: %v", err), http.StatusInternalServerError)
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
				s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка подсчета размера: %v", err), http.StatusInternalServerError)
				return
			}
		case <-ctx.Done():
			s.sendErrorResponse(w, r, "Таймаут запроса", http.StatusRequestTimeout)
			return
		}
	}

sizeDone:
	var avgSize int64
	if totalKeys > 0 {
		avgSize = totalSize / int64(totalKeys)
	}

	// Обновляем метрики
	if s.metrics != nil {
		s.metrics.DatastoreSize.Set(float64(totalSize))
		s.metrics.DatastoreKeys.Set(float64(totalKeys))
	}

	stats := map[string]interface{}{
		"total_keys":         totalKeys,
		"total_size_bytes":   totalSize,
		"total_size_human":   formatBytes(totalSize),
		"average_size":       avgSize,
		"average_size_human": formatBytes(avgSize),
		"timestamp":          time.Now().Format(time.RFC3339),
	}

	s.sendResponse(w, r, stats)
}

func (s *APIServer) handleClear(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	if r.URL.Query().Get("confirm") != "true" {
		s.sendErrorResponse(w, r, "Требуется параметр ?confirm=true для подтверждения", http.StatusBadRequest)
		return
	}

	err := s.ds.Clear(ctx)
	if err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка очистки: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendResponseWithMessage(w, r, nil, "Датастор очищен", http.StatusOK)
}

// JQ Query handlers

func (s *APIServer) handleJQQuery(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	var req struct {
		Query            string        `json:"query"`
		Prefix           string        `json:"prefix,omitempty"`
		Limit            int           `json:"limit,omitempty"`
		Timeout          time.Duration `json:"timeout,omitempty"`
		TreatAsString    bool          `json:"treat_as_string,omitempty"`
		IgnoreParseError bool          `json:"ignore_parse_error,omitempty"`
	}

	if err := s.parseJSONBody(r, &req); err != nil {
		s.sendErrorResponse(w, r, "Неверный JSON", http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		s.sendErrorResponse(w, r, "Требуется JQ запрос", http.StatusBadRequest)
		return
	}

	prefix := ds.NewKey("/")
	if req.Prefix != "" {
		prefix = ds.NewKey(req.Prefix)
	}

	opts := &JQQueryOptions{
		Prefix:           prefix,
		Limit:            req.Limit,
		Timeout:          req.Timeout,
		TreatAsString:    req.TreatAsString,
		IgnoreParseError: req.IgnoreParseError,
	}

	resultChan, errorChan, err := s.ds.QueryJQ(ctx, req.Query, opts)
	if err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка выполнения JQ запроса: %v", err), http.StatusInternalServerError)
		return
	}

	var results []map[string]interface{}
	for {
		select {
		case <-ctx.Done():
			s.sendErrorResponse(w, r, "Таймаут запроса", http.StatusRequestTimeout)
			return
		case err, ok := <-errorChan:
			if ok && err != nil {
				s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка JQ запроса: %v", err), http.StatusInternalServerError)
				return
			}
		case result, ok := <-resultChan:
			if !ok {
				s.sendResponse(w, r, map[string]interface{}{
					"results": results,
					"total":   len(results),
				})
				return
			}

			results = append(results, map[string]interface{}{
				"key":   result.Key.String(),
				"value": result.Value,
			})
		}
	}
}

func (s *APIServer) handleJQAggregate(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	var req struct {
		Query            string        `json:"query"`
		Prefix           string        `json:"prefix,omitempty"`
		Timeout          time.Duration `json:"timeout,omitempty"`
		TreatAsString    bool          `json:"treat_as_string,omitempty"`
		IgnoreParseError bool          `json:"ignore_parse_error,omitempty"`
	}

	if err := s.parseJSONBody(r, &req); err != nil {
		s.sendErrorResponse(w, r, "Неверный JSON", http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		s.sendErrorResponse(w, r, "Требуется JQ запрос", http.StatusBadRequest)
		return
	}

	prefix := ds.NewKey("/")
	if req.Prefix != "" {
		prefix = ds.NewKey(req.Prefix)
	}

	opts := &JQQueryOptions{
		Prefix:           prefix,
		Timeout:          req.Timeout,
		TreatAsString:    req.TreatAsString,
		IgnoreParseError: req.IgnoreParseError,
	}

	result, err := s.ds.AggregateJQ(ctx, req.Query, opts)
	if err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка выполнения JQ агрегации: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendResponse(w, r, map[string]interface{}{
		"result": result,
	})
}

func (s *APIServer) handleJQSingle(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	vars := mux.Vars(r)
	key := vars["key"]

	if key == "" {
		s.sendErrorResponse(w, r, "Требуется ключ", http.StatusBadRequest)
		return
	}

	var req struct {
		Query string `json:"query"`
	}

	if err := s.parseJSONBody(r, &req); err != nil {
		s.sendErrorResponse(w, r, "Неверный JSON", http.StatusBadRequest)
		return
	}

	if req.Query == "" {
		s.sendErrorResponse(w, r, "Требуется JQ запрос", http.StatusBadRequest)
		return
	}

	dsKey := ds.NewKey(key)
	result, err := s.ds.QueryJQSingle(ctx, dsKey, req.Query)
	if err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка выполнения JQ запроса: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendResponse(w, r, map[string]interface{}{
		"key":    key,
		"result": result,
	})
}

// Views handlers

func (s *APIServer) handleListViews(w http.ResponseWriter, r *http.Request) {
	views := s.ds.ListViews()
	viewData := make([]map[string]interface{}, len(views))

	for i, view := range views {
		config := view.Config()
		stats := view.Stats()
		viewData[i] = map[string]interface{}{
			"id":     config.ID,
			"name":   config.Name,
			"config": config,
			"stats":  stats,
		}
	}

	s.sendResponse(w, r, map[string]interface{}{
		"views": viewData,
		"total": len(views),
	})
}

func (s *APIServer) handleCreateView(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	var config ViewConfig
	if err := s.parseJSONBody(r, &config); err != nil {
		s.sendErrorResponse(w, r, "Неверный JSON", http.StatusBadRequest)
		return
	}

	view, err := s.ds.CreateView(ctx, config)
	if err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка создания view: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendResponseWithMessage(w, r, map[string]interface{}{
		"id":     view.ID(),
		"config": view.Config(),
	}, "View создан", http.StatusCreated)
}

func (s *APIServer) handleGetView(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	view, exists := s.ds.GetView(id)
	if !exists {
		s.sendErrorResponse(w, r, "View не найден", http.StatusNotFound)
		return
	}

	s.sendResponse(w, r, map[string]interface{}{
		"id":     view.ID(),
		"config": view.Config(),
		"stats":  view.Stats(),
	})
}

func (s *APIServer) handleUpdateView(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	view, exists := s.ds.GetView(id)
	if !exists {
		s.sendErrorResponse(w, r, "View не найден", http.StatusNotFound)
		return
	}

	var config ViewConfig
	if err := s.parseJSONBody(r, &config); err != nil {
		s.sendErrorResponse(w, r, "Неверный JSON", http.StatusBadRequest)
		return
	}

	config.ID = id // ID не может быть изменен
	if err := view.UpdateConfig(config); err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка обновления view: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendResponseWithMessage(w, r, map[string]interface{}{
		"id":     view.ID(),
		"config": view.Config(),
	}, "View обновлен", http.StatusOK)
}

func (s *APIServer) handleDeleteView(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	vars := mux.Vars(r)
	id := vars["id"]

	if err := s.ds.RemoveView(ctx, id); err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка удаления view: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendResponseWithMessage(w, r, nil, "View удален", http.StatusOK)
}

func (s *APIServer) handleExecuteView(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	vars := mux.Vars(r)
	id := vars["id"]

	results, err := s.ds.ExecuteView(ctx, id)
	if err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка выполнения view: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendResponse(w, r, map[string]interface{}{
		"results": results,
		"total":   len(results),
	})
}

func (s *APIServer) handleRefreshView(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	vars := mux.Vars(r)
	id := vars["id"]

	if err := s.ds.RefreshView(ctx, id); err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка обновления view: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendResponseWithMessage(w, r, nil, "View обновлен", http.StatusOK)
}

func (s *APIServer) handleRefreshAllViews(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	if err := s.ds.RefreshAllViews(ctx); err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка обновления views: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendResponseWithMessage(w, r, nil, "Все views обновлены", http.StatusOK)
}

func (s *APIServer) handleViewStats(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	stats, exists := s.ds.GetViewStats(id)
	if !exists {
		s.sendErrorResponse(w, r, "View не найден", http.StatusNotFound)
		return
	}

	s.sendResponse(w, r, stats)
}

// Transform handlers

func (s *APIServer) handleTransform(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	var req struct {
		Key     string            `json:"key,omitempty"`
		Prefix  string            `json:"prefix,omitempty"`
		Options *TransformOptions `json:"options"`
	}

	if err := s.parseJSONBody(r, &req); err != nil {
		s.sendErrorResponse(w, r, "Неверный JSON", http.StatusBadRequest)
		return
	}

	var key ds.Key
	if req.Key != "" {
		key = ds.NewKey(req.Key)
	}

	if req.Prefix != "" && req.Options != nil {
		req.Options.Prefix = ds.NewKey(req.Prefix)
	}

	summary, err := s.ds.Transform(ctx, key, req.Options)
	if err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка трансформации: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendResponse(w, r, summary)
}

func (s *APIServer) handleTransformJQ(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	var req struct {
		Key          string            `json:"key,omitempty"`
		Prefix       string            `json:"prefix,omitempty"`
		JQExpression string            `json:"jq_expression"`
		Options      *TransformOptions `json:"options,omitempty"`
	}

	if err := s.parseJSONBody(r, &req); err != nil {
		s.sendErrorResponse(w, r, "Неверный JSON", http.StatusBadRequest)
		return
	}

	if req.JQExpression == "" {
		s.sendErrorResponse(w, r, "Требуется JQ выражение", http.StatusBadRequest)
		return
	}

	var key ds.Key
	if req.Key != "" {
		key = ds.NewKey(req.Key)
	}

	if req.Prefix != "" && req.Options != nil {
		req.Options.Prefix = ds.NewKey(req.Prefix)
	}

	summary, err := s.ds.TransformWithJQ(ctx, key, req.JQExpression, req.Options)
	if err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка JQ трансформации: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendResponse(w, r, summary)
}

func (s *APIServer) handleTransformPatch(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	var req struct {
		Key      string            `json:"key,omitempty"`
		Prefix   string            `json:"prefix,omitempty"`
		PatchOps []PatchOp         `json:"patch_ops"`
		Options  *TransformOptions `json:"options,omitempty"`
	}

	if err := s.parseJSONBody(r, &req); err != nil {
		s.sendErrorResponse(w, r, "Неверный JSON", http.StatusBadRequest)
		return
	}

	if len(req.PatchOps) == 0 {
		s.sendErrorResponse(w, r, "Требуются patch операции", http.StatusBadRequest)
		return
	}

	var key ds.Key
	if req.Key != "" {
		key = ds.NewKey(req.Key)
	}

	if req.Prefix != "" && req.Options != nil {
		req.Options.Prefix = ds.NewKey(req.Prefix)
	}

	summary, err := s.ds.TransformWithPatch(ctx, key, req.PatchOps, req.Options)
	if err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка patch трансформации: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendResponse(w, r, summary)
}

// TTL handlers

func (s *APIServer) handleTTLStats(w http.ResponseWriter, r *http.Request) {
	// ctx, cancel := s.getContextWithTimeout(r)
	// defer cancel()

	prefix := r.URL.Query().Get("prefix")
	if prefix == "" {
		prefix = "/"
	}

	// TODO: Implement GetTTLStats method
	// stats, err := s.ds.GetTTLStats(ctx, ds.NewKey(prefix))
	// if err != nil {
	// 	s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка получения TTL статистики: %v", err), http.StatusInternalServerError)
	// 	return
	// }

	// s.sendResponse(w, r, stats)
	s.sendErrorResponse(w, r, "TTL статистика не реализована", http.StatusNotImplemented)
}

func (s *APIServer) handleTTLKeys(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement ListTTLKeys method
	s.sendErrorResponse(w, r, "Список TTL ключей не реализован", http.StatusNotImplemented)
}

func (s *APIServer) handleTTLCleanup(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement CleanupExpiredKeys method
	s.sendErrorResponse(w, r, "Очистка TTL ключей не реализована", http.StatusNotImplemented)
}

func (s *APIServer) handleExtendTTL(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement ExtendTTL method
	s.sendErrorResponse(w, r, "Продление TTL не реализовано", http.StatusNotImplemented)
}

func (s *APIServer) handleRefreshTTL(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement RefreshTTL method
	s.sendErrorResponse(w, r, "Обновление TTL не реализовано", http.StatusNotImplemented)
}

// Streaming handlers

func (s *APIServer) handleStream(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	format := StreamFormat(r.URL.Query().Get("format"))
	if format == "" {
		format = StreamFormatJSON
	}

	prefix := r.URL.Query().Get("prefix")
	if prefix == "" {
		prefix = "/"
	}

	jqFilter := r.URL.Query().Get("jq")
	includeKeys := r.URL.Query().Get("include_keys") == "true"

	limit := 0
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil {
			limit = l
		}
	}

	opts := &StreamOptions{
		Format:      format,
		Prefix:      ds.NewKey(prefix),
		JQFilter:    jqFilter,
		IncludeKeys: includeKeys,
		Limit:       limit,
	}

	// Устанавливаем подходящий Content-Type
	switch format {
	case StreamFormatJSON:
		w.Header().Set("Content-Type", "application/json")
	case StreamFormatJSONL:
		w.Header().Set("Content-Type", "application/x-jsonlines")
	case StreamFormatCSV:
		w.Header().Set("Content-Type", "text/csv")
	case StreamFormatSSE:
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}

	if err := s.ds.StreamTo(ctx, w, opts); err != nil {
		s.logger.Printf("Ошибка стрима: %v", err)
	}
}

func (s *APIServer) handleStreamEvents(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	opts := &StreamOptions{
		Format:     StreamFormatSSE,
		BufferSize: 100,
	}

	if err := s.ds.StreamEvents(ctx, w, opts); err != nil {
		s.logger.Printf("Ошибка стрима событий: %v", err)
	}
}

func (s *APIServer) handleStreamJSON(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	prefix := r.URL.Query().Get("prefix")
	if prefix == "" {
		prefix = "/"
	}
	includeKeys := r.URL.Query().Get("include_keys") == "true"

	w.Header().Set("Content-Type", "application/json")

	if err := s.ds.StreamJSON(ctx, w, ds.NewKey(prefix), includeKeys); err != nil {
		s.logger.Printf("Ошибка JSON стрима: %v", err)
	}
}

func (s *APIServer) handleStreamJSONL(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	prefix := r.URL.Query().Get("prefix")
	if prefix == "" {
		prefix = "/"
	}
	includeKeys := r.URL.Query().Get("include_keys") == "true"

	w.Header().Set("Content-Type", "application/x-jsonlines")

	if err := s.ds.StreamJSONL(ctx, w, ds.NewKey(prefix), includeKeys); err != nil {
		s.logger.Printf("Ошибка JSONL стрима: %v", err)
	}
}

func (s *APIServer) handleStreamCSV(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	prefix := r.URL.Query().Get("prefix")
	if prefix == "" {
		prefix = "/"
	}
	includeKeys := r.URL.Query().Get("include_keys") == "true"

	w.Header().Set("Content-Type", "text/csv")

	if err := s.ds.StreamCSV(ctx, w, ds.NewKey(prefix), includeKeys); err != nil {
		s.logger.Printf("Ошибка CSV стрима: %v", err)
	}
}

func (s *APIServer) handleStreamSSE(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	headers := map[string]string{
		"Access-Control-Allow-Origin": "*",
	}

	if err := s.ds.StreamSSE(ctx, w, headers); err != nil {
		s.logger.Printf("Ошибка SSE стрима: %v", err)
	}
}

// Batch operations

func (s *APIServer) handleBatch(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	var req struct {
		Operations []struct {
			Op    string        `json:"op"` // put, delete
			Key   string        `json:"key"`
			Value string        `json:"value,omitempty"`
			TTL   time.Duration `json:"ttl,omitempty"`
		} `json:"operations"`
	}

	if err := s.parseJSONBody(r, &req); err != nil {
		s.sendErrorResponse(w, r, "Неверный JSON", http.StatusBadRequest)
		return
	}

	if len(req.Operations) == 0 {
		s.sendErrorResponse(w, r, "Требуются операции", http.StatusBadRequest)
		return
	}

	batch, err := s.ds.Batch(ctx)
	if err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка создания batch: %v", err), http.StatusInternalServerError)
		return
	}

	for _, op := range req.Operations {
		key := ds.NewKey(op.Key)
		switch op.Op {
		case "put":
			if err := batch.Put(ctx, key, []byte(op.Value)); err != nil {
				s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка добавления в batch: %v", err), http.StatusInternalServerError)
				return
			}
		case "delete":
			if err := batch.Delete(ctx, key); err != nil {
				s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка добавления в batch: %v", err), http.StatusInternalServerError)
				return
			}
		default:
			s.sendErrorResponse(w, r, fmt.Sprintf("Неизвестная операция: %s", op.Op), http.StatusBadRequest)
			return
		}
	}

	if err := batch.Commit(ctx); err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка коммита batch: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendResponseWithMessage(w, r, map[string]interface{}{
		"operations_count": len(req.Operations),
	}, "Batch операции выполнены", http.StatusOK)
}

// Subscription handlers

func (s *APIServer) handleListSubscriptions(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	subscriptions, err := s.ds.ListJSSubscriptions(ctx)
	if err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка получения подписок: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendResponse(w, r, map[string]interface{}{
		"subscriptions": subscriptions,
		"total":         len(subscriptions),
	})
}

func (s *APIServer) handleCreateSubscription(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	var req struct {
		ID               string   `json:"id"`
		Script           string   `json:"script"`
		ExecutionTimeout int      `json:"execution_timeout,omitempty"`
		EnableNetworking bool     `json:"enable_networking,omitempty"`
		EnableLogging    bool     `json:"enable_logging,omitempty"`
		EventFilters     []string `json:"event_filters,omitempty"`
		StrictMode       bool     `json:"strict_mode,omitempty"`
	}

	if err := s.parseJSONBody(r, &req); err != nil {
		s.sendErrorResponse(w, r, "Неверный JSON", http.StatusBadRequest)
		return
	}

	if req.ID == "" || req.Script == "" {
		s.sendErrorResponse(w, r, "Требуются поля ID и Script", http.StatusBadRequest)
		return
	}

	var eventFilters []EventType
	for _, eventStr := range req.EventFilters {
		switch strings.ToLower(eventStr) {
		case "put":
			eventFilters = append(eventFilters, EventPut)
		case "delete":
			eventFilters = append(eventFilters, EventDelete)
		case "batch":
			eventFilters = append(eventFilters, EventBatch)
		case "ttl_expired":
			eventFilters = append(eventFilters, EventTTLExpired)
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

	err := s.ds.CreateJSSubscription(ctx, req.ID, req.Script, config)
	if err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка создания подписки: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendResponseWithMessage(w, r, nil, "Подписка создана", http.StatusCreated)
}

func (s *APIServer) handleDeleteSubscription(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		s.sendErrorResponse(w, r, "Требуется ID подписки", http.StatusBadRequest)
		return
	}

	err := s.ds.RemoveJSSubscription(ctx, id)
	if err != nil {
		s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка удаления подписки: %v", err), http.StatusInternalServerError)
		return
	}

	s.sendResponseWithMessage(w, r, nil, "Подписка удалена", http.StatusOK)
}

// System handlers

func (s *APIServer) handleSetMode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Silent bool `json:"silent"`
	}

	if err := s.parseJSONBody(r, &req); err != nil {
		s.sendErrorResponse(w, r, "Неверный JSON", http.StatusBadRequest)
		return
	}

	s.ds.SetSilentMode(req.Silent)

	mode := "normal"
	if req.Silent {
		mode = "silent"
	}

	s.sendResponseWithMessage(w, r, map[string]interface{}{
		"mode": mode,
	}, "Режим изменен", http.StatusOK)
}

func (s *APIServer) handleGC(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.getContextWithTimeout(r)
	defer cancel()

	if gcds, ok := s.ds.(ds.GCDatastore); ok {
		if err := gcds.CollectGarbage(ctx); err != nil {
			s.sendErrorResponse(w, r, fmt.Sprintf("Ошибка сборки мусора: %v", err), http.StatusInternalServerError)
			return
		}
	}

	s.sendResponseWithMessage(w, r, nil, "Сборка мусора выполнена", http.StatusOK)
}

// Export/Import handlers

func (s *APIServer) handleExport(w http.ResponseWriter, r *http.Request) {
	s.sendErrorResponse(w, r, "Экспорт не реализован", http.StatusNotImplemented)
}

func (s *APIServer) handleImport(w http.ResponseWriter, r *http.Request) {
	s.sendErrorResponse(w, r, "Импорт не реализован", http.StatusNotImplemented)
}

// Documentation handler

func (s *APIServer) handleDocs(w http.ResponseWriter, r *http.Request) {
	docs := `<!DOCTYPE html>
<html>
<head>
    <title>UES Datastore API v1 Documentation</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif; margin: 40px; line-height: 1.6; }
        .endpoint { margin: 20px 0; padding: 20px; border: 1px solid #e0e0e0; border-radius: 8px; background: #fafafa; }
        .method { display: inline-block; padding: 6px 12px; border-radius: 4px; color: white; font-weight: bold; margin-right: 12px; }
        .GET { background: #28a745; }
        .POST { background: #007bff; }
        .PUT { background: #ffc107; color: #212529; }
        .DELETE { background: #dc3545; }
        pre { background: #f8f9fa; padding: 15px; border-radius: 5px; overflow-x: auto; border: 1px solid #e9ecef; }
        .section { margin: 30px 0; }
        h1 { color: #2c3e50; border-bottom: 3px solid #3498db; padding-bottom: 10px; }
        h2 { color: #34495e; margin-top: 40px; }
        .description { color: #666; margin: 10px 0; }
    </style>
</head>
<body>
    <h1>🗄️ UES Datastore REST API v1</h1>
    <p class="description">Полнофункциональный API для работы с датастором, включающий JQ queries, Views, Transform операции и многое другое.</p>

    <div class="section">
        <h2>📊 Basic Operations</h2>
        
        <div class="endpoint">
            <span class="method GET">GET</span><code>/api/v1/health</code>
            <p>Проверка состояния сервера и датастора</p>
        </div>

        <div class="endpoint">
            <span class="method GET">GET</span><code>/api/v1/keys?prefix=/&keys_only=false&limit=100</code>
            <p>Получить список ключей с фильтрацией</p>
        </div>

        <div class="endpoint">
            <span class="method GET">GET</span><code>/api/v1/keys/{key}?format=json</code>
            <p>Получить значение ключа (format: json|raw)</p>
        </div>

        <div class="endpoint">
            <span class="method PUT">PUT</span><code>/api/v1/keys/{key}?ttl=1h</code>
            <p>Установить значение ключа с TTL</p>
        </div>

        <div class="endpoint">
            <span class="method DELETE">DELETE</span><code>/api/v1/keys/{key}</code>
            <p>Удалить ключ</p>
        </div>

        <div class="endpoint">
            <span class="method GET">GET</span><code>/api/v1/keys/{key}/info</code>
            <p>Получить подробную информацию о ключе (размер, TTL, метаданные)</p>
        </div>

        <div class="endpoint">
            <span class="method GET">GET</span><code>/api/v1/stats</code>
            <p>Статистика датастора (количество ключей, размер, и т.д.)</p>
        </div>
    </div>

    <div class="section">
        <h2>🔍 Search & Query</h2>
        
        <div class="endpoint">
            <span class="method POST">POST</span><code>/api/v1/search</code>
            <p>Поиск по ключам</p>
            <pre>{"query": "user", "case_sensitive": false, "limit": 100}</pre>
        </div>

        <div class="endpoint">
            <span class="method POST">POST</span><code>/api/v1/query</code>
            <p>JQ запросы для фильтрации и трансформации данных</p>
            <pre>{"query": ".[] | select(.active == true)", "prefix": "/users/"}</pre>
        </div>

        <div class="endpoint">
            <span class="method POST">POST</span><code>/api/v1/query/aggregate</code>
            <p>JQ агрегация данных</p>
            <pre>{"query": "map(.price) | add", "prefix": "/products/"}</pre>
        </div>

        <div class="endpoint">
            <span class="method POST">POST</span><code>/api/v1/keys/{key}/query</code>
            <p>JQ запрос для одного ключа</p>
            <pre>{"query": ".name + \" (\" + .email + \")\""}</pre>
        </div>
    </div>

    <div class="section">
        <h2>👁️ Views</h2>
        
        <div class="endpoint">
            <span class="method GET">GET</span><code>/api/v1/views</code>
            <p>Список всех view</p>
        </div>

        <div class="endpoint">
            <span class="method POST">POST</span><code>/api/v1/views</code>
            <p>Создать новый view</p>
            <pre>{
  "id": "active_users",
  "name": "Active Users",
  "source_prefix": "/users/",
  "filter_script": "data.json && data.json.active === true",
  "enable_caching": true,
  "auto_refresh": true
}</pre>
        </div>

        <div class="endpoint">
            <span class="method GET">GET</span><code>/api/v1/views/{id}</code>
            <p>Получить конфигурацию view</p>
        </div>

        <div class="endpoint">
            <span class="method POST">POST</span><code>/api/v1/views/{id}/execute</code>
            <p>Выполнить view и получить результаты</p>
        </div>

        <div class="endpoint">
            <span class="method POST">POST</span><code>/api/v1/views/{id}/refresh</code>
            <p>Принудительно обновить view</p>
        </div>
    </div>

    <div class="section">
        <h2>🔄 Transform Operations</h2>
        
        <div class="endpoint">
            <span class="method POST">POST</span><code>/api/v1/transform/jq</code>
            <p>Трансформация данных с помощью JQ</p>
            <pre>{
  "prefix": "/users/",
  "jq_expression": ".name = (.first_name + \" \" + .last_name) | del(.first_name, .last_name)",
  "options": {"dry_run": false}
}</pre>
        </div>

        <div class="endpoint">
            <span class="method POST">POST</span><code>/api/v1/transform/patch</code>
            <p>JSON Patch операции</p>
            <pre>{
  "key": "/user/123",
  "patch_ops": [{"op": "replace", "path": "/status", "value": "active"}]
}</pre>
        </div>
    </div>

    <div class="section">
        <h2>📡 Streaming</h2>
        
        <div class="endpoint">
            <span class="method GET">GET</span><code>/api/v1/stream?format=json&prefix=/users/&include_keys=true</code>
            <p>Стрим данных в различных форматах (json, jsonl, csv, sse)</p>
        </div>

        <div class="endpoint">
            <span class="method GET">GET</span><code>/api/v1/stream/events</code>
            <p>Server-Sent Events для реального времени</p>
        </div>

        <div class="endpoint">
            <span class="method GET">GET</span><code>/api/v1/stream/sse</code>
            <p>SSE поток событий датастора</p>
        </div>
    </div>

    <div class="section">
        <h2>📦 Batch Operations</h2>
        
        <div class="endpoint">
            <span class="method POST">POST</span><code>/api/v1/batch</code>
            <p>Выполнить несколько операций атомарно</p>
            <pre>{
  "operations": [
    {"op": "put", "key": "/user/1", "value": "{\"name\":\"John\"}"},
    {"op": "delete", "key": "/temp/old_data"}
  ]
}</pre>
        </div>
    </div>

    <div class="section">
        <h2>🔔 Subscriptions</h2>
        
        <div class="endpoint">
            <span class="method GET">GET</span><code>/api/v1/subscriptions</code>
            <p>Список JavaScript подписок на события</p>
        </div>

        <div class="endpoint">
            <span class="method POST">POST</span><code>/api/v1/subscriptions</code>
            <p>Создать JavaScript подписку</p>
            <pre>{
  "id": "logger",
  "script": "console.log('Event:', event.type, event.key);",
  "enable_logging": true,
  "event_filters": ["put", "delete"]
}</pre>
        </div>
    </div>

    <div class="section">
        <h2>⚙️ System</h2>
        
        <div class="endpoint">
            <span class="method DELETE">DELETE</span><code>/api/v1/clear?confirm=true</code>
            <p>Очистить весь датастор</p>
        </div>

        <div class="endpoint">
            <span class="method POST">POST</span><code>/api/v1/system/mode</code>
            <p>Установить режим работы</p>
            <pre>{"silent": true}</pre>
        </div>

        <div class="endpoint">
            <span class="method POST">POST</span><code>/api/v1/system/gc</code>
            <p>Запустить сборку мусора</p>
        </div>
    </div>

    <div class="section">
        <h2>📈 Monitoring</h2>
        
        <div class="endpoint">
            <span class="method GET">GET</span><code>/metrics</code>
            <p>Prometheus метрики</p>
        </div>
    </div>

    <div class="section">
        <h2>🔐 Authentication</h2>
        <p>Если включена аутентификация, добавьте заголовок:</p>
        <pre>Authorization: Bearer YOUR_TOKEN</pre>
    </div>

    <div class="section">
        <h2>📝 Notes</h2>
        <ul>
            <li>Все ответы содержат <code>request_id</code> для трейсинга</li>
            <li>Поддерживается CORS для cross-origin запросов</li>
            <li>Есть rate limiting и request timeout</li>
            <li>Graceful shutdown при получении SIGTERM</li>
            <li>Структурированное логирование</li>
            <li>Compression для больших ответов</li>
        </ul>
    </div>

</body>
</html>`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(docs))
}

// metricsUpdater обновляет метрики
func (s *APIServer) metricsUpdater(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Обновляем метрики датастора
			ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)

			keysChan, errChan, err := s.ds.Keys(ctxTimeout, ds.NewKey("/"))
			if err != nil {
				cancel()
				continue
			}

			totalKeys := 0
			for {
				select {
				case _, ok := <-keysChan:
					if !ok {
						goto keysCountDone
					}
					totalKeys++
				case <-errChan:
					goto keysCountDone
				case <-ctxTimeout.Done():
					goto keysCountDone
				}
			}

		keysCountDone:
			s.metrics.DatastoreKeys.Set(float64(totalKeys))
			cancel()
		}
	}
}

// Utility functions

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
