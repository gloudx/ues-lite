package datastore

import (
	"context"
	"fmt"
	"strings"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/itchyny/gojq"
)

// JQQueryBuilder помогает строить сложные jq-запросы
type JQQueryBuilder struct {
	filters    []string
	selectors  []string
	transforms []string
	groups     []string
	sorts      []string
}

func NewJQQueryBuilder() *JQQueryBuilder {
	return &JQQueryBuilder{}
}

func (b *JQQueryBuilder) Filter(condition string) *JQQueryBuilder {
	b.filters = append(b.filters, fmt.Sprintf("select(%s)", condition))
	return b
}

func (b *JQQueryBuilder) Select(fields ...string) *JQQueryBuilder {
	if len(fields) == 1 {
		b.selectors = append(b.selectors, fields[0])
	} else {
		selector := "{" + strings.Join(fields, ", ") + "}"
		b.selectors = append(b.selectors, selector)
	}
	return b
}

func (b *JQQueryBuilder) Transform(expression string) *JQQueryBuilder {
	b.transforms = append(b.transforms, expression)
	return b
}

func (b *JQQueryBuilder) GroupBy(field string) *JQQueryBuilder {
	b.groups = append(b.groups, fmt.Sprintf("group_by(.%s)", field))
	return b
}

func (b *JQQueryBuilder) SortBy(field string, descending bool) *JQQueryBuilder {
	if descending {
		b.sorts = append(b.sorts, fmt.Sprintf("sort_by(.%s) | reverse", field))
	} else {
		b.sorts = append(b.sorts, fmt.Sprintf("sort_by(.%s)", field))
	}
	return b
}

func (b *JQQueryBuilder) Build() string {
	var parts []string

	// Добавляем фильтры
	parts = append(parts, b.filters...)

	// Добавляем трансформации
	parts = append(parts, b.transforms...)

	// Добавляем селекторы
	parts = append(parts, b.selectors...)

	// Добавляем группировки
	parts = append(parts, b.groups...)

	// Добавляем сортировки
	parts = append(parts, b.sorts...)

	if len(parts) == 0 {
		return "."
	}

	return strings.Join(parts, " | ")
}

// JQHelper содержит часто используемые jq-запросы
type JQHelper struct{}

func NewJQHelper() *JQHelper {
	return &JQHelper{}
}

// Готовые запросы для общих случаев
func (h *JQHelper) FilterByField(field string, operator string, value interface{}) string {
	switch v := value.(type) {
	case string:
		return fmt.Sprintf(`select(.%s %s "%s")`, field, operator, v)
	case int, int64, float64:
		return fmt.Sprintf(`select(.%s %s %v)`, field, operator, v)
	default:
		return fmt.Sprintf(`select(.%s %s %v)`, field, operator, v)
	}
}

func (h *JQHelper) SelectFields(fields ...string) string {
	return "{" + strings.Join(fields, ", ") + "}"
}

func (h *JQHelper) CountBy(field string) string {
	return fmt.Sprintf("group_by(.%s) | map({key: .[0].%s, count: length})", field, field)
}

func (h *JQHelper) SumBy(field string) string {
	return fmt.Sprintf("map(.%s) | add", field)
}

func (h *JQHelper) AvgBy(field string) string {
	return fmt.Sprintf("map(.%s) | add / length", field)
}

func (h *JQHelper) MinMaxBy(field string) string {
	return fmt.Sprintf("map(.%s) | {min: min, max: max, avg: (add / length)}", field)
}

// JQPaginator для пагинации результатов jq-запросов
type JQPaginator struct {
	store    Datastore
	query    string
	opts     *JQQueryOptions
	pageSize int
	offset   int
}

func NewJQPaginator(store Datastore, query string, opts *JQQueryOptions, pageSize int) *JQPaginator {
	if opts == nil {
		opts = &JQQueryOptions{
			Prefix: ds.NewKey("/"),
		}
	}

	return &JQPaginator{
		store:    store,
		query:    query,
		opts:     opts,
		pageSize: pageSize,
		offset:   0,
	}
}

func (p *JQPaginator) NextPage(ctx context.Context) ([]JQResult, bool, error) {
	// Модифицируем запрос для пагинации
	paginatedQuery := fmt.Sprintf("[%s] | .[] | select(. != null) | . as $item | %d as $offset | %d as $limit | if $offset <= %d and %d < ($offset + $limit) then $item else empty end",
		p.query, p.offset, p.pageSize, p.offset, p.offset)

	resultChan, errorChan, err := p.store.QueryJQ(ctx, paginatedQuery, p.opts)
	if err != nil {
		return nil, false, err
	}

	var results []JQResult
	count := 0

	for {
		select {
		case <-ctx.Done():
			return nil, false, ctx.Err()
		case err, ok := <-errorChan:
			if ok && err != nil {
				return nil, false, err
			}
		case result, ok := <-resultChan:
			if !ok {
				hasMore := count == p.pageSize
				p.offset += count
				return results, hasMore, nil
			}

			results = append(results, result)
			count++

			if count >= p.pageSize {
				hasMore := true
				p.offset += count
				return results, hasMore, nil
			}
		}
	}
}

