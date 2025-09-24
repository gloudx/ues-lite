package datastore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
	"ues-lite/datastore"

	ds "github.com/ipfs/go-datastore"
)

// APIClient представляет клиент для работы с REST API
type APIClient struct {
	client  *http.Client
	baseURL string
	isUnix  bool
}

// NewAPIClient создает новый API клиент
func NewAPIClient(endpoint string) (*APIClient, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	baseURL := endpoint
	isUnix := false

	// Проверяем, это Unix socket или HTTP URL
	if strings.HasPrefix(endpoint, "unix://") {
		socketPath := strings.TrimPrefix(endpoint, "unix://")

		// Создаем transport для Unix socket
		client.Transport = &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "unix", socketPath)
			},
		}

		baseURL = "http://unix"
		isUnix = true
	} else if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		baseURL = "http://" + endpoint
	}

	return &APIClient{
		client:  client,
		baseURL: baseURL,
		isUnix:  isUnix,
	}, nil
}

// Вспомогательные методы для HTTP запросов

func (c *APIClient) get(endpoint string) (*APIResponse, error) {
	url := c.baseURL + "/api" + endpoint

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP GET ошибка: %w", err)
	}
	defer resp.Body.Close()

	return c.parseResponse(resp)
}

func (c *APIClient) post(endpoint string, body interface{}) (*APIResponse, error) {
	return c.request("POST", endpoint, body)
}

func (c *APIClient) put(endpoint string, body interface{}) (*APIResponse, error) {
	return c.request("PUT", endpoint, body)
}

func (c *APIClient) delete(endpoint string) (*APIResponse, error) {
	return c.request("DELETE", endpoint, nil)
}

func (c *APIClient) request(method, endpoint string, body interface{}) (*APIResponse, error) {
	url := c.baseURL + "/api" + endpoint

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("ошибка сериализации JSON: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP %s ошибка: %w", method, err)
	}
	defer resp.Body.Close()

	return c.parseResponse(resp)
}

func (c *APIClient) parseResponse(resp *http.Response) (*APIResponse, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ошибка чтения ответа: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("API ошибка: %s", apiResp.Error)
	}

	return &apiResp, nil
}

// Методы для работы с датастором через API

func (c *APIClient) Get(ctx context.Context, key ds.Key) ([]byte, error) {
	endpoint := fmt.Sprintf("/keys%s?format=raw", key.String())

	url := c.baseURL + "/api" + endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "text/plain")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, ds.ErrNotFound
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

func (c *APIClient) Put(ctx context.Context, key ds.Key, value []byte) error {
	endpoint := fmt.Sprintf("/keys%s", key.String())
	url := c.baseURL + "/api" + endpoint

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(value))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "text/plain")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *APIClient) PutWithTTL(ctx context.Context, key ds.Key, value []byte, ttl time.Duration) error {
	endpoint := fmt.Sprintf("/keys%s?ttl=%s", key.String(), ttl.String())
	url := c.baseURL + "/api" + endpoint

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(value))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "text/plain")

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *APIClient) Delete(ctx context.Context, key ds.Key) error {
	endpoint := fmt.Sprintf("/keys%s", key.String())

	apiResp, err := c.delete(endpoint)
	if err != nil {
		return err
	}

	_ = apiResp // Игнорируем успешный ответ
	return nil
}

