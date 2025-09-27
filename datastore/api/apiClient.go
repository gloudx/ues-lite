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

	ds "github.com/ipfs/go-datastore"
)

// Структуры для API запросов и ответов
type SearchRequest struct {
	Query         string `json:"query"`
	CaseSensitive bool   `json:"case_sensitive,omitempty"`
	KeysOnly      bool   `json:"keys_only,omitempty"`
	Limit         int    `json:"limit,omitempty"`
}

type SubscriptionRequest struct {
	ID               string   `json:"id"`
	Script           string   `json:"script"`
	ExecutionTimeout int      `json:"execution_timeout,omitempty"`
	EnableNetworking bool     `json:"enable_networking,omitempty"`
	EnableLogging    bool     `json:"enable_logging,omitempty"`
	EventFilters     []string `json:"event_filters,omitempty"`
	StrictMode       bool     `json:"strict_mode,omitempty"`
}

type BatchRequest struct {
	Operations []BatchOperation `json:"operations"`
}

type BatchOperation struct {
	Op    string        `json:"op"` // put, delete
	Key   string        `json:"key"`
	Value string        `json:"value,omitempty"`
	TTL   time.Duration `json:"ttl,omitempty"`
}

type JQQueryRequest struct {
	Query            string        `json:"query"`
	Prefix           string        `json:"prefix,omitempty"`
	Limit            int           `json:"limit,omitempty"`
	Timeout          time.Duration `json:"timeout,omitempty"`
	TreatAsString    bool          `json:"treat_as_string,omitempty"`
	IgnoreParseError bool          `json:"ignore_parse_error,omitempty"`
}

type JQSingleRequest struct {
	Query string `json:"query"`
}

type TransformRequest struct {
	Key     string            `json:"key,omitempty"`
	Prefix  string            `json:"prefix,omitempty"`
	Options *TransformOptions `json:"options"`
}

type TransformJQRequest struct {
	Key          string            `json:"key,omitempty"`
	Prefix       string            `json:"prefix,omitempty"`
	JQExpression string            `json:"jq_expression"`
	Options      *TransformOptions `json:"options,omitempty"`
}

type TransformPatchRequest struct {
	Key      string            `json:"key,omitempty"`
	Prefix   string            `json:"prefix,omitempty"`
	PatchOps []PatchOp         `json:"patch_ops"`
	Options  *TransformOptions `json:"options,omitempty"`
}

type ViewRequest struct {
	Config ViewConfig `json:",inline"`
}

// APIClient представляет клиент для работы с REST API
type APIClient struct {
	client  *http.Client
	baseURL string
	isUnix  bool
	token   string
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

// NewAPIClientWithAuth создает API клиент с авторизацией
func NewAPIClientWithAuth(endpoint, token string) (*APIClient, error) {
	client, err := NewAPIClient(endpoint)
	if err != nil {
		return nil, err
	}
	client.token = token
	return client, nil
}

// Вспомогательные методы для HTTP запросов

func (c *APIClient) get(endpoint string) (*APIResponse, error) {
	url := c.baseURL + "/api/v1" + endpoint
	return c.doRequest("GET", url, nil)
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
	url := c.baseURL + "/api/v1" + endpoint

	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("ошибка сериализации JSON: %w", err)
		}
		reqBody = bytes.NewReader(jsonData)
	}

	return c.doRequest(method, url, reqBody)
}

func (c *APIClient) doRequest(method, url string, body io.Reader) (*APIResponse, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("ошибка создания запроса: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// Добавляем авторизацию если есть токен
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
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

// Базовые методы датастора

func (c *APIClient) Get(ctx context.Context, key ds.Key) ([]byte, error) {
	endpoint := fmt.Sprintf("/keys%s?format=raw", key.String())
	url := c.baseURL + "/api/v1" + endpoint

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "text/plain")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

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
	url := c.baseURL + "/api/v1" + endpoint

	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader(value))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "text/plain")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

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
	reqBody := struct {
		Value string        `json:"value"`
		TTL   time.Duration `json:"ttl"`
	}{
		Value: string(value),
		TTL:   ttl,
	}

	endpoint := fmt.Sprintf("/keys%s", key.String())
	_, err := c.request("PUT", endpoint, reqBody)
	return err
}

func (c *APIClient) Delete(ctx context.Context, key ds.Key) error {
	endpoint := fmt.Sprintf("/keys%s", key.String())
	_, err := c.delete(endpoint)
	return err
}