func (p *JQPaginator) Reset() {
	p.offset = 0
}

// JQAnalyzer для анализа производительности jq-запросов
type JQAnalyzer struct {
	store Datastore
}

func NewJQAnalyzer(store Datastore) *JQAnalyzer {
	return &JQAnalyzer{store: store}
}

type QueryStats struct {
	Query         string        `json:"query"`
	ExecutionTime time.Duration `json:"execution_time"`
	ResultCount   int           `json:"result_count"`
	ErrorCount    int           `json:"error_count"`
	AvgItemTime   time.Duration `json:"avg_item_time"`
}

func (a *JQAnalyzer) AnalyzeQuery(ctx context.Context, query string, opts *JQQueryOptions) (*QueryStats, error) {
	start := time.Now()

	resultChan, errorChan, err := a.store.QueryJQ(ctx, query, opts)
	if err != nil {
		return nil, err
	}

	stats := &QueryStats{
		Query: query,
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case err, ok := <-errorChan:
			if ok && err != nil {
				stats.ErrorCount++
			} else if !ok {
				// Канал закрыт, завершаем
				stats.ExecutionTime = time.Since(start)
				if stats.ResultCount > 0 {
					stats.AvgItemTime = stats.ExecutionTime / time.Duration(stats.ResultCount)
				}
				return stats, nil
			}
		case _, ok := <-resultChan:
			if !ok {
				// Канал закрыт, завершаем
				stats.ExecutionTime = time.Since(start)
				if stats.ResultCount > 0 {
					stats.AvgItemTime = stats.ExecutionTime / time.Duration(stats.ResultCount)
				}
				return stats, nil
			}
			stats.ResultCount++
		}
	}
}

// Расширенные примеры использования
func ExampleAdvancedQueries() {
	// Пример 1: Построитель запросов
	builder := NewJQQueryBuilder()
	query := builder.
		Filter(".age > 25").
		Filter(".salary > 50000").
		Select("name", "age", "salary").
		SortBy("salary", true).
		Build()

	fmt.Println("Построенный запрос:", query)

	// Пример 2: Использование хелпера
	helper := NewJQHelper()

	queries := []string{
		helper.FilterByField("department", "==", "IT"),
		helper.SelectFields("name", "salary"),
		helper.CountBy("department"),
		helper.AvgBy("salary"),
		helper.MinMaxBy("age"),
	}

	for i, q := range queries {
		fmt.Printf("Запрос %d: %s\n", i+1, q)
	}
}

// Вспомогательные функции для валидации jq-запросов
func ValidateJQQuery(query string) error {
	_, err := gojq.Parse(query)
	if err != nil {
		return fmt.Errorf("невалидный jq-запрос: %w", err)
	}
	return nil
}

func OptimizeJQQuery(query string) (string, error) {
	// Базовые оптимизации для jq-запросов
	optimized := strings.TrimSpace(query)

	// Удаляем избыточные скобки
	if strings.HasPrefix(optimized, "(") && strings.HasSuffix(optimized, ")") {
		inner := optimized[1 : len(optimized)-1]
		if ValidateJQQuery(inner) == nil {
			optimized = inner
		}
	}

	// Проверяем валидность оптимизированного запроса
	if err := ValidateJQQuery(optimized); err != nil {
		return query, err // Возвращаем оригинальный запрос если оптимизация неудачна
	}

	return optimized, nil
}

// Middleware для логирования jq-запросов
type JQQueryLogger struct {
	store  Datastore
	logger func(query string, duration time.Duration, resultCount int, err error)
}

func NewJQQueryLogger(store Datastore, logger func(string, time.Duration, int, error)) *JQQueryLogger {
	return &JQQueryLogger{
		store:  store,
		logger: logger,
	}
}

func (l *JQQueryLogger) QueryJQ(ctx context.Context, jqQuery string, opts *JQQueryOptions) (<-chan JQResult, <-chan error, error) {
	start := time.Now()

	resultChan, errorChan, err := l.store.QueryJQ(ctx, jqQuery, opts)
	if err != nil {
		l.logger(jqQuery, time.Since(start), 0, err)
		return nil, nil, err
	}

	// Оборачиваем каналы для подсчета результатов
	wrappedResultChan := make(chan JQResult)
	wrappedErrorChan := make(chan error, 1)

	go func() {
		defer close(wrappedResultChan)
		defer close(wrappedErrorChan)

		resultCount := 0
		var finalErr error

		for {
			select {
			case <-ctx.Done():
				finalErr = ctx.Err()
				l.logger(jqQuery, time.Since(start), resultCount, finalErr)
				return
			case err, ok := <-errorChan:
				if ok && err != nil {
					finalErr = err
					wrappedErrorChan <- err
				} else if !ok {
					l.logger(jqQuery, time.Since(start), resultCount, finalErr)
					return
				}
			case result, ok := <-resultChan:
				if !ok {
					l.logger(jqQuery, time.Since(start), resultCount, finalErr)
					return
				}
				resultCount++
				wrappedResultChan <- result
			}
		}
	}()

	return wrappedResultChan, wrappedErrorChan, nil
}
