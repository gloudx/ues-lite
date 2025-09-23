package datastore

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
)

// TransformOptions - опции для трансформации значений
type TransformOptions struct {
	Prefix          ds.Key        // Префикс для поиска ключей (если пустой, то работаем с одним ключом)
	JQExpression    string        // JQ выражение для трансформации
	ExtractPath     string        // Путь для извлечения (JSON path)
	PatchOperations []PatchOp     // Операции патчинга
	TreatAsString   bool          // Трактовать значения как строки
	IgnoreErrors    bool          // Игнорировать ошибки парсинга
	DryRun          bool          // Только показать что будет изменено, не применять
	Timeout         time.Duration // Таймаут операции
	BatchSize       int           // Размер батча для массовых операций
}

// PatchOp - операция патчинга
type PatchOp struct {
	Op    string      `json:"op"`    // "replace", "add", "remove", "copy", "move", "test"
	Path  string      `json:"path"`  // JSON путь
	Value interface{} `json:"value"` // Значение (для операций replace/add)
	From  string      `json:"from"`  // Исходный путь (для copy/move)
}

// TransformResult - результат трансформации
type TransformResult struct {
	Key           ds.Key      `json:"key"`
	OriginalValue interface{} `json:"original_value,omitempty"`
	NewValue      interface{} `json:"new_value"`
	Error         error       `json:"error,omitempty"`
	Skipped       bool        `json:"skipped"`
}

// TransformSummary - сводка по трансформации
type TransformSummary struct {
	TotalProcessed int               `json:"total_processed"`
	Successful     int               `json:"successful"`
	Errors         int               `json:"errors"`
	Skipped        int               `json:"skipped"`
	Results        []TransformResult `json:"results"`
	Duration       time.Duration     `json:"duration"`
}

// TransformWithJQ - трансформирует значения с помощью JQ выражения
func (s *datastorage) TransformWithJQ(ctx context.Context, key ds.Key, jqExpression string, opts *TransformOptions) (*TransformSummary, error) {
	if opts == nil {
		opts = &TransformOptions{}
	}
	opts.JQExpression = jqExpression

	return s.Transform(ctx, key, opts)
}

// TransformWithPatch - трансформирует значения с помощью JSON patch операций
func (s *datastorage) TransformWithPatch(ctx context.Context, key ds.Key, patchOps []PatchOp, opts *TransformOptions) (*TransformSummary, error) {
	if opts == nil {
		opts = &TransformOptions{}
	}
	opts.PatchOperations = patchOps

	return s.Transform(ctx, key, opts)
}

// Transform - основной метод трансформации значений
func (s *datastorage) Transform(ctx context.Context, key ds.Key, opts *TransformOptions) (*TransformSummary, error) {

	// startTime := time.Now()

	// Установка значений по умолчанию
	if opts == nil {
		opts = &TransformOptions{}
	}
	if opts.Timeout == 0 {
		opts.Timeout = 30 * time.Second
	}
	if opts.BatchSize == 0 {
		opts.BatchSize = 100
	}

	// Создаем контекст с таймаутом
	transformCtx, cancel := context.WithTimeout(ctx, opts.Timeout)
	defer cancel()

	summary := &TransformSummary{
		Results: make([]TransformResult, 0),
	}

	// Определяем, работаем ли мы с одним ключом или коллекцией
	if opts.Prefix.String() != "" {
		return s.transformByPrefix(transformCtx, opts.Prefix, opts, summary)
	} else {
		return s.transformSingleKey(transformCtx, key, opts, summary)
	}
}

// transformSingleKey - трансформация одного ключа
func (s *datastorage) transformSingleKey(ctx context.Context, key ds.Key, opts *TransformOptions, summary *TransformSummary) (*TransformSummary, error) {
	result := TransformResult{Key: key}

	// Получаем текущее значение
	originalValue, err := s.Datastore.Get(ctx, key)
	if err != nil {
		result.Error = fmt.Errorf("ошибка получения значения: %w", err)
		summary.Results = append(summary.Results, result)
		summary.Errors++
		summary.TotalProcessed++
		summary.Duration = time.Since(time.Now().Add(-summary.Duration))
		return summary, nil
	}

	// Парсим оригинальное значение
	var originalData interface{}
	if opts.TreatAsString {
		originalData = string(originalValue)
	} else {
		if err := json.Unmarshal(originalValue, &originalData); err != nil {
			if opts.IgnoreErrors {
				originalData = string(originalValue)
			} else {
				result.Error = fmt.Errorf("ошибка парсинга JSON: %w", err)
				summary.Results = append(summary.Results, result)
				summary.Errors++
				summary.TotalProcessed++
				summary.Duration = time.Since(time.Now().Add(-summary.Duration))
				return summary, nil
			}
		}
	}

	result.OriginalValue = originalData

	// Применяем трансформацию
	transformedValue, err := s.applyTransformation(originalData, opts)
	if err != nil {
		result.Error = fmt.Errorf("ошибка трансформации: %w", err)
		summary.Results = append(summary.Results, result)
		summary.Errors++
		summary.TotalProcessed++
		summary.Duration = time.Since(time.Now().Add(-summary.Duration))
		return summary, nil
	}

	result.NewValue = transformedValue

	// Если это не dry run, сохраняем результат
	if !opts.DryRun {
		var newValueBytes []byte
		if strVal, ok := transformedValue.(string); ok && opts.TreatAsString {
			newValueBytes = []byte(strVal)
		} else {
			newValueBytes, err = json.Marshal(transformedValue)
			if err != nil {
				result.Error = fmt.Errorf("ошибка маршалинга результата: %w", err)
				summary.Results = append(summary.Results, result)
				summary.Errors++
				summary.TotalProcessed++
				summary.Duration = time.Since(time.Now().Add(-summary.Duration))
				return summary, nil
			}
		}

		if err := s.Put(ctx, key, newValueBytes); err != nil {
			result.Error = fmt.Errorf("ошибка сохранения: %w", err)
			summary.Results = append(summary.Results, result)
			summary.Errors++
			summary.TotalProcessed++
			summary.Duration = time.Since(time.Now().Add(-summary.Duration))
			return summary, nil
		}
	}

	summary.Results = append(summary.Results, result)
	summary.Successful++
	summary.TotalProcessed++
	summary.Duration = time.Since(time.Now().Add(-summary.Duration))

	return summary, nil
}

