package datastore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dop251/goja"
	ds "github.com/ipfs/go-datastore"
)

// JSView реализация View с использованием JavaScript
type JSView struct {
	id        string
	config    ViewConfig
	datastore Datastore
	vm        *goja.Runtime
	stats     ViewStats
	mu        sync.RWMutex
	logger    *log.Logger
}

var _ View = (*JSView)(nil)

// NewJSView создает новый JSView
func NewJSView(ds Datastore, config ViewConfig) (*JSView, error) {
	if config.ID == "" {
		return nil, fmt.Errorf("view ID cannot be empty")
	}

	if config.SourcePrefix == "" {
		return nil, fmt.Errorf("source prefix cannot be empty")
	}

	if config.TargetPrefix == "" {
		config.TargetPrefix = fmt.Sprintf("/views/%s", config.ID)
	}

	if config.CacheTTL <= 0 {
		config.CacheTTL = 10 * time.Minute
	}

	if config.RefreshDebounce <= 0 {
		config.RefreshDebounce = 1 * time.Second
	}

	view := &JSView{
		id:        config.ID,
		config:    config,
		datastore: ds,
		stats: ViewStats{
			ID:          config.ID,
			LastRefresh: time.Now(),
		},
		logger: log.New(log.Writer(), fmt.Sprintf("[JSView-%s] ", config.ID), log.LstdFlags),
	}

	if err := view.initVM(); err != nil {
		return nil, fmt.Errorf("failed to initialize JS runtime: %w", err)
	}

	return view, nil
}

func (jv *JSView) initVM() error {
	jv.mu.Lock()
	defer jv.mu.Unlock()

	vm := goja.New()

	// Настройка безопасности
	vm.Set("require", goja.Undefined())
	vm.Set("module", goja.Undefined())
	vm.Set("exports", goja.Undefined())
	vm.Set("global", goja.Undefined())
	vm.Set("process", goja.Undefined())

	// Добавляем встроенные функции
	jv.setupBuiltins(vm)

	jv.vm = vm
	return nil
}

func (jv *JSView) setupBuiltins(vm *goja.Runtime) {
	// Console для логирования
	console := map[string]interface{}{
		"log": func(args ...interface{}) {
			jv.logger.Println(args...)
		},
		"error": func(args ...interface{}) {
			jv.logger.Printf("ERROR: %v\n", args)
		},
		"info": func(args ...interface{}) {
			jv.logger.Printf("INFO: %v\n", args)
		},
	}
	vm.Set("console", console)

	// JSON утилиты
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

	// String утилиты
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
		"trim": func(s string) string {
			return strings.TrimSpace(s)
		},
	}
	vm.Set("Strings", stringUtils)

	// Math утилиты
	mathUtils := map[string]interface{}{
		"abs": func(x float64) float64 {
			if x < 0 {
				return -x
			}
			return x
		},
		"max": func(a, b float64) float64 {
			if a > b {
				return a
			}
			return b
		},
		"min": func(a, b float64) float64 {
			if a < b {
				return a
			}
			return b
		},
	}
	vm.Set("Math", mathUtils)
}

func (jv *JSView) ID() string {
	return jv.id
}

func (jv *JSView) Config() ViewConfig {
	jv.mu.RLock()
	defer jv.mu.RUnlock()
	return jv.config
}

func (jv *JSView) Execute(ctx context.Context) ([]ViewResult, error) {
	startKey := ds.NewKey(jv.config.SourcePrefix)
	var endKey ds.Key
	if jv.config.EndKey != "" {
		endKey = ds.NewKey(jv.config.EndKey)
	}
	return jv.ExecuteWithRange(ctx, startKey, endKey)
}

