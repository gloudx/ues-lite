package datastore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
)

// DefaultViewManager реализация ViewManager
type DefaultViewManager struct {
	datastore Datastore
	views     map[string]View
	mu        sync.RWMutex
	logger    *log.Logger

	// Канал для обработки событий обновления
	refreshQueue chan string
	refreshDone  chan struct{}
	refreshWg    sync.WaitGroup

	// Подписчик для автоматического обновления
	subscriber *ViewUpdateSubscriber
}

var _ ViewManager = (*DefaultViewManager)(nil)

// NewViewManager создает новый ViewManager
func NewViewManager(ds Datastore) *DefaultViewManager {
	vm := &DefaultViewManager{
		datastore:    ds,
		views:        make(map[string]View),
		logger:       log.New(log.Writer(), "[ViewManager] ", log.LstdFlags),
		refreshQueue: make(chan string, 100),
		refreshDone:  make(chan struct{}),
	}

	// Запускаем обработчик очереди обновлений
	vm.refreshWg.Add(1)
	go vm.refreshProcessor()

	// Создаем подписчика для автоматического обновления
	vm.subscriber = NewViewUpdateSubscriber(vm)
	ds.Subscribe(vm.subscriber)

	return vm
}

func (vm *DefaultViewManager) refreshProcessor() {
	defer vm.refreshWg.Done()

	// Группируем обновления по view ID с дебаунсом
	debounceMap := make(map[string]*time.Timer)

	for {
		select {
		case <-vm.refreshDone:
			return
		case viewID := <-vm.refreshQueue:
			// Отменяем предыдущий таймер, если есть
			if timer, exists := debounceMap[viewID]; exists {
				timer.Stop()
			}

			// Создаем новый таймер с дебаунсом
			debounceMap[viewID] = time.AfterFunc(1*time.Second, func() {
				vm.logger.Printf("Auto-refreshing view: %s", viewID)
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				if err := vm.RefreshView(ctx, viewID); err != nil {
					vm.logger.Printf("Failed to auto-refresh view %s: %v", viewID, err)
				}
				cancel()
				delete(debounceMap, viewID)
			})
		}
	}
}

func (vm *DefaultViewManager) CreateView(ctx context.Context, config ViewConfig) (View, error) {
	if config.ID == "" {
		return nil, fmt.Errorf("view ID cannot be empty")
	}

	// Проверяем, не существует ли уже view с таким ID
	vm.mu.RLock()
	if _, exists := vm.views[config.ID]; exists {
		vm.mu.RUnlock()
		return nil, fmt.Errorf("view with ID %s already exists", config.ID)
	}
	vm.mu.RUnlock()

	// Устанавливаем временные метки
	now := time.Now()
	if config.CreatedAt.IsZero() {
		config.CreatedAt = now
	}
	config.UpdatedAt = now

	// Создаем view
	view, err := NewJSView(vm.datastore, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create view: %w", err)
	}

	// Сохраняем конфигурацию
	if err := vm.SaveViewConfig(ctx, config); err != nil {
		view.Close()
		return nil, fmt.Errorf("failed to save view config: %w", err)
	}

	// Добавляем в память
	vm.mu.Lock()
	vm.views[config.ID] = view
	vm.mu.Unlock()

	vm.logger.Printf("Created view: %s", config.ID)
	return view, nil
}

func (vm *DefaultViewManager) GetView(id string) (View, bool) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	view, exists := vm.views[id]
	return view, exists
}

func (vm *DefaultViewManager) ListViews() []View {
	vm.mu.RLock()
	defer vm.mu.RUnlock()
	views := make([]View, 0, len(vm.views))
	for _, view := range vm.views {
		views = append(views, view)
	}
	return views
}

func (vm *DefaultViewManager) RemoveView(ctx context.Context, id string) error {
	vm.mu.Lock()
	view, exists := vm.views[id]
	if exists {
		delete(vm.views, id)
	}
	vm.mu.Unlock()

	if !exists {
		return fmt.Errorf("view %s not found", id)
	}

	// Закрываем view
	if err := view.Close(); err != nil {
		vm.logger.Printf("Error closing view %s: %v", id, err)
	}

	// Удаляем конфигурацию из хранилища
	configKey := ds.NewKey(ViewsNamespace).ChildString(id)
	if err := vm.datastore.Delete(ctx, configKey); err != nil {
		vm.logger.Printf("Failed to delete view config %s: %v", id, err)
	}

	// Удаляем кеш
	cacheKey := ds.NewKey(ViewsCacheNamespace).ChildString(id)
	if err := vm.datastore.Delete(ctx, cacheKey); err != nil {
		vm.logger.Printf("Failed to delete view cache %s: %v", id, err)
	}

	// Удаляем статистику
	statsKey := ds.NewKey(ViewsStatsNamespace).ChildString(id)
	if err := vm.datastore.Delete(ctx, statsKey); err != nil {
		vm.logger.Printf("Failed to delete view stats %s: %v", id, err)
	}

	vm.logger.Printf("Removed view: %s", id)
	return nil
}

