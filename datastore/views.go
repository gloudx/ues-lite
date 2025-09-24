package datastore

import (
	"context"
	"fmt"
	"time"

	ds "github.com/ipfs/go-datastore"
)

// ViewConfig конфигурация для создания view
type ViewConfig struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	Description     string        `json:"description"`
	SourcePrefix    string        `json:"source_prefix"`    // Префикс для исходных данных
	TargetPrefix    string        `json:"target_prefix"`    // Префикс для результатов view
	FilterScript    string        `json:"filter_script"`    // JS функция фильтрации
	TransformScript string        `json:"transform_script"` // JS функция трансформации
	SortScript      string        `json:"sort_script"`      // JS функция сортировки
	StartKey        string        `json:"start_key"`        // Начальный ключ диапазона
	EndKey          string        `json:"end_key"`          // Конечный ключ диапазона
	EnableCaching   bool          `json:"enable_caching"`   // Включить кеширование
	CacheTTL        time.Duration `json:"cache_ttl"`        // TTL для кеша
	AutoRefresh     bool          `json:"auto_refresh"`     // Автоматическое обновление при изменении данных
	RefreshDebounce time.Duration `json:"refresh_debounce"` // Задержка для группировки обновлений
	MaxResults      int           `json:"max_results"`      // Максимальное количество результатов
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

// ViewResult результат выполнения view
type ViewResult struct {
	Key       ds.Key                 `json:"key"`
	Value     interface{}            `json:"value"`
	Score     float64                `json:"score"` // Для сортировки
	Metadata  map[string]interface{} `json:"metadata"`
	Timestamp time.Time              `json:"timestamp"`
}

// ViewStats статистика view
type ViewStats struct {
	ID              string    `json:"id"`
	LastRefresh     time.Time `json:"last_refresh"`
	RefreshCount    int64     `json:"refresh_count"`
	ErrorCount      int64     `json:"error_count"`
	CacheHits       int64     `json:"cache_hits"`
	CacheMisses     int64     `json:"cache_misses"`
	ResultCount     int       `json:"result_count"`
	ExecutionTimeMs int64     `json:"execution_time_ms"`
	LastError       string    `json:"last_error,omitempty"`
}

// View интерфейс для представлений данных
type View interface {
	// ID возвращает уникальный идентификатор view
	ID() string

	// Config возвращает конфигурацию view
	Config() ViewConfig

	// Execute выполняет view и возвращает результаты
	Execute(ctx context.Context) ([]ViewResult, error)

	// ExecuteWithRange выполняет view с ограничением диапазона
	ExecuteWithRange(ctx context.Context, start, end ds.Key) ([]ViewResult, error)

	// Refresh принудительно обновляет кеш view
	Refresh(ctx context.Context) error

	// GetCached возвращает кешированные результаты (если доступны)
	GetCached(ctx context.Context) ([]ViewResult, bool, error)

	// InvalidateCache инвалидирует кеш
	InvalidateCache(ctx context.Context) error

	// Stats возвращает статистику view
	Stats() ViewStats

	// UpdateConfig обновляет конфигурацию view
	UpdateConfig(config ViewConfig) error

	// Close освобождает ресурсы
	Close() error
}

// ViewManager интерфейс для управления view
type ViewManager interface {
	// CreateView создает новый view
	CreateView(ctx context.Context, config ViewConfig) (View, error)

	// GetView возвращает view по ID
	GetView(id string) (View, bool)

	// ListViews возвращает список всех view
	ListViews() []View

	// RemoveView удаляет view
	RemoveView(ctx context.Context, id string) error

	// RefreshView обновляет конкретный view
	RefreshView(ctx context.Context, id string) error

	// RefreshAllViews обновляет все view
	RefreshAllViews(ctx context.Context) error

	// GetViewStats возвращает статистику view
	GetViewStats(id string) (ViewStats, bool)

	// SaveViewConfig сохраняет конфигурацию view в хранилище
	SaveViewConfig(ctx context.Context, config ViewConfig) error

	// LoadViewConfigs загружает все сохраненные конфигурации view
	LoadViewConfigs(ctx context.Context) error
}

// Константы для namespace view
const (
	ViewsNamespace      = "/_system/ds-views"
	ViewsCacheNamespace = "/_system/ds-views-cache"
	ViewsStatsNamespace = "/_system/ds-views-stats"
)

// ViewError специальный тип ошибки для view
type ViewError struct {
	ViewID  string
	Message string
	Err     error
}

func (e *ViewError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("view %s: %s: %v", e.ViewID, e.Message, e.Err)
	}
	return fmt.Sprintf("view %s: %s", e.ViewID, e.Message)
}

func (e *ViewError) Unwrap() error {
	return e.Err
}

// NewViewError создает новую ошибку view
func NewViewError(viewID, message string, err error) *ViewError {
	return &ViewError{
		ViewID:  viewID,
		Message: message,
		Err:     err,
	}
}

// ViewEventType типы событий view
type ViewEventType int

const (
	ViewEventRefresh ViewEventType = iota
	ViewEventCacheHit
	ViewEventCacheMiss
	ViewEventError
)

// ViewEvent событие view
type ViewEvent struct {
	Type      ViewEventType
	ViewID    string
	Message   string
	Timestamp time.Time
	Error     error
	Stats     *ViewStats
}