func (jv *JSView) ExecuteWithRange(ctx context.Context, start, end ds.Key) ([]ViewResult, error) {
	startTime := time.Now()

	jv.mu.Lock()
	jv.stats.RefreshCount++
	jv.mu.Unlock()

	// Проверяем кеш, если включен
	if jv.config.EnableCaching {
		if cached, found, err := jv.GetCached(ctx); err == nil && found {
			jv.mu.Lock()
			jv.stats.CacheHits++
			jv.mu.Unlock()
			return cached, nil
		}
		jv.mu.Lock()
		jv.stats.CacheMisses++
		jv.mu.Unlock()
	}

	// Получаем исходные данные
	var sourceData []KeyValue

	if end.String() == "" {
		// Используем Iterator с префиксом
		it, errc, err := jv.datastore.Iterator(ctx, start, false)
		if err != nil {
			jv.recordError(fmt.Sprintf("failed to create iterator: %v", err))
			return nil, NewViewError(jv.id, "failed to create iterator", err)
		}

		for {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case err := <-errc:
				if err != nil {
					jv.recordError(fmt.Sprintf("iterator error: %v", err))
					return nil, NewViewError(jv.id, "iterator error", err)
				}
			case kv, ok := <-it:
				if !ok {
					goto processData
				}
				sourceData = append(sourceData, kv)
			}
		}
	} else {
		// Используем диапазон ключей
		keys, errc, err := jv.datastore.Keys(ctx, start)
		if err != nil {
			jv.recordError(fmt.Sprintf("failed to get keys: %v", err))
			return nil, NewViewError(jv.id, "failed to get keys", err)
		}

		for {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case err := <-errc:
				if err != nil {
					jv.recordError(fmt.Sprintf("keys error: %v", err))
					return nil, NewViewError(jv.id, "keys error", err)
				}
			case key, ok := <-keys:
				if !ok {
					goto processData
				}
				if end.String() != "" && key.String() > end.String() {
					continue
				}

				value, err := jv.datastore.Get(ctx, key)
				if err != nil {
					continue // Пропускаем ошибочные ключи
				}
				sourceData = append(sourceData, KeyValue{Key: key, Value: value})
			}
		}
	}

processData:
	// Обрабатываем данные через JavaScript
	results, err := jv.processData(sourceData)
	if err != nil {
		jv.recordError(fmt.Sprintf("data processing error: %v", err))
		return nil, NewViewError(jv.id, "data processing failed", err)
	}

	// Сохраняем в кеш, если включен
	if jv.config.EnableCaching {
		if err := jv.saveToCache(ctx, results); err != nil {
			jv.logger.Printf("Failed to save to cache: %v", err)
		}
	}

	// Обновляем статистику
	jv.mu.Lock()
	jv.stats.LastRefresh = time.Now()
	jv.stats.ResultCount = len(results)
	jv.stats.ExecutionTimeMs = time.Since(startTime).Milliseconds()
	jv.stats.LastError = ""
	jv.mu.Unlock()

	return results, nil
}

func (jv *JSView) processData(sourceData []KeyValue) ([]ViewResult, error) {
	jv.mu.Lock()
	defer jv.mu.Unlock()

	if jv.vm == nil {
		return nil, fmt.Errorf("JS runtime not initialized")
	}

	var results []ViewResult

	for _, kv := range sourceData {
		// Подготавливаем данные для JavaScript
		jsData := map[string]interface{}{
			"key":   kv.Key.String(),
			"value": string(kv.Value),
			"size":  len(kv.Value),
		}

		// Пытаемся разобрать JSON, если возможно
		var jsonValue interface{}
		if err := json.Unmarshal(kv.Value, &jsonValue); err == nil {
			jsData["json"] = jsonValue
		}

		// Устанавливаем данные в VM
		jv.vm.Set("data", jsData)

		// Применяем фильтр, если определен
		if jv.config.FilterScript != "" {
			filterResult, err := jv.vm.RunString(jv.config.FilterScript)
			if err != nil {
				continue // Пропускаем элементы с ошибками фильтрации
			}

			if !filterResult.ToBoolean() {
				continue // Элемент не прошел фильтр
			}
		}

		// Создаем результат
		result := ViewResult{
			Key:       kv.Key,
			Value:     jsonValue,
			Timestamp: time.Now(),
			Metadata:  make(map[string]interface{}),
		}

		// Применяем трансформацию, если определена
		if jv.config.TransformScript != "" {
			transformResult, err := jv.vm.RunString(jv.config.TransformScript)
			if err != nil {
				continue // Пропускаем элементы с ошибками трансформации
			}

			result.Value = transformResult.Export()
		}

		// Применяем сортировку, если определена
		if jv.config.SortScript != "" {
			scoreResult, err := jv.vm.RunString(jv.config.SortScript)
			if err == nil {
				if score, ok := scoreResult.Export().(float64); ok {
					result.Score = score
				}
			}
		}

		results = append(results, result)

		// Проверяем лимит результатов
		if jv.config.MaxResults > 0 && len(results) >= jv.config.MaxResults {
			break
		}
	}

	// Сортируем результаты по score, если есть сортировочный скрипт
	if jv.config.SortScript != "" {
		sort.Slice(results, func(i, j int) bool {
			return results[i].Score > results[j].Score
		})
	}

	return results, nil
}