func (c *APIClient) Has(ctx context.Context, key ds.Key) (bool, error) {
	endpoint := fmt.Sprintf("/keys%s/info", key.String())
	_, err := c.get(endpoint)
	if err != nil {
		if strings.Contains(err.Error(), "Ключ не найден") || strings.Contains(err.Error(), "404") {
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

	if data, ok := apiResp.Data.(map[string]interface{}); ok {
		if size, ok := data["size"].(float64); ok {
			return int(size), nil
		}
	}

	return 0, fmt.Errorf("неожиданный формат ответа")
}

func (c *APIClient) GetExpiration(ctx context.Context, key ds.Key) (time.Time, error) {
	endpoint := fmt.Sprintf("/keys%s/info", key.String())
	apiResp, err := c.get(endpoint)
	if err != nil {
		return time.Time{}, err
	}

	if data, ok := apiResp.Data.(map[string]interface{}); ok {
		if ttl, ok := data["ttl"].(string); ok && ttl != "" {
			duration, err := time.ParseDuration(ttl)
			if err != nil {
				return time.Time{}, err
			}
			return time.Now().Add(duration), nil
		}
	}

	return time.Time{}, fmt.Errorf("ключ не имеет TTL")
}

func (c *APIClient) SetTTL(ctx context.Context, key ds.Key, ttl time.Duration) error {
	// Получаем текущее значение
	value, err := c.Get(ctx, key)
	if err != nil {
		return err
	}

	// Перезаписываем с TTL
	return c.PutWithTTL(ctx, key, value, ttl)
}

// Расширенные методы

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

// Batch операции

func (c *APIClient) ExecuteBatch(ctx context.Context, operations []BatchOperation) error {
	req := BatchRequest{Operations: operations}
	_, err := c.post("/batch", req)
	return err
}

// JQ запросы

func (c *APIClient) QueryJQ(ctx context.Context, query string, opts *JQQueryOptions) ([]map[string]interface{}, error) {
	req := JQQueryRequest{
		Query: query,
	}

	if opts != nil {
		req.Prefix = opts.Prefix.String()
		req.Limit = opts.Limit
		req.Timeout = opts.Timeout
		req.TreatAsString = opts.TreatAsString
		req.IgnoreParseError = opts.IgnoreParseError
	}

	apiResp, err := c.post("/query", req)
	if err != nil {
		return nil, err
	}

	if data, ok := apiResp.Data.(map[string]interface{}); ok {
		if results, ok := data["results"].([]interface{}); ok {
			var queryResults []map[string]interface{}
			for _, item := range results {
				if resultMap, ok := item.(map[string]interface{}); ok {
					queryResults = append(queryResults, resultMap)
				}
			}
			return queryResults, nil
		}
	}

	return nil, fmt.Errorf("неожиданный формат ответа")
}

func (c *APIClient) AggregateJQ(ctx context.Context, query string, opts *JQQueryOptions) (interface{}, error) {
	req := JQQueryRequest{
		Query: query,
	}

	if opts != nil {
		req.Prefix = opts.Prefix.String()
		req.Timeout = opts.Timeout
		req.TreatAsString = opts.TreatAsString
		req.IgnoreParseError = opts.IgnoreParseError
	}

	apiResp, err := c.post("/query/aggregate", req)
	if err != nil {
		return nil, err
	}

	if data, ok := apiResp.Data.(map[string]interface{}); ok {
		if result, ok := data["result"]; ok {
			return result, nil
		}
	}

	return nil, fmt.Errorf("неожиданный формат ответа")
}

func (c *APIClient) QueryJQSingle(ctx context.Context, key ds.Key, query string) (interface{}, error) {
	endpoint := fmt.Sprintf("/keys%s/query", key.String())
	req := JQSingleRequest{Query: query}

	apiResp, err := c.post(endpoint, req)
	if err != nil {
		return nil, err
	}

	if data, ok := apiResp.Data.(map[string]interface{}); ok {
		if result, ok := data["result"]; ok {
			return result, nil
		}
	}

	return nil, fmt.Errorf("неожиданный формат ответа")
}

// Transform операции

func (c *APIClient) Transform(ctx context.Context, key ds.Key, opts *TransformOptions) (*TransformSummary, error) {
	req := TransformRequest{
		Options: opts,
	}
	if key.String() != "/" {
		req.Key = key.String()
	}

	apiResp, err := c.post("/transform", req)
	if err != nil {
		return nil, err
	}

	var summary TransformSummary
	if data, ok := apiResp.Data.(map[string]interface{}); ok {
		bytes, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(bytes, &summary); err != nil {
			return nil, err
		}
		return &summary, nil
	}

	return nil, fmt.Errorf("неожиданный формат ответа")
}

func (c *APIClient) TransformWithJQ(ctx context.Context, key ds.Key, jqExpression string, opts *TransformOptions) (*TransformSummary, error) {
	req := TransformJQRequest{
		JQExpression: jqExpression,
		Options:      opts,
	}
	if key.String() != "/" {
		req.Key = key.String()
	}

	apiResp, err := c.post("/transform/jq", req)
	if err != nil {
		return nil, err
	}

	var summary TransformSummary
	if data, ok := apiResp.Data.(map[string]interface{}); ok {
		bytes, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(bytes, &summary); err != nil {
			return nil, err
		}
		return &summary, nil
	}

	return nil, fmt.Errorf("неожиданный формат ответа")
}

func (c *APIClient) TransformWithPatch(ctx context.Context, key ds.Key, patchOps []PatchOp, opts *TransformOptions) (*TransformSummary, error) {
	req := TransformPatchRequest{
		PatchOps: patchOps,
		Options:  opts,
	}
	if key.String() != "/" {
		req.Key = key.String()
	}

	apiResp, err := c.post("/transform/patch", req)
	if err != nil {
		return nil, err
	}

	var summary TransformSummary
	if data, ok := apiResp.Data.(map[string]interface{}); ok {
		bytes, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(bytes, &summary); err != nil {
			return nil, err
		}
		return &summary, nil
	}

	return nil, fmt.Errorf("неожиданный формат ответа")
}

// Views

func (c *APIClient) ListViews(ctx context.Context) ([]ViewConfig, error) {
	apiResp, err := c.get("/views")
	if err != nil {
		return nil, err
	}

	if data, ok := apiResp.Data.(map[string]interface{}); ok {
		if views, ok := data["views"].([]interface{}); ok {
			var configs []ViewConfig
			for _, view := range views {
				if viewMap, ok := view.(map[string]interface{}); ok {
					if config, ok := viewMap["config"].(map[string]interface{}); ok {
						bytes, err := json.Marshal(config)
						if err != nil {
							continue
						}
						var viewConfig ViewConfig
						if err := json.Unmarshal(bytes, &viewConfig); err != nil {
							continue
						}
						configs = append(configs, viewConfig)
					}
				}
			}
			return configs, nil
		}
	}

	return nil, fmt.Errorf("неожиданный формат ответа")
}

func (c *APIClient) CreateView(ctx context.Context, config ViewConfig) error {
	_, err := c.post("/views", config)
	return err
}

func (c *APIClient) GetView(ctx context.Context, id string) (*ViewConfig, error) {
	endpoint := fmt.Sprintf("/views/%s", id)
	apiResp, err := c.get(endpoint)
	if err != nil {
		return nil, err
	}

	if data, ok := apiResp.Data.(map[string]interface{}); ok {
		if config, ok := data["config"].(map[string]interface{}); ok {
			bytes, err := json.Marshal(config)
			if err != nil {
				return nil, err
			}
			var viewConfig ViewConfig
			if err := json.Unmarshal(bytes, &viewConfig); err != nil {
				return nil, err
			}
			return &viewConfig, nil
		}
	}

	return nil, fmt.Errorf("неожиданный формат ответа")
}

func (c *APIClient) UpdateView(ctx context.Context, id string, config ViewConfig) error {
	endpoint := fmt.Sprintf("/views/%s", id)
	config.ID = id
	_, err := c.put(endpoint, config)
	return err
}

func (c *APIClient) DeleteView(ctx context.Context, id string) error {
	endpoint := fmt.Sprintf("/views/%s", id)
	_, err := c.delete(endpoint)
	return err
}

func (c *APIClient) ExecuteView(ctx context.Context, id string) ([]ViewResult, error) {
	endpoint := fmt.Sprintf("/views/%s/execute", id)
	apiResp, err := c.post(endpoint, nil)
	if err != nil {
		return nil, err
	}

	if data, ok := apiResp.Data.(map[string]interface{}); ok {
		if results, ok := data["results"].([]interface{}); ok {
			var viewResults []ViewResult
			bytes, err := json.Marshal(results)
			if err != nil {
				return nil, err
			}
			if err := json.Unmarshal(bytes, &viewResults); err != nil {
				return nil, err
			}
			return viewResults, nil
		}
	}

	return nil, fmt.Errorf("неожиданный формат ответа")
}

func (c *APIClient) RefreshView(ctx context.Context, id string) error {
	endpoint := fmt.Sprintf("/views/%s/refresh", id)
	_, err := c.post(endpoint, nil)
	return err
}

func (c *APIClient) RefreshAllViews(ctx context.Context) error {
	_, err := c.post("/views/refresh", nil)
	return err
}

func (c *APIClient) GetViewStats(ctx context.Context, id string) (*ViewStats, error) {
	endpoint := fmt.Sprintf("/views/%s/stats", id)
	apiResp, err := c.get(endpoint)
	if err != nil {
		return nil, err
	}

	var stats ViewStats
	if data, ok := apiResp.Data.(map[string]interface{}); ok {
		bytes, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(bytes, &stats); err != nil {
			return nil, err
		}
		return &stats, nil
	}

	return nil, fmt.Errorf("неожиданный формат ответа")
}

// Подписки

func (c *APIClient) CreateSubscription(ctx context.Context, id, script string, config *JSSubscriberConfig) error {
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
			case EventPut:
				req.EventFilters = append(req.EventFilters, "put")
			case EventDelete:
				req.EventFilters = append(req.EventFilters, "delete")
			case EventBatch:
				req.EventFilters = append(req.EventFilters, "batch")
			case EventTTLExpired:
				req.EventFilters = append(req.EventFilters, "ttl_expired")
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

func (c *APIClient) ListSubscriptions(ctx context.Context) ([]jsSubscription, error) {
	apiResp, err := c.get("/subscriptions")
	if err != nil {
		return nil, err
	}

	if data, ok := apiResp.Data.(map[string]interface{}); ok {
		if subsData, ok := data["subscriptions"].([]interface{}); ok {
			var subscriptions []jsSubscription

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

// Система

func (c *APIClient) SetMode(ctx context.Context, silent bool) error {
	req := map[string]bool{"silent": silent}
	_, err := c.post("/system/mode", req)
	return err
}

func (c *APIClient) GC(ctx context.Context) error {
	_, err := c.post("/system/gc", nil)
	return err
}

// Утилиты

func (c *APIClient) Health() error {
	_, err := c.get("/health")
	return err
}

func (c *APIClient) GetDocs() (string, error) {
	url := c.baseURL + "/api/v1/docs"
	resp, err := c.client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// Стрим операции (упрощенные версии)

func (c *APIClient) StreamTo(ctx context.Context, writer io.Writer, opts *StreamOptions) error {
	endpoint := "/stream"
	params := []string{}

	if opts != nil {
		if opts.Format != "" {
			params = append(params, fmt.Sprintf("format=%s", opts.Format))
		}
		if opts.Prefix.String() != "" {
			params = append(params, fmt.Sprintf("prefix=%s", opts.Prefix.String()))
		}
		if opts.JQFilter != "" {
			params = append(params, fmt.Sprintf("jq=%s", opts.JQFilter))
		}
		if opts.IncludeKeys {
			params = append(params, "include_keys=true")
		}
		if opts.Limit > 0 {
			params = append(params, fmt.Sprintf("limit=%d", opts.Limit))
		}
	}

	if len(params) > 0 {
		endpoint += "?" + strings.Join(params, "&")
	}

	url := c.baseURL + "/api/v1" + endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	_, err = io.Copy(writer, resp.Body)
	return err
}

func (c *APIClient) GetTTLStats(ctx context.Context, prefix ds.Key) (*TTLStats, error) {
	endpoint := "/ttl/stats"
	if prefix.String() != "" {
		endpoint += fmt.Sprintf("?prefix=%s", prefix.String())
	}

	apiResp, err := c.get(endpoint)
	if err != nil {
		return nil, err
	}

	var stats TTLStats
	if data, ok := apiResp.Data.(map[string]interface{}); ok {
		bytes, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(bytes, &stats); err != nil {
			return nil, err
		}
		return &stats, nil
	}

	return nil, fmt.Errorf("неожиданный формат ответа")
}