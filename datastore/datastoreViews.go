package datastore

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Добавляем методы для работы с view в основной интерфейс Datastore


// Добавляем ViewManager в datastorage
func (s *datastorage) ensureViewManager() {
	if s.viewManager == nil {
		s.viewManager = NewViewManager(s)
	}
}

// ViewManager методы для datastorage
var viewManagerOnce sync.Once

// CreateView создает новый view
func (s *datastorage) CreateView(ctx context.Context, config ViewConfig) (View, error) {
	return s.getViewManager().CreateView(ctx, config)
}

// GetView возвращает view по ID
func (s *datastorage) GetView(id string) (View, bool) {
	return s.getViewManager().GetView(id)
}

// ListViews возвращает список всех view
func (s *datastorage) ListViews() []View {
	return s.getViewManager().ListViews()
}

// RemoveView удаляет view
func (s *datastorage) RemoveView(ctx context.Context, id string) error {
	return s.getViewManager().RemoveView(ctx, id)
}

// RefreshView обновляет конкретный view
func (s *datastorage) RefreshView(ctx context.Context, id string) error {
	return s.getViewManager().RefreshView(ctx, id)
}

// RefreshAllViews обновляет все view
func (s *datastorage) RefreshAllViews(ctx context.Context) error {
	return s.getViewManager().RefreshAllViews(ctx)
}

// GetViewStats возвращает статистику view
func (s *datastorage) GetViewStats(id string) (ViewStats, bool) {
	return s.getViewManager().GetViewStats(id)
}

// SaveViewConfig сохраняет конфигурацию view в хранилище
func (s *datastorage) SaveViewConfig(ctx context.Context, config ViewConfig) error {
	return s.getViewManager().SaveViewConfig(ctx, config)
}

// LoadViewConfigs загружает все сохраненные конфигурации view
func (s *datastorage) LoadViewConfigs(ctx context.Context) error {
	return s.getViewManager().LoadViewConfigs(ctx)
}

// Convenience методы для быстрого создания view

// CreateSimpleView создает простой view с JavaScript скриптом
func (s *datastorage) CreateSimpleView(ctx context.Context, id, name, sourcePrefix, script string) (View, error) {
	config := ViewConfig{
		ID:              id,
		Name:            name,
		SourcePrefix:    sourcePrefix,
		FilterScript:    script,
		EnableCaching:   true,
		AutoRefresh:     true,
		RefreshDebounce: 2 * time.Second,
		CacheTTL:        10 * time.Minute,
	}
	return s.CreateView(ctx, config)
}

// CreateFilteredView создает view с фильтрацией
func (s *datastorage) CreateFilteredView(ctx context.Context, id, name, sourcePrefix, filterScript string) (View, error) {
	config := ViewConfig{
		ID:              id,
		Name:            name,
		SourcePrefix:    sourcePrefix,
		FilterScript:    filterScript,
		EnableCaching:   true,
		AutoRefresh:     true,
		RefreshDebounce: 2 * time.Second,
		CacheTTL:        10 * time.Minute,
	}
	return s.CreateView(ctx, config)
}

// CreateTransformView создает view с трансформацией данных
func (s *datastorage) CreateTransformView(ctx context.Context, id, name, sourcePrefix, transformScript string) (View, error) {
	config := ViewConfig{
		ID:              id,
		Name:            name,
		SourcePrefix:    sourcePrefix,
		TransformScript: transformScript,
		EnableCaching:   true,
		AutoRefresh:     true,
		RefreshDebounce: 2 * time.Second,
		CacheTTL:        10 * time.Minute,
	}
	return s.CreateView(ctx, config)
}

// CreateSortedView создает view с сортировкой
func (s *datastorage) CreateSortedView(ctx context.Context, id, name, sourcePrefix, sortScript string, maxResults int) (View, error) {
	config := ViewConfig{
		ID:              id,
		Name:            name,
		SourcePrefix:    sourcePrefix,
		SortScript:      sortScript,
		MaxResults:      maxResults,
		EnableCaching:   true,
		AutoRefresh:     true,
		RefreshDebounce: 2 * time.Second,
		CacheTTL:        10 * time.Minute,
	}
	return s.CreateView(ctx, config)
}

// ExecuteView выполняет view и возвращает результаты
func (s *datastorage) ExecuteView(ctx context.Context, id string) ([]ViewResult, error) {
	view, exists := s.GetView(id)
	if !exists {
		return nil, fmt.Errorf("view %s not found", id)
	}
	return view.Execute(ctx)
}

// GetViewCached возвращает кешированные результаты view
func (s *datastorage) GetViewCached(ctx context.Context, id string) ([]ViewResult, bool, error) {
	view, exists := s.GetView(id)
	if !exists {
		return nil, false, fmt.Errorf("view %s not found", id)
	}
	return view.GetCached(ctx)
}

// Добавляем поле viewManager в структуру datastorage
// Это нужно добавить в основной файл datastore.go:

// Обновляем метод Close чтобы закрывать ViewManager
func (s *datastorage) closeViewManager() error {
	if s.viewManager != nil {
		if closer, ok := s.viewManager.(*DefaultViewManager); ok {
			return closer.Close()
		}
	}
	return nil
}

// 4. Добавить методы для инициализации ViewManager
func (s *datastorage) initViewManager() {
	// ViewManager будет инициализирован при первом обращении
}

func (s *datastorage) getViewManager() ViewManager {
	s.viewOnce.Do(func() {
		s.viewManager = NewViewManager(s)

		// Загружаем сохраненные конфигурации view при инициализации
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := s.viewManager.LoadViewConfigs(ctx); err != nil {
			log.Printf("ошибка загрузки конфигураций view: %v", err)
		}
	})
	return s.viewManager
}
