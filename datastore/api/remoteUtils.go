package datastore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	ds "github.com/ipfs/go-datastore"
)

// RemoteDatastoreConfig конфигурация для удаленного датастора
type RemoteDatastoreConfig struct {
	Endpoint      string        `json:"endpoint"`
	AuthToken     string        `json:"auth_token,omitempty"`
	Timeout       time.Duration `json:"timeout"`
	RetryAttempts int           `json:"retry_attempts"`
	RetryDelay    time.Duration `json:"retry_delay"`
	UserAgent     string        `json:"user_agent"`
	EnableMetrics bool          `json:"enable_metrics"`
	MaxIdleConns  int           `json:"max_idle_conns"`
	IdleTimeout   time.Duration `json:"idle_timeout"`
}

// DefaultRemoteDatastoreConfig возвращает конфигурацию по умолчанию
func DefaultRemoteDatastoreConfig(endpoint string) *RemoteDatastoreConfig {
	return &RemoteDatastoreConfig{
		Endpoint:      endpoint,
		Timeout:       30 * time.Second,
		RetryAttempts: 3,
		RetryDelay:    1 * time.Second,
		UserAgent:     "ues-datastore-client/1.0",
		EnableMetrics: false,
		MaxIdleConns:  10,
		IdleTimeout:   90 * time.Second,
	}
}

// RemoteDatastoreBuilder строитель для создания удаленного датастора
type RemoteDatastoreBuilder struct {
	config *RemoteDatastoreConfig
}

// NewRemoteDatastoreBuilder создает новый строитель
func NewRemoteDatastoreBuilder(endpoint string) *RemoteDatastoreBuilder {
	return &RemoteDatastoreBuilder{
		config: DefaultRemoteDatastoreConfig(endpoint),
	}
}

// WithAuth устанавливает токен авторизации
func (b *RemoteDatastoreBuilder) WithAuth(token string) *RemoteDatastoreBuilder {
	b.config.AuthToken = token
	return b
}

// WithTimeout устанавливает таймаут
func (b *RemoteDatastoreBuilder) WithTimeout(timeout time.Duration) *RemoteDatastoreBuilder {
	b.config.Timeout = timeout
	return b
}

// WithRetry устанавливает параметры повторных попыток
func (b *RemoteDatastoreBuilder) WithRetry(attempts int, delay time.Duration) *RemoteDatastoreBuilder {
	b.config.RetryAttempts = attempts
	b.config.RetryDelay = delay
	return b
}

// WithUserAgent устанавливает User-Agent
func (b *RemoteDatastoreBuilder) WithUserAgent(userAgent string) *RemoteDatastoreBuilder {
	b.config.UserAgent = userAgent
	return b
}

// EnableMetrics включает метрики
func (b *RemoteDatastoreBuilder) EnableMetrics() *RemoteDatastoreBuilder {
	b.config.EnableMetrics = true
	return b
}

// WithConnectionPool настраивает пул соединений
func (b *RemoteDatastoreBuilder) WithConnectionPool(maxIdle int, idleTimeout time.Duration) *RemoteDatastoreBuilder {
	b.config.MaxIdleConns = maxIdle
	b.config.IdleTimeout = idleTimeout
	return b
}