func (jv *JSView) GetCached(ctx context.Context) ([]ViewResult, bool, error) {
	if !jv.config.EnableCaching {
		return nil, false, nil
	}

	cacheKey := ds.NewKey(ViewsCacheNamespace).ChildString(jv.id)

	// Проверяем TTL
	if jv.config.CacheTTL > 0 {
		if ttlDs, ok := jv.datastore.(ds.TTLDatastore); ok {
			expiry, err := ttlDs.GetExpiration(ctx, cacheKey)
			if err != nil || time.Now().After(expiry) {
				return nil, false, nil
			}
		}
	}

	data, err := jv.datastore.Get(ctx, cacheKey)
	if err != nil {
		return nil, false, nil
	}

	var results []ViewResult
	if err := json.Unmarshal(data, &results); err != nil {
		return nil, false, err
	}

	return results, true, nil
}

func (jv *JSView) saveToCache(ctx context.Context, results []ViewResult) error {
	if !jv.config.EnableCaching {
		return nil
	}

	data, err := json.Marshal(results)
	if err != nil {
		return err
	}

	cacheKey := ds.NewKey(ViewsCacheNamespace).ChildString(jv.id)

	if err := jv.datastore.Put(ctx, cacheKey, data); err != nil {
		return err
	}

	// Устанавливаем TTL, если поддерживается
	if jv.config.CacheTTL > 0 {
		if ttlDs, ok := jv.datastore.(ds.TTLDatastore); ok {
			// expiry := time.Now().Add(jv.config.CacheTTL)
			ttlDs.SetTTL(ctx, cacheKey, jv.config.CacheTTL)
		}
	}

	return nil
}

func (jv *JSView) Refresh(ctx context.Context) error {
	_, err := jv.Execute(ctx)
	return err
}

func (jv *JSView) InvalidateCache(ctx context.Context) error {
	if !jv.config.EnableCaching {
		return nil
	}

	cacheKey := ds.NewKey(ViewsCacheNamespace).ChildString(jv.id)
	return jv.datastore.Delete(ctx, cacheKey)
}

func (jv *JSView) Stats() ViewStats {
	jv.mu.RLock()
	defer jv.mu.RUnlock()
	return jv.stats
}

func (jv *JSView) UpdateConfig(config ViewConfig) error {
	jv.mu.Lock()
	defer jv.mu.Unlock()

	config.ID = jv.id // ID не может быть изменен
	config.UpdatedAt = time.Now()
	jv.config = config

	// Пересоздаем VM с новой конфигурацией
	return jv.initVM()
}

func (jv *JSView) Close() error {
	jv.mu.Lock()
	defer jv.mu.Unlock()

	if jv.vm != nil {
		jv.vm.ClearInterrupt()
		jv.vm = nil
	}

	return nil
}

func (jv *JSView) recordError(message string) {
	jv.mu.Lock()
	defer jv.mu.Unlock()

	jv.stats.ErrorCount++
	jv.stats.LastError = message
	jv.logger.Printf("Error: %s", message)
}