func (c *APIClient) Has(ctx context.Context, key ds.Key) (bool, error) {
	endpoint := fmt.Sprintf("/keys%s/info", key.String())

	_, err := c.get(endpoint)
	if err != nil {
		if strings.Contains(err.Error(), "Ключ не найден") {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (c *APIClient) GetSize(ctx context.Context, key ds.Key) (int, error) {
	endpoint := fmt.Sprintf("/keys%s/info", key.String())

	apiResp, err := c.get(endpoint)
	if err != nil {
		return 0, err
	}

	// Извлекаем размер из ответа
	if data, ok := apiResp.Data.(map[string]interface{}); ok {
		if size, ok := data["size"].(float64); ok {
			return int(size), nil
		}
	}

	return 0, fmt.Errorf("неожиданный формат ответа")
}

func (c *APIClient) ListKeys(ctx context.Context, prefix string, keysOnly bool, limit int) ([]interface{}, error) {
	endpoint := fmt.Sprintf("/keys?prefix=%s&keys_only=%t", prefix, keysOnly)
	if limit > 0 {
		endpoint += fmt.Sprintf("&limit=%d", limit)
	}

	apiResp, err := c.get(endpoint)
	if err != nil {
		return nil, err
	}

	if data, ok := apiResp.Data.(map[string]interface{}); ok {
		if keys, ok := data["keys"].([]interface{}); ok {
			return keys, nil
		}
	}

	return nil, fmt.Errorf("неожиданный формат ответа")
}

func (c *APIClient) Search(ctx context.Context, query string, caseSensitive, keysOnly bool, limit int) ([]interface{}, error) {
	reqBody := SearchRequest{
		Query:         query,
		CaseSensitive: caseSensitive,
		KeysOnly:      keysOnly,
		Limit:         limit,
	}

	apiResp, err := c.post("/search", reqBody)
	if err != nil {
		return nil, err
	}

	if data, ok := apiResp.Data.(map[string]interface{}); ok {
		if results, ok := data["results"].([]interface{}); ok {
			return results, nil
		}
	}

	return nil, fmt.Errorf("неожиданный формат ответа")
}

func (c *APIClient) GetStats(ctx context.Context) (map[string]interface{}, error) {
	apiResp, err := c.get("/stats")
	if err != nil {
		return nil, err
	}

	if stats, ok := apiResp.Data.(map[string]interface{}); ok {
		return stats, nil
	}

	return nil, fmt.Errorf("неожиданный формат ответа")
}

func (c *APIClient) Clear(ctx context.Context) error {
	_, err := c.delete("/clear?confirm=true")
	return err
}

func (c *APIClient) CreateSubscription(ctx context.Context, id, script string, config *datastore.JSSubscriberConfig) error {
	req := SubscriptionRequest{
		ID:     id,
		Script: script,
	}

	if config != nil {
		req.ExecutionTimeout = int(config.ExecutionTimeout.Seconds())
		req.EnableNetworking = config.EnableNetworking
		req.EnableLogging = config.EnableLogging
		req.StrictMode = config.StrictMode

		// Преобразуем EventType в строки
		for _, eventType := range config.EventFilters {
			switch eventType {
			case datastore.EventPut:
				req.EventFilters = append(req.EventFilters, "put")
			case datastore.EventDelete:
				req.EventFilters = append(req.EventFilters, "delete")
			case datastore.EventBatch:
				req.EventFilters = append(req.EventFilters, "batch")
			}
		}
	}

	_, err := c.post("/subscriptions", req)
	return err
}

func (c *APIClient) RemoveSubscription(ctx context.Context, id string) error {
	endpoint := fmt.Sprintf("/subscriptions/%s", id)
	_, err := c.delete(endpoint)
	return err
}

func (c *APIClient) ListSubscriptions(ctx context.Context) ([]datastore.SavedJSSubscription, error) {
	apiResp, err := c.get("/subscriptions")
	if err != nil {
		return nil, err
	}

	if data, ok := apiResp.Data.(map[string]interface{}); ok {
		if subsData, ok := data["subscriptions"].([]interface{}); ok {
			var subscriptions []datastore.SavedJSSubscription

			// Конвертируем через JSON для корректного парсинга
			jsonData, err := json.Marshal(subsData)
			if err != nil {
				return nil, err
			}

			if err := json.Unmarshal(jsonData, &subscriptions); err != nil {
				return nil, err
			}

			return subscriptions, nil
		}
	}

	return nil, fmt.Errorf("неожиданный формат ответа")
}

func (c *APIClient) Health() error {
	_, err := c.get("/health")
	return err
}

// RemoteDatastore адаптер для работы с удаленным датастором через API
type RemoteDatastore struct {
	client *APIClient
}

func NewRemoteDatastore(endpoint string) (*RemoteDatastore, error) {
	client, err := NewAPIClient(endpoint)
	if err != nil {
		return nil, err
	}

	// Проверяем соединение
	if err := client.Health(); err != nil {
		return nil, fmt.Errorf("не удалось подключиться к серверу: %w", err)
	}

	return &RemoteDatastore{client: client}, nil
}

// Реализация интерфейса ds.Datastore

func (rd *RemoteDatastore) Get(ctx context.Context, key ds.Key) ([]byte, error) {
	return rd.client.Get(ctx, key)
}

func (rd *RemoteDatastore) Put(ctx context.Context, key ds.Key, value []byte) error {
	return rd.client.Put(ctx, key, value)
}

func (rd *RemoteDatastore) Delete(ctx context.Context, key ds.Key) error {
	return rd.client.Delete(ctx, key)
}

func (rd *RemoteDatastore) Has(ctx context.Context, key ds.Key) (bool, error) {
	return rd.client.Has(ctx, key)
}

func (rd *RemoteDatastore) GetSize(ctx context.Context, key ds.Key) (int, error) {
	return rd.client.GetSize(ctx, key)
}

func (rd *RemoteDatastore) Query(ctx context.Context, q ds.Query) (ds.Results, error) {
	// Упрощенная реализация Query - используем ListKeys
	keys, err := rd.client.ListKeys(ctx, q.Prefix, q.KeysOnly, 0)
	if err != nil {
		return nil, err
	}

	// Создаем channel-based результат
	results := make(chan ds.Result)

	go func() {
		defer close(results)

		for _, item := range keys {
			if q.KeysOnly {
				if keyStr, ok := item.(string); ok {
					results <- ds.Result{
						Entry: ds.Entry{Key: keyStr},
					}
				}
			} else {
				if keyInfo, ok := item.(map[string]interface{}); ok {
					if keyStr, ok := keyInfo["key"].(string); ok {
						if valueStr, ok := keyInfo["value"].(string); ok {
							results <- ds.Result{
								Entry: ds.Entry{
									Key:   keyStr,
									Value: []byte(valueStr),
								},
							}
						}
					}
				}
			}
		}
	}()

	return ds.ResultsFromChannel(results), nil
}

func (rd *RemoteDatastore) Batch(ctx context.Context) (ds.Batch, error) {
	return &RemoteBatch{
		client: rd.client,
		ctx:    ctx,
		ops:    make([]batchOperation, 0),
	}, nil
}

func (rd *RemoteDatastore) Close() error {
	// HTTP клиент не требует явного закрытия
	return nil
}

// RemoteBatch реализует ds.Batch для удаленного датастора
type RemoteBatch struct {
	client *APIClient
	ctx    context.Context
	ops    []batchOperation
}

type batchOperation struct {
	isDelete bool
	key      ds.Key
	value    []byte
}

func (rb *RemoteBatch) Put(ctx context.Context, key ds.Key, value []byte) error {
	rb.ops = append(rb.ops, batchOperation{
		isDelete: false,
		key:      key,
		value:    value,
	})
	return nil
}

func (rb *RemoteBatch) Delete(ctx context.Context, key ds.Key) error {
	rb.ops = append(rb.ops, batchOperation{
		isDelete: true,
		key:      key,
	})
	return nil
}

func (rb *RemoteBatch) Commit(ctx context.Context) error {
	// Выполняем все операции последовательно
	// В реальной реализации можно было бы сделать batch API endpoint
	for _, op := range rb.ops {
		if op.isDelete {
			if err := rb.client.Delete(ctx, op.key); err != nil {
				return err
			}
		} else {
			if err := rb.client.Put(ctx, op.key, op.value); err != nil {
				return err
			}
		}
	}

	// Очищаем операции после коммита
	rb.ops = rb.ops[:0]
	return nil
}

// Дополнительные методы RemoteDatastore для специфичных функций

func (rd *RemoteDatastore) PutWithTTL(ctx context.Context, key ds.Key, value []byte, ttl time.Duration) error {
	return rd.client.PutWithTTL(ctx, key, value, ttl)
}

func (rd *RemoteDatastore) ListKeys(ctx context.Context, prefix string, keysOnly bool, limit int) ([]interface{}, error) {
	return rd.client.ListKeys(ctx, prefix, keysOnly, limit)
}

func (rd *RemoteDatastore) Search(ctx context.Context, query string, caseSensitive, keysOnly bool, limit int) ([]interface{}, error) {
	return rd.client.Search(ctx, query, caseSensitive, keysOnly, limit)
}

func (rd *RemoteDatastore) GetStats(ctx context.Context) (map[string]interface{}, error) {
	return rd.client.GetStats(ctx)
}

func (rd *RemoteDatastore) Clear(ctx context.Context) error {
	return rd.client.Clear(ctx)
}

func (rd *RemoteDatastore) CreateJSSubscription(ctx context.Context, id, script string, config *datastore.JSSubscriberConfig) error {
	return rd.client.CreateSubscription(ctx, id, script, config)
}

func (rd *RemoteDatastore) RemoveJSSubscription(ctx context.Context, id string) error {
	return rd.client.RemoveSubscription(ctx, id)
}

func (rd *RemoteDatastore) ListJSSubscriptions(ctx context.Context) ([]datastore.SavedJSSubscription, error) {
	return rd.client.ListSubscriptions(ctx)
}

var _ datastore.Datastore = (*RemoteDatastoreAdapter)(nil)

// RemoteDatastoreAdapter адаптирует RemoteDatastore к DatastoreInterface
type RemoteDatastoreAdapter struct {
	*RemoteDatastore
}

func (r *RemoteDatastoreAdapter) Iterator(ctx interface{}, prefix interface{}, keysOnly bool) (<-chan datastore.KeyValue, <-chan error, error) {
	// Для удаленного датастора используем ListKeys и создаем channel
	keys, err := r.RemoteDatastore.ListKeys(ctx.(interface{ Deadline() (interface{}, bool) }),
		prefix.(string), keysOnly, 0)
	if err != nil {
		return nil, nil, err
	}

	kvChan := make(chan datastore.KeyValue, len(keys))
	errChan := make(chan error, 1)

	go func() {
		defer close(kvChan)
		defer close(errChan)

		for _, item := range keys {
			if keysOnly {
				if keyStr, ok := item.(string); ok {
					kvChan <- datastore.KeyValue{
						Key: datastore.NewKey(keyStr),
					}
				}
			} else {
				if keyInfo, ok := item.(map[string]interface{}); ok {
					if keyStr, ok := keyInfo["key"].(string); ok {
						valueStr := ""
						if val, ok := keyInfo["value"].(string); ok {
							valueStr = val
						}
						kvChan <- datastore.KeyValue{
							Key:   datastore.NewKey(keyStr),
							Value: []byte(valueStr),
						}
					}
				}
			}
		}
	}()

	return kvChan, errChan, nil
}

func (r *RemoteDatastoreAdapter) ListKeys(ctx interface{}, prefix string, keysOnly bool, limit int) ([]interface{}, error) {
	return r.RemoteDatastore.ListKeys(ctx.(interface{ Deadline() (interface{}, bool) }), prefix, keysOnly, limit)
}

func (r *RemoteDatastoreAdapter) Search(ctx interface{}, query string, caseSensitive, keysOnly bool, limit int) ([]interface{}, error) {
	return r.RemoteDatastore.Search(ctx.(interface{ Deadline() (interface{}, bool) }), query, caseSensitive, keysOnly, limit)
}

func (r *RemoteDatastoreAdapter) GetStats(ctx interface{}) (map[string]interface{}, error) {
	return r.RemoteDatastore.GetStats(ctx.(interface{ Deadline() (interface{}, bool) }))
}

func (r *RemoteDatastoreAdapter) Clear(ctx context.Context) error {
	return r.RemoteDatastore.Clear(ctx)
}

func (r *RemoteDatastoreAdapter) CreateJSSubscription(ctx interface{}, id, script string, config *datastore.JSSubscriberConfig) error {
	return r.RemoteDatastore.CreateJSSubscription(ctx.(interface{ Deadline() (interface{}, bool) }), id, script, config)
}

func (r *RemoteDatastoreAdapter) RemoveJSSubscription(ctx interface{}, id string) error {
	return r.RemoteDatastore.RemoveJSSubscription(ctx.(interface{ Deadline() (interface{}, bool) }), id)
}

func (r *RemoteDatastoreAdapter) ListJSSubscriptions(ctx interface{}) ([]datastore.SavedJSSubscription, error) {
	return r.RemoteDatastore.ListJSSubscriptions(ctx.(interface{ Deadline() (interface{}, bool) }))
}

// CollectGarbage is a no-op for RemoteDatastore
func (r *RemoteDatastoreAdapter) CollectGarbage(ctx context.Context) error {
	// No-op for remote datastore
	return nil
}

func (r *RemoteDatastoreAdapter) Merge(ctx context.Context, other datastore.Datastore) error {
	// No-op for remote datastore
	return nil
}

func (r *RemoteDatastoreAdapter) Close() error {
	return r.RemoteDatastore.Close()
}

// CreateFilteredJSSubscription creates a JS subscription with event filters
func (s *RemoteDatastoreAdapter) CreateFilteredJSSubscription(ctx context.Context, id, script string, eventTypes ...EventType) error {
	config := &datastore.JSSubscriberConfig{
		ExecutionTimeout: 5 * time.Second,
		EnableNetworking: true,
		EnableLogging:    true,
		EventFilters:     eventTypes,
		StrictMode:       false,
	}
	return s.CreateJSSubscription(ctx, id, script, config)
}