// Build создает удаленный датастор
func (b *RemoteDatastoreBuilder) Build() (*RemoteDatastoreAdapter, error) {
	// Валидируем конфигурацию
	if err := b.validateConfig(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	// Создаем клиент
	var client *APIClient
	var err error

	if b.config.AuthToken != "" {
		client, err = NewAPIClientWithAuth(b.config.Endpoint, b.config.AuthToken)
	} else {
		client, err = NewAPIClient(b.config.Endpoint)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	// Настраиваем клиент
	client.client.Timeout = b.config.Timeout

	// Проверяем подключение
	if err := client.Health(); err != nil {
		return nil, fmt.Errorf("health check failed: %w", err)
	}

	// Создаем адаптер
	rd := &RemoteDatastore{client: client}
	return &RemoteDatastoreAdapter{RemoteDatastore: rd}, nil
}

// validateConfig валидирует конфигурацию
func (b *RemoteDatastoreBuilder) validateConfig() error {
	if b.config.Endpoint == "" {
		return fmt.Errorf("endpoint cannot be empty")
	}

	// Проверяем формат URL (кроме Unix socket)
	if !strings.HasPrefix(b.config.Endpoint, "unix://") {
		if _, err := url.Parse(b.config.Endpoint); err != nil {
			return fmt.Errorf("invalid endpoint URL: %w", err)
		}
	}

	if b.config.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	if b.config.RetryAttempts < 0 {
		return fmt.Errorf("retry attempts cannot be negative")
	}

	if b.config.RetryDelay < 0 {
		return fmt.Errorf("retry delay cannot be negative")
	}

	return nil
}

// RemoteDatastorePool пул подключений к удаленным датасторам
type RemoteDatastorePool struct {
	adapters map[string]*RemoteDatastoreAdapter
	configs  map[string]*RemoteDatastoreConfig
}

// NewRemoteDatastorePool создает новый пул
func NewRemoteDatastorePool() *RemoteDatastorePool {
	return &RemoteDatastorePool{
		adapters: make(map[string]*RemoteDatastoreAdapter),
		configs:  make(map[string]*RemoteDatastoreConfig),
	}
}

// Add добавляет датастор в пул
func (p *RemoteDatastorePool) Add(name string, config *RemoteDatastoreConfig) error {
	adapter, err := NewRemoteDatastoreBuilder(config.Endpoint).
		WithAuth(config.AuthToken).
		WithTimeout(config.Timeout).
		WithRetry(config.RetryAttempts, config.RetryDelay).
		WithUserAgent(config.UserAgent).
		WithConnectionPool(config.MaxIdleConns, config.IdleTimeout).
		Build()

	if err != nil {
		return fmt.Errorf("failed to create adapter for %s: %w", name, err)
	}

	p.adapters[name] = adapter
	p.configs[name] = config
	return nil
}

// Get получает датастор из пула
func (p *RemoteDatastorePool) Get(name string) (*RemoteDatastoreAdapter, bool) {
	adapter, exists := p.adapters[name]
	return adapter, exists
}

// Remove удаляет датастор из пула
func (p *RemoteDatastorePool) Remove(name string) error {
	if adapter, exists := p.adapters[name]; exists {
		if err := adapter.Close(); err != nil {
			return fmt.Errorf("failed to close adapter %s: %w", name, err)
		}
		delete(p.adapters, name)
		delete(p.configs, name)
	}
	return nil
}

// List возвращает список имен датасторов в пуле
func (p *RemoteDatastorePool) List() []string {
	names := make([]string, 0, len(p.adapters))
	for name := range p.adapters {
		names = append(names, name)
	}
	return names
}

// Close закрывает все датасторы в пуле
func (p *RemoteDatastorePool) Close() error {
	var lastErr error
	for name, adapter := range p.adapters {
		if err := adapter.Close(); err != nil {
			lastErr = fmt.Errorf("failed to close adapter %s: %w", name, err)
		}
	}
	p.adapters = make(map[string]*RemoteDatastoreAdapter)
	p.configs = make(map[string]*RemoteDatastoreConfig)
	return lastErr
}

// HealthCheck проверяет здоровье всех датасторов в пуле
func (p *RemoteDatastorePool) HealthCheck(ctx context.Context) map[string]error {
	results := make(map[string]error)
	for name, adapter := range p.adapters {
		if err := adapter.client.Health(); err != nil {
			results[name] = err
		} else {
			results[name] = nil
		}
	}
	return results
}

// RemoteDatastoreRouter роутер для распределения запросов между датасторами
type RemoteDatastoreRouter struct {
	pool     *RemoteDatastorePool
	strategy RoutingStrategy
}

// RoutingStrategy стратегия маршрутизации
type RoutingStrategy interface {
	Route(key ds.Key, adapters map[string]*RemoteDatastoreAdapter) (*RemoteDatastoreAdapter, error)
}

// RoundRobinStrategy стратегия round-robin
type RoundRobinStrategy struct {
	counter int
}

func NewRoundRobinStrategy() *RoundRobinStrategy {
	return &RoundRobinStrategy{}
}

func (r *RoundRobinStrategy) Route(key ds.Key, adapters map[string]*RemoteDatastoreAdapter) (*RemoteDatastoreAdapter, error) {
	if len(adapters) == 0 {
		return nil, fmt.Errorf("no adapters available")
	}

	names := make([]string, 0, len(adapters))
	for name := range adapters {
		names = append(names, name)
	}

	selectedName := names[r.counter%len(names)]
	r.counter++

	return adapters[selectedName], nil
}

// KeyPrefixStrategy стратегия по префиксу ключа
type KeyPrefixStrategy struct {
	prefixMap map[string]string // префикс -> имя адаптера
	defaultDS string            // адаптер по умолчанию
}

func NewKeyPrefixStrategy(prefixMap map[string]string, defaultDS string) *KeyPrefixStrategy {
	return &KeyPrefixStrategy{
		prefixMap: prefixMap,
		defaultDS: defaultDS,
	}
}

func (k *KeyPrefixStrategy) Route(key ds.Key, adapters map[string]*RemoteDatastoreAdapter) (*RemoteDatastoreAdapter, error) {
	keyStr := key.String()

	// Ищем наиболее длинный совпадающий префикс
	var selectedAdapter string
	maxPrefixLen := 0

	for prefix, adapterName := range k.prefixMap {
		if strings.HasPrefix(keyStr, prefix) && len(prefix) > maxPrefixLen {
			selectedAdapter = adapterName
			maxPrefixLen = len(prefix)
		}
	}

	// Если не найден подходящий префикс, используем адаптер по умолчанию
	if selectedAdapter == "" {
		selectedAdapter = k.defaultDS
	}

	if adapter, exists := adapters[selectedAdapter]; exists {
		return adapter, nil
	}

	return nil, fmt.Errorf("adapter %s not found", selectedAdapter)
}

// HashStrategy стратегия по хешу ключа
type HashStrategy struct {
	hasher func(string) uint32
}

func NewHashStrategy(hasher func(string) uint32) *HashStrategy {
	if hasher == nil {
		hasher = defaultHasher
	}
	return &HashStrategy{hasher: hasher}
}

func (h *HashStrategy) Route(key ds.Key, adapters map[string]*RemoteDatastoreAdapter) (*RemoteDatastoreAdapter, error) {
	if len(adapters) == 0 {
		return nil, fmt.Errorf("no adapters available")
	}

	names := make([]string, 0, len(adapters))
	for name := range adapters {
		names = append(names, name)
	}

	hash := h.hasher(key.String())
	selectedName := names[hash%uint32(len(names))]

	return adapters[selectedName], nil
}

// defaultHasher простая хеш функция
func defaultHasher(s string) uint32 {
	var hash uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		hash ^= uint32(s[i])
		hash *= 16777619
	}
	return hash
}

// NewRemoteDatastoreRouter создает новый роутер
func NewRemoteDatastoreRouter(pool *RemoteDatastorePool, strategy RoutingStrategy) *RemoteDatastoreRouter {
	return &RemoteDatastoreRouter{
		pool:     pool,
		strategy: strategy,
	}
}

// Get получает значение по ключу через роутер
func (r *RemoteDatastoreRouter) Get(ctx context.Context, key ds.Key) ([]byte, error) {
	adapter, err := r.strategy.Route(key, r.pool.adapters)
	if err != nil {
		return nil, err
	}
	return adapter.Get(ctx, key)
}

// Put сохраняет значение по ключу через роутер
func (r *RemoteDatastoreRouter) Put(ctx context.Context, key ds.Key, value []byte) error {
	adapter, err := r.strategy.Route(key, r.pool.adapters)
	if err != nil {
		return err
	}
	return adapter.Put(ctx, key, value)
}

// Delete удаляет значение по ключу через роутер
func (r *RemoteDatastoreRouter) Delete(ctx context.Context, key ds.Key) error {
	adapter, err := r.strategy.Route(key, r.pool.adapters)
	if err != nil {
		return err
	}
	return adapter.Delete(ctx, key)
}

// Has проверяет существование ключа через роутер
func (r *RemoteDatastoreRouter) Has(ctx context.Context, key ds.Key) (bool, error) {
	adapter, err := r.strategy.Route(key, r.pool.adapters)
	if err != nil {
		return false, err
	}
	return adapter.Has(ctx, key)
}

// RemoteDatastoreMetrics метрики для удаленного датастора
type RemoteDatastoreMetrics struct {
	TotalRequests    int64         `json:"total_requests"`
	SuccessfulReqs   int64         `json:"successful_requests"`
	FailedRequests   int64         `json:"failed_requests"`
	AverageLatency   time.Duration `json:"average_latency"`
	TotalDataSent    int64         `json:"total_data_sent"`
	TotalDataRecv    int64         `json:"total_data_received"`
	ConnectionErrors int64         `json:"connection_errors"`
	TimeoutErrors    int64         `json:"timeout_errors"`
}

// RemoteDatastoreMonitor монитор для удаленного датастора
type RemoteDatastoreMonitor struct {
	metrics  *RemoteDatastoreMetrics
	adapter  *RemoteDatastoreAdapter
	interval time.Duration
	stopChan chan struct{}
}

// NewRemoteDatastoreMonitor создает новый монитор
func NewRemoteDatastoreMonitor(adapter *RemoteDatastoreAdapter, interval time.Duration) *RemoteDatastoreMonitor {
	return &RemoteDatastoreMonitor{
		metrics:  &RemoteDatastoreMetrics{},
		adapter:  adapter,
		interval: interval,
		stopChan: make(chan struct{}),
	}
}

// Start запускает мониторинг
func (m *RemoteDatastoreMonitor) Start() {
	go m.monitor()
}

// Stop останавливает мониторинг
func (m *RemoteDatastoreMonitor) Stop() {
	close(m.stopChan)
}

// GetMetrics возвращает текущие метрики
func (m *RemoteDatastoreMonitor) GetMetrics() *RemoteDatastoreMetrics {
	return m.metrics
}

// monitor основной цикл мониторинга
func (m *RemoteDatastoreMonitor) monitor() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.collectMetrics()
		case <-m.stopChan:
			return
		}
	}
}