// transformByPrefix - трансформация по префиксу
func (s *datastorage) transformByPrefix(ctx context.Context, prefix ds.Key, opts *TransformOptions, summary *TransformSummary) (*TransformSummary, error) {
	// Получаем все ключи по префиксу
	q := query.Query{
		Prefix: prefix.String(),
	}

	results, err := s.Datastore.Query(ctx, q)
	if err != nil {
		return summary, fmt.Errorf("ошибка запроса по префиксу: %w", err)
	}
	defer results.Close()

	// Создаем batch для эффективного сохранения
	var batch ds.Batch
	if !opts.DryRun {
		batch, err = s.Batch(ctx)
		if err != nil {
			return summary, fmt.Errorf("ошибка создания batch: %w", err)
		}
	}

	batchCount := 0

	for {
		select {
		case <-ctx.Done():
			return summary, ctx.Err()

		case res, ok := <-results.Next():
			if !ok {
				// Коммитим последний batch если есть изменения
				if !opts.DryRun && batch != nil && batchCount > 0 {
					if err := batch.Commit(ctx); err != nil {
						return summary, fmt.Errorf("ошибка коммита batch: %w", err)
					}
				}
				summary.Duration = time.Since(time.Now().Add(-summary.Duration))
				return summary, nil
			}

			if res.Error != nil {
				result := TransformResult{
					Key:   ds.NewKey(res.Key),
					Error: res.Error,
				}
				summary.Results = append(summary.Results, result)
				summary.Errors++
				summary.TotalProcessed++
				continue
			}

			result := TransformResult{Key: ds.NewKey(res.Key)}

			// Парсим значение
			var originalData interface{}
			if opts.TreatAsString {
				originalData = string(res.Value)
			} else {
				if err := json.Unmarshal(res.Value, &originalData); err != nil {
					if opts.IgnoreErrors {
						originalData = string(res.Value)
					} else {
						result.Error = fmt.Errorf("ошибка парсинга JSON: %w", err)
						summary.Results = append(summary.Results, result)
						summary.Errors++
						summary.TotalProcessed++
						continue
					}
				}
			}

			result.OriginalValue = originalData

			// Применяем трансформацию
			transformedValue, err := s.applyTransformation(originalData, opts)
			if err != nil {
				if opts.IgnoreErrors {
					result.Skipped = true
					summary.Results = append(summary.Results, result)
					summary.Skipped++
					summary.TotalProcessed++
					continue
				} else {
					result.Error = fmt.Errorf("ошибка трансформации: %w", err)
					summary.Results = append(summary.Results, result)
					summary.Errors++
					summary.TotalProcessed++
					continue
				}
			}

			result.NewValue = transformedValue

			// Сохраняем если это не dry run
			if !opts.DryRun {
				var newValueBytes []byte
				if strVal, ok := transformedValue.(string); ok && opts.TreatAsString {
					newValueBytes = []byte(strVal)
				} else {
					newValueBytes, err = json.Marshal(transformedValue)
					if err != nil {
						result.Error = fmt.Errorf("ошибка маршалинга: %w", err)
						summary.Results = append(summary.Results, result)
						summary.Errors++
						summary.TotalProcessed++
						continue
					}
				}

				if err := batch.Put(ctx, result.Key, newValueBytes); err != nil {
					result.Error = fmt.Errorf("ошибка добавления в batch: %w", err)
					summary.Results = append(summary.Results, result)
					summary.Errors++
					summary.TotalProcessed++
					continue
				}

				batchCount++

				// Коммитим batch если достигли размера батча
				if batchCount >= opts.BatchSize {
					if err := batch.Commit(ctx); err != nil {
						result.Error = fmt.Errorf("ошибка коммита batch: %w", err)
						summary.Results = append(summary.Results, result)
						summary.Errors++
						summary.TotalProcessed++
						continue
					}

					// Создаем новый batch
					batch, err = s.Batch(ctx)
					if err != nil {
						return summary, fmt.Errorf("ошибка создания нового batch: %w", err)
					}
					batchCount = 0
				}
			}

			summary.Results = append(summary.Results, result)
			summary.Successful++
			summary.TotalProcessed++
		}
	}
}