func (vm *DefaultViewManager) RefreshView(ctx context.Context, id string) error {
	view, exists := vm.GetView(id)
	if !exists {
		return fmt.Errorf("view %s not found", id)
	}

	return view.Refresh(ctx)
}

func (vm *DefaultViewManager) RefreshAllViews(ctx context.Context) error {
	views := vm.ListViews()

	var lastErr error
	for _, view := range views {
		if err := view.Refresh(ctx); err != nil {
			vm.logger.Printf("Failed to refresh view %s: %v", view.ID(), err)
			lastErr = err
		}
	}

	return lastErr
}

func (vm *DefaultViewManager) GetViewStats(id string) (ViewStats, bool) {
	view, exists := vm.GetView(id)
	if !exists {
		return ViewStats{}, false
	}

	return view.Stats(), true
}

func (vm *DefaultViewManager) SaveViewConfig(ctx context.Context, config ViewConfig) error {
	data, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal view config: %w", err)
	}

	key := ds.NewKey(ViewsNamespace).ChildString(config.ID)
	return vm.datastore.Put(ctx, key, data)
}

func (vm *DefaultViewManager) LoadViewConfigs(ctx context.Context) error {
	q := query.Query{
		Prefix: ViewsNamespace,
	}

	results, err := vm.datastore.Query(ctx, q)
	if err != nil {
		return fmt.Errorf("failed to query view configs: %w", err)
	}
	defer results.Close()

	loadedCount := 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case result, ok := <-results.Next():
			if !ok {
				vm.logger.Printf("Loaded %d view configurations", loadedCount)
				return nil
			}
			if result.Error != nil {
				vm.logger.Printf("Error reading view config: %v", result.Error)
				continue
			}

			var config ViewConfig
			if err := json.Unmarshal(result.Value, &config); err != nil {
				vm.logger.Printf("Failed to unmarshal view config: %v", err)
				continue
			}

			// Создаем view без сохранения конфигурации (она уже сохранена)
			view, err := NewJSView(vm.datastore, config)
			if err != nil {
				vm.logger.Printf("Failed to recreate view %s: %v", config.ID, err)
				continue
			}

			vm.mu.Lock()
			// Закрываем существующий view, если есть
			if existing, exists := vm.views[config.ID]; exists {
				existing.Close()
			}
			vm.views[config.ID] = view
			vm.mu.Unlock()

			loadedCount++
		}
	}
}

// Close закрывает ViewManager и все view
func (vm *DefaultViewManager) Close() error {
	// Останавливаем обработчик обновлений
	close(vm.refreshDone)
	vm.refreshWg.Wait()

	// Отписываемся от событий
	if vm.subscriber != nil {
		vm.datastore.Unsubscribe(vm.subscriber.ID())
	}

	// Закрываем все view
	vm.mu.Lock()
	defer vm.mu.Unlock()

	for _, view := range vm.views {
		if err := view.Close(); err != nil {
			vm.logger.Printf("Error closing view %s: %v", view.ID(), err)
		}
	}

	vm.views = make(map[string]View)
	return nil
}

// ScheduleRefresh добавляет view в очередь на обновление
func (vm *DefaultViewManager) ScheduleRefresh(viewID string) {
	select {
	case vm.refreshQueue <- viewID:
	default:
		// Очередь переполнена, пропускаем
		vm.logger.Printf("Refresh queue full, skipping refresh for view: %s", viewID)
	}
}

// ViewUpdateSubscriber подписчик для автоматического обновления view
type ViewUpdateSubscriber struct {
	id          string
	viewManager *DefaultViewManager
}

func NewViewUpdateSubscriber(vm *DefaultViewManager) *ViewUpdateSubscriber {
	return &ViewUpdateSubscriber{
		id:          "view-update-subscriber",
		viewManager: vm,
	}
}

func (vus *ViewUpdateSubscriber) ID() string {
	return vus.id
}

func (vus *ViewUpdateSubscriber) OnEvent(ctx context.Context, event Event) {
	// Игнорируем системные события
	if strings.HasPrefix(event.Key.String(), "/_system/") {
		return
	}

	// Проверяем какие view должны быть обновлены
	views := vus.viewManager.ListViews()
	for _, view := range views {
		config := view.Config()

		// Проверяем, влияет ли изменение на этот view
		if config.AutoRefresh && vus.shouldRefresh(event.Key.String(), config.SourcePrefix) {
			vus.viewManager.ScheduleRefresh(view.ID())
		}
	}
}

func (vus *ViewUpdateSubscriber) shouldRefresh(changedKey, sourcePrefix string) bool {
	// Простая проверка префикса
	return strings.HasPrefix(changedKey, sourcePrefix)
}
