package datastore

import (
	"fmt"
	"strings"
	"time"

	ds "github.com/ipfs/go-datastore"
)

// APIHealthResponse ответ на проверку здоровья
type APIHealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Uptime    string    `json:"uptime"`
}

// APIStatsResponse ответ со статистикой
type APIStatsResponse struct {
	TotalKeys        int    `json:"total_keys"`
	TotalSizeBytes   int64  `json:"total_size_bytes"`
	TotalSizeHuman   string `json:"total_size_human"`
	AverageSize      int64  `json:"average_size"`
	AverageSizeHuman string `json:"average_size_human"`
	Timestamp        string `json:"timestamp"`
}

// APIKeyInfo информация о ключе
type APIKeyInfo struct {
	Key         string                 `json:"key"`
	Value       string                 `json:"value"`
	Size        int                    `json:"size"`
	ContentType string                 `json:"content_type"`
	TTL         string                 `json:"ttl,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// APISearchResponse ответ поиска
type APISearchResponse struct {
	Results   []interface{} `json:"results"`
	Found     int           `json:"found"`
	Total     int           `json:"total"`
	Truncated bool          `json:"truncated,omitempty"`
}

// APIListKeysResponse ответ со списком ключей
type APIListKeysResponse struct {
	Keys      []interface{} `json:"keys"`
	Total     int           `json:"total"`
	Truncated bool          `json:"truncated,omitempty"`
}

// APIJQQueryResponse ответ JQ запроса
type APIJQQueryResponse struct {
	Results []map[string]interface{} `json:"results"`
	Total   int                      `json:"total"`
}

// APIJQAggregateResponse ответ JQ агрегации
type APIJQAggregateResponse struct {
	Result interface{} `json:"result"`
}

// APIJQSingleResponse ответ одиночного JQ запроса
type APIJQSingleResponse struct {
	Key    string      `json:"key"`
	Result interface{} `json:"result"`
}

// APIViewsListResponse ответ со списком view
type APIViewsListResponse struct {
	Views []APIViewInfo `json:"views"`
	Total int           `json:"total"`
}

// APIViewInfo информация о view
type APIViewInfo struct {
	ID     string     `json:"id"`
	Name   string     `json:"name"`
	Config ViewConfig `json:"config"`
	Stats  ViewStats  `json:"stats"`
}

// APIViewResponse ответ с view
type APIViewResponse struct {
	ID     string     `json:"id"`
	Config ViewConfig `json:"config"`
	Stats  ViewStats  `json:"stats"`
}

// APIViewExecuteResponse ответ выполнения view
type APIViewExecuteResponse struct {
	Results []ViewResult `json:"results"`
	Total   int          `json:"total"`
}

// APISubscriptionsListResponse ответ со списком подписок
type APISubscriptionsListResponse struct {
	Subscriptions []jsSubscription `json:"subscriptions"`
	Total         int              `json:"total"`
}

// APIBatchResponse ответ batch операции
type APIBatchResponse struct {
	OperationsCount int `json:"operations_count"`
}

// APITransformResponse ответ трансформации
type APITransformResponse struct {
	Summary *TransformSummary `json:",inline"`
}

// APISystemModeResponse ответ смены режима системы
type APISystemModeResponse struct {
	Mode string `json:"mode"`
}

// Дополнительные вспомогательные функции

// NewKey создает новый ключ из строки
func NewKey(s string) ds.Key {
	return ds.NewKey(s)
}

// KeyToString конвертирует ключ в строку
func KeyToString(key ds.Key) string {
	return key.String()
}

// StringToKey конвертирует строку в ключ
func StringToKey(s string) ds.Key {
	return ds.NewKey(s)
}

// ParseStreamFormat парсит формат стрима из строки
func ParseStreamFormat(s string) StreamFormat {
	switch s {
	case "json":
		return StreamFormatJSON
	case "jsonl":
		return StreamFormatJSONL
	case "csv":
		return StreamFormatCSV
	case "sse":
		return StreamFormatSSE
	case "binary":
		return StreamFormatBinary
	case "xml":
		return StreamFormatXML
	case "yaml":
		return StreamFormatYAML
	default:
		return StreamFormatJSON
	}
}

// EventTypeToString конвертирует тип события в строку
func EventTypeToString(eventType EventType) string {
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

// StringToEventType конвертирует строку в тип события
func StringToEventType(s string) EventType {
	switch s {
	case "put":
		return EventPut
	case "delete":
		return EventDelete
	case "batch":
		return EventBatch
	case "ttl_expired":
		return EventTTLExpired
	default:
		return EventPut // дефолтный тип
	}
}

// ValidateViewConfig проверяет корректность конфигурации view
func ValidateViewConfig(config ViewConfig) error {
	if config.ID == "" {
		return fmt.Errorf("view ID не может быть пустым")
	}
	if config.Name == "" {
		return fmt.Errorf("view Name не может быть пустым")
	}
	if config.SourcePrefix == "" {
		return fmt.Errorf("view SourcePrefix не может быть пустым")
	}
	return nil
}

// ValidateJSSubscriberConfig проверяет корректность конфигурации JS подписчика
func ValidateJSSubscriberConfig(config JSSubscriberConfig) error {
	if config.ID == "" {
		return fmt.Errorf("subscriber ID не может быть пустым")
	}
	if config.Script == "" {
		return fmt.Errorf("subscriber Script не может быть пустым")
	}
	if config.ExecutionTimeout <= 0 {
		return fmt.Errorf("ExecutionTimeout должен быть положительным")
	}
	return nil
}

// DefaultJSSubscriberConfig возвращает конфигурацию JS подписчика по умолчанию
func DefaultJSSubscriberConfig(id, script string) *JSSubscriberConfig {
	return &JSSubscriberConfig{
		ID:               id,
		Script:           script,
		ExecutionTimeout: 5 * time.Second,
		EnableNetworking: true,
		EnableLogging:    true,
		StrictMode:       false,
		EventFilters:     []EventType{EventPut, EventDelete},
	}
}

// DefaultStreamOptions возвращает опции стрима по умолчанию
func DefaultStreamOptions() *StreamOptions {
	return &StreamOptions{
		Format:        StreamFormatJSON,
		Prefix:        ds.NewKey("/"),
		BufferSize:    1000,
		IncludeKeys:   true,
		TreatAsString: false,
		IgnoreErrors:  false,
		Timeout:       30 * time.Second,
	}
}

// DefaultTransformOptions возвращает опции трансформации по умолчанию
func DefaultTransformOptions() *TransformOptions {
	return &TransformOptions{
		TreatAsString: false,
		IgnoreErrors:  false,
		DryRun:        false,
		Timeout:       30 * time.Second,
		BatchSize:     100,
	}
}

// DefaultJQQueryOptions возвращает опции JQ запроса по умолчанию
func DefaultJQQueryOptions() *JQQueryOptions {
	return &JQQueryOptions{
		Prefix:           ds.NewKey("/"),
		KeysOnly:         false,
		Limit:            0,
		Timeout:          30 * time.Second,
		TreatAsString:    false,
		IgnoreParseError: false,
	}
}

// DefaultViewConfig возвращает конфигурацию view по умолчанию
func DefaultViewConfig(id, name, sourcePrefix string) ViewConfig {
	return ViewConfig{
		ID:              id,
		Name:            name,
		SourcePrefix:    sourcePrefix,
		EnableCaching:   true,
		CacheTTL:        10 * time.Minute,
		AutoRefresh:     true,
		RefreshDebounce: 1 * time.Second,
		MaxResults:      1000,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
}

// ClientError специальный тип ошибки для клиента
type ClientError struct {
	Message    string
	StatusCode int
	RequestID  string
	Err        error
}

func (e *ClientError) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("API error (request %s): %s (status %d)", e.RequestID, e.Message, e.StatusCode)
	}
	return fmt.Sprintf("API error: %s (status %d)", e.Message, e.StatusCode)
}

func (e *ClientError) Unwrap() error {
	return e.Err
}

// NewClientError создает новую ошибку клиента
func NewClientError(message string, statusCode int, requestID string, err error) *ClientError {
	return &ClientError{
		Message:    message,
		StatusCode: statusCode,
		RequestID:  requestID,
		Err:        err,
	}
}

// IsNotFoundError проверяет, является ли ошибка ошибкой "не найдено"
func IsNotFoundError(err error) bool {
	if clientErr, ok := err.(*ClientError); ok {
		return clientErr.StatusCode == 404
	}
	return err == ds.ErrNotFound
}

// IsUnauthorizedError проверяет, является ли ошибка ошибкой авторизации
func IsUnauthorizedError(err error) bool {
	if clientErr, ok := err.(*ClientError); ok {
		return clientErr.StatusCode == 401
	}
	return false
}

// IsForbiddenError проверяет, является ли ошибка ошибкой доступа
func IsForbiddenError(err error) bool {
	if clientErr, ok := err.(*ClientError); ok {
		return clientErr.StatusCode == 403
	}
	return false
}

// IsTimeoutError проверяет, является ли ошибка таймаутом
func IsTimeoutError(err error) bool {
	if clientErr, ok := err.(*ClientError); ok {
		return clientErr.StatusCode == 408 || clientErr.StatusCode == 504
	}
	return strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "context deadline exceeded")
}

// IsServerError проверяет, является ли ошибка серверной ошибкой
func IsServerError(err error) bool {
	if clientErr, ok := err.(*ClientError); ok {
		return clientErr.StatusCode >= 500
	}
	return false
}

// FormatSize форматирует размер в человекочитаемый формат
func FormatSize(bytes int64) string {
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

// FormatDuration форматирует длительность в человекочитаемый формат
func FormatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%.1fмс", float64(d.Nanoseconds())/1e6)
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fс", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fм", d.Minutes())
	}
	return fmt.Sprintf("%.1fч", d.Hours())
}

// ParseDurationSafe безопасно парсит длительность
func ParseDurationSafe(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return d
}