// applyTransformation - применяет трансформацию к данным
func (s *datastorage) applyTransformation(data interface{}, opts *TransformOptions) (interface{}, error) {
	// JQ трансформация
	if opts.JQExpression != "" {
		s.initJQCache()

		compiled, exists := s.jqCache.get(opts.JQExpression)
		if !exists {
			var err error
			compiled, err = s.compileJQ(opts.JQExpression)
			if err != nil {
				return nil, fmt.Errorf("ошибка компиляции JQ: %w", err)
			}
			s.jqCache.set(opts.JQExpression, compiled)
		}

		return compiled.Execute(data)
	}

	// Extract path (простой JSON path)
	if opts.ExtractPath != "" {
		return s.extractPath(data, opts.ExtractPath)
	}

	// JSON Patch операции
	if len(opts.PatchOperations) > 0 {
		return s.applyPatch(data, opts.PatchOperations)
	}

	return data, fmt.Errorf("не указана операция трансформации")
}

// extractPath - извлекает значение по JSON пути
func (s *datastorage) extractPath(data interface{}, path string) (interface{}, error) {
	// Простая реализация JSON path извлечения
	// Для более сложных случаев лучше использовать специальную библиотеку

	// Если путь пустой, возвращаем данные как есть
	if path == "" || path == "." {
		return data, nil
	}

	// Конвертируем в map для навигации
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("данные должны быть объектом для извлечения по пути")
	}

	// Простой парсер пути (без поддержки массивов и сложных выражений)
	keys := splitPath(path)
	current := interface{}(dataMap)

	for _, key := range keys {
		if currentMap, ok := current.(map[string]interface{}); ok {
			if value, exists := currentMap[key]; exists {
				current = value
			} else {
				return nil, fmt.Errorf("ключ %s не найден", key)
			}
		} else {
			return nil, fmt.Errorf("невозможно извлечь ключ %s из не-объекта", key)
		}
	}

	return current, nil
}

// applyPatch - применяет JSON patch операции
func (s *datastorage) applyPatch(data interface{}, patches []PatchOp) (interface{}, error) {
	// Простая реализация JSON patch
	// Для production лучше использовать специализированную библиотеку

	result := data

	for _, patch := range patches {
		var err error
		switch patch.Op {
		case "replace":
			result, err = s.patchReplace(result, patch.Path, patch.Value)
		case "add":
			result, err = s.patchAdd(result, patch.Path, patch.Value)
		case "remove":
			result, err = s.patchRemove(result, patch.Path)
		default:
			return nil, fmt.Errorf("неподдерживаемая patch операция: %s", patch.Op)
		}
		if err != nil {
			return nil, fmt.Errorf("ошибка применения patch %s: %w", patch.Op, err)
		}
	}

	return result, nil
}

// Вспомогательные функции для JSON patch
func (s *datastorage) patchReplace(data interface{}, path string, value interface{}) (interface{}, error) {
	// Простая реализация replace операции
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("replace поддерживается только для объектов")
	}

	keys := splitPath(path)
	if len(keys) == 0 {
		return value, nil
	}

	result := make(map[string]interface{})
	for k, v := range dataMap {
		result[k] = v
	}

	// Для простоты поддерживаем только прямые ключи
	if len(keys) == 1 {
		result[keys[0]] = value
		return result, nil
	}

	return nil, fmt.Errorf("вложенные пути пока не поддерживаются")
}

func (s *datastorage) patchAdd(data interface{}, path string, value interface{}) (interface{}, error) {
	return s.patchReplace(data, path, value) // В простой реализации add = replace
}

func (s *datastorage) patchRemove(data interface{}, path string) (interface{}, error) {
	dataMap, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("remove поддерживается только для объектов")
	}

	keys := splitPath(path)
	if len(keys) != 1 {
		return nil, fmt.Errorf("вложенные пути пока не поддерживаются")
	}

	result := make(map[string]interface{})
	for k, v := range dataMap {
		if k != keys[0] {
			result[k] = v
		}
	}

	return result, nil
}

// splitPath - разбивает путь на компоненты
func splitPath(path string) []string {
	if path == "" {
		return []string{}
	}

	// Убираем ведущий слэш если есть
	if path[0] == '/' {
		path = path[1:]
	}

	if path == "" {
		return []string{}
	}

	// Простое разбиение по точкам или слэшам
	var result []string
	current := ""

	for _, char := range path {
		if char == '.' || char == '/' {
			if current != "" {
				result = append(result, current)
				current = ""
			}
		} else {
			current += string(char)
		}
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}