// collectMetrics собирает метрики
func (m *RemoteDatastoreMonitor) collectMetrics() {
	// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// defer cancel()

	start := time.Now()
	err := m.adapter.client.Health()
	latency := time.Since(start)

	m.metrics.TotalRequests++

	if err != nil {
		m.metrics.FailedRequests++
		if IsTimeoutError(err) {
			m.metrics.TimeoutErrors++
		} else {
			m.metrics.ConnectionErrors++
		}
	} else {
		m.metrics.SuccessfulReqs++
	}

	// Обновляем среднюю латентность
	if m.metrics.TotalRequests > 0 {
		m.metrics.AverageLatency = time.Duration(
			(int64(m.metrics.AverageLatency)*m.metrics.TotalRequests + int64(latency)) / m.metrics.TotalRequests)
	}
}

// RemoteDatastoreLogger логгер для удаленного датастора
type RemoteDatastoreLogger struct {
	adapter *RemoteDatastoreAdapter
	logFunc func(level string, message string, fields map[string]interface{})
}

// NewRemoteDatastoreLogger создает новый логгер
func NewRemoteDatastoreLogger(adapter *RemoteDatastoreAdapter, logFunc func(string, string, map[string]interface{})) *RemoteDatastoreLogger {
	return &RemoteDatastoreLogger{
		adapter: adapter,
		logFunc: logFunc,
	}
}

