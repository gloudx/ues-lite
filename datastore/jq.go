package datastore

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	"github.com/itchyny/gojq"
)

type jqCompiledQuery struct {
	query    *gojq.Query
	original string
	mu       sync.RWMutex
}

func (c *jqCompiledQuery) Execute(input any) (any, error) {

	c.mu.RLock()
	defer c.mu.RUnlock()

	iter := c.query.Run(input)

	var results []any

	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			return nil, err
		}
		results = append(results, v)
	}

	if len(results) == 0 {
		return nil, nil
	}

	if len(results) == 1 {
		return results[0], nil
	}

	return results, nil
}

func (c *jqCompiledQuery) String() string {

	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.original
}

type jqQueryCache struct {
	cache map[string]*jqCompiledQuery
	mu    sync.RWMutex
}

func newJQQueryCache() *jqQueryCache {
	return &jqQueryCache{
		cache: make(map[string]*jqCompiledQuery),
	}
}

func (c *jqQueryCache) get(query string) (*jqCompiledQuery, bool) {

	c.mu.RLock()
	defer c.mu.RUnlock()

	compiled, exists := c.cache[query]

	return compiled, exists
}

func (c *jqQueryCache) set(query string, compiled *jqCompiledQuery) {

	c.mu.Lock()
	defer c.mu.Unlock()

	c.cache[query] = compiled
}

type JQResult struct {
	Key   ds.Key      `json:"key"`
	Value interface{} `json:"value"`
	// Result interface{} `json:"result"`
}

type JQQueryOptions struct {
	Prefix           ds.Key
	KeysOnly         bool
	Limit            int
	Timeout          time.Duration
	TreatAsString    bool // если true, значения трактуются как строки, а не JSON
	IgnoreParseError bool // если true, игнорируем ошибки парсинга JSON
}

func (s *datastorage) initJQCache() {
	if s.jqCache == nil {
		s.jqCache = newJQQueryCache()
	}
}

func (s *datastorage) compileJQ(jqQuery string) (*jqCompiledQuery, error) {

	query, err := gojq.Parse(jqQuery)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга jq-запроса: %w", err)
	}

	compiled := &jqCompiledQuery{
		query:    query,
		original: jqQuery,
	}

	return compiled, nil
}

func (s *datastorage) QueryJQSingle(ctx context.Context, key ds.Key, jqQuery string) (any, error) {

	value, err := s.Datastore.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения значения для ключа %s: %w", key.String(), err)
	}

	s.initJQCache()

	var compiled *jqCompiledQuery
	if cached, exists := s.jqCache.get(jqQuery); exists {
		compiled = cached
	} else {
		compiled, err = s.compileJQ(jqQuery)
		if err != nil {
			return nil, err
		}
		s.jqCache.set(jqQuery, compiled)
	}

	var input any
	if err := json.Unmarshal(value, &input); err != nil {
		input = string(value)
	}

	return compiled.Execute(input)
}

func (s *datastorage) QueryJQ(ctx context.Context, jqQuery string, opts *JQQueryOptions) (<-chan JQResult, <-chan error, error) {

	compiled, err := s.compileJQ(jqQuery)
	if err != nil {
		return nil, nil, err
	}

	return s.queryJQCompiled(ctx, compiled, opts)
}

func (s *datastorage) AggregateJQ(ctx context.Context, jqQuery string, opts *JQQueryOptions) (interface{}, error) {

	resultChan, errorChan, err := s.QueryJQ(ctx, jqQuery, opts)
	if err != nil {
		return nil, err
	}

	var results []interface{}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case err, ok := <-errorChan:
			if ok && err != nil {
				return nil, err
			}

		case result, ok := <-resultChan:

			if !ok {

				if len(results) == 0 {
					return nil, nil
				}

				if len(results) == 1 {
					return results[0], nil
				}

				return results, nil
			}

			results = append(results, result.Value)
		}
	}
}

func (s *datastorage) queryJQCompiled(ctx context.Context, compiled *jqCompiledQuery, opts *JQQueryOptions) (<-chan JQResult, <-chan error, error) {

	if opts == nil {
		opts = &JQQueryOptions{
			Prefix:  ds.NewKey("/"),
			Timeout: 30 * time.Second,
		}
	}

	queryCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		queryCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	resultChan := make(chan JQResult)
	errorChan := make(chan error, 1)

	go func() {

		defer close(resultChan)
		defer close(errorChan)

		q := query.Query{
			Prefix:   opts.Prefix.String(),
			KeysOnly: opts.KeysOnly,
		}

		result, err := s.Datastore.Query(queryCtx, q)
		if err != nil {
			errorChan <- fmt.Errorf("ошибка выполнения базового запроса: %w", err)
			return
		}
		defer result.Close()

		count := 0

		for {

			select {

			case <-queryCtx.Done():
				errorChan <- queryCtx.Err()
				return

			case res, ok := <-result.Next():

				if !ok {
					return
				}

				if res.Error != nil {
					errorChan <- res.Error
					return
				}

				if opts.Limit > 0 && count >= opts.Limit {
					return
				}

				key := ds.NewKey(res.Key)

				if opts.KeysOnly {

					jqResult := JQResult{
						Key:   key,
						Value: nil,
						// Result: nil,
					}

					select {
					case resultChan <- jqResult:
						count++
					case <-queryCtx.Done():
						return
					}

					continue
				}

				var input any
				// var originalValue any

				if opts.TreatAsString {
					input = string(res.Value)
					// originalValue = string(res.Value)
				} else {
					if err := json.Unmarshal(res.Value, &input); err != nil {
						if opts.IgnoreParseError {
							input = string(res.Value)
							// originalValue = string(res.Value)
						} else {
							errorChan <- fmt.Errorf("ошибка парсинга JSON для ключа %s: %w", key.String(), err)
							return
						}
						// } else {
						// 	originalValue = input
					}
				}

				jqResult, err := compiled.Execute(input)
				if err != nil {
					if opts.IgnoreParseError {
						// Пропускаем элементы с ошибками
						continue
					} else {
						errorChan <- fmt.Errorf("ошибка выполнения jq-запроса для ключа %s: %w", key.String(), err)
						return
					}
				}

				result := JQResult{
					Key:   key,
					Value: jqResult,
					// Result: jqResult,
				}

				select {
				case resultChan <- result:
					count++
				case <-queryCtx.Done():
					return
				}
			}
		}
	}()

	return resultChan, errorChan, nil
}