// LogOperation логирует операцию
func (l *RemoteDatastoreLogger) LogOperation(operation string, key ds.Key, startTime time.Time, err error) {
	duration := time.Since(startTime)
	level := "info"
	if err != nil {
		level = "error"
	}

	fields := map[string]interface{}{
		"operation": operation,
		"key":       key.String(),
		"duration":  duration.String(),
		"success":   err == nil,
	}

	if err != nil {
		fields["error"] = err.Error()
	}

	message := fmt.Sprintf("Remote datastore operation: %s", operation)
	l.logFunc(level, message, fields)
}

// Wrapper методы с логированием

// Get с логированием
func (l *RemoteDatastoreLogger) Get(ctx context.Context, key ds.Key) ([]byte, error) {
	start := time.Now()
	result, err := l.adapter.Get(ctx, key)
	l.LogOperation("get", key, start, err)
	return result, err
}

// Put с логированием
func (l *RemoteDatastoreLogger) Put(ctx context.Context, key ds.Key, value []byte) error {
	start := time.Now()
	err := l.adapter.Put(ctx, key, value)
	l.LogOperation("put", key, start, err)
	return err
}

// Delete с логированием
func (l *RemoteDatastoreLogger) Delete(ctx context.Context, key ds.Key) error {
	start := time.Now()
	err := l.adapter.Delete(ctx, key)
	l.LogOperation("delete", key, start, err)
	return err
}

// Утилиты для работы с конфигурацией

// LoadRemoteDatastoreConfigFromJSON загружает конфигурацию из JSON
func LoadRemoteDatastoreConfigFromJSON(jsonData []byte) (*RemoteDatastoreConfig, error) {
	var config RemoteDatastoreConfig
	if err := json.Unmarshal(jsonData, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return &config, nil
}

// SaveRemoteDatastoreConfigToJSON сохраняет конфигурацию в JSON
func SaveRemoteDatastoreConfigToJSON(config *RemoteDatastoreConfig) ([]byte, error) {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal config: %w", err)
	}
	return data, nil
}

// TestConnection тестирует подключение к удаленному датастору
func TestConnection(endpoint string, authToken string, timeout time.Duration) error {
	// config := &RemoteDatastoreConfig{
	// 	Endpoint:  endpoint,
	// 	AuthToken: authToken,
	// 	Timeout:   timeout,
	// }

	builder := NewRemoteDatastoreBuilder(endpoint)
	if authToken != "" {
		builder = builder.WithAuth(authToken)
	}
	if timeout > 0 {
		builder = builder.WithTimeout(timeout)
	}

	adapter, err := builder.Build()
	if err != nil {
		return fmt.Errorf("failed to create adapter: %w", err)
	}
	defer adapter.Close()

	// Тестируем базовые операции
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	testKey := ds.NewKey("/_test_connection")
	testValue := []byte("test")

	// Тестируем запись
	if err := adapter.Put(ctx, testKey, testValue); err != nil {
		return fmt.Errorf("put test failed: %w", err)
	}

	// Тестируем чтение
	if _, err := adapter.Get(ctx, testKey); err != nil {
		return fmt.Errorf("get test failed: %w", err)
	}

	// Очищаем тестовый ключ
	if err := adapter.Delete(ctx, testKey); err != nil {
		return fmt.Errorf("delete test failed: %w", err)
	}

	return nil
}
