package datastore

import (
	"context"
	"fmt"
	"io"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
)

// RemoteResults реализует интерфейс query.Results для удаленного датастора
type RemoteResults struct {
	results []query.Result
	index   int
}

func (r *RemoteResults) Next() <-chan query.Result {
	ch := make(chan query.Result)
	go func() {
		defer close(ch)
		if r.index < len(r.results) {
			ch <- r.results[r.index]
			r.index++
		}
	}()
	return ch
}

func (r *RemoteResults) Rest() ([]query.Entry, error) {
	if r.index >= len(r.results) {
		return nil, nil
	}
	entries := make([]query.Entry, 0, len(r.results)-r.index)
	for ; r.index < len(r.results); r.index++ {
		entries = append(entries, r.results[r.index].Entry)
	}
	return entries, nil
}

func (r *RemoteResults) Close() error {
	return nil
}

func (r *RemoteResults) Done() <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		defer close(ch)
		if r.index >= len(r.results) {
			ch <- struct{}{}
		}
	}()
	return ch
}

func (r *RemoteResults) NextSync() (query.Result, bool) {
	if r.index < len(r.results) {
		res := r.results[r.index]
		r.index++
		return res, true
	}
	return query.Result{}, false
}

func (r *RemoteResults) Query() query.Query {
	// Возвращаем пустой запрос, так как оригинальный запрос не сохраняется
	return query.Query{}
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

func NewRemoteDatastoreWithAuth(endpoint, token string) (*RemoteDatastore, error) {
	client, err := NewAPIClientWithAuth(endpoint, token)
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

func (rd *RemoteDatastore) Query(ctx context.Context, q query.Query) (query.Results, error) {
	// Конвертируем query в ListKeys запрос
	keys, err := rd.client.ListKeys(ctx, q.Prefix, q.KeysOnly, 0)
	if err != nil {
		return nil, err
	}

	// Создаем результаты в слайсе
	var results []query.Result

	for _, item := range keys {
		if q.KeysOnly {
			if keyStr, ok := item.(string); ok {
				results = append(results, query.Result{
					Entry: query.Entry{Key: keyStr},
				})
			}
		} else {
			if keyInfo, ok := item.(map[string]interface{}); ok {
				if keyStr, ok := keyInfo["key"].(string); ok {
					valueStr := ""
					if val, ok := keyInfo["value"].(string); ok {
						valueStr = val
					}
					results = append(results, query.Result{
						Entry: query.Entry{
							Key:   keyStr,
							Value: []byte(valueStr),
						},
					})
				}
			}
		}
	}

	return &RemoteResults{results: results, index: 0}, nil
}

func (rd *RemoteDatastore) Batch(ctx context.Context) (ds.Batch, error) {
	return &RemoteBatch{
		client: rd.client,
		ctx:    ctx,
		ops:    make([]BatchOperation, 0),
	}, nil
}

func (rd *RemoteDatastore) Close() error {
	// HTTP клиент не требует явного закрытия
	return nil
}

// Реализация интерфейса ds.TTLDatastore

func (rd *RemoteDatastore) PutWithTTL(ctx context.Context, key ds.Key, value []byte, ttl time.Duration) error {
	return rd.client.PutWithTTL(ctx, key, value, ttl)
}

func (rd *RemoteDatastore) SetTTL(ctx context.Context, key ds.Key, ttl time.Duration) error {
	return rd.client.SetTTL(ctx, key, ttl)
}

func (rd *RemoteDatastore) GetExpiration(ctx context.Context, key ds.Key) (time.Time, error) {
	return rd.client.GetExpiration(ctx, key)
}

// Реализация интерфейса ds.GCDatastore

func (rd *RemoteDatastore) CollectGarbage(ctx context.Context) error {
	return rd.client.GC(ctx)
}

// Реализация интерфейса ds.TxnDatastore (упрощенная)

func (rd *RemoteDatastore) NewTransaction(ctx context.Context, readOnly bool) (ds.Txn, error) {
	// Удаленный датастор не поддерживает настоящие транзакции
	// Возвращаем batch как транзакцию
	batch, err := rd.Batch(ctx)
	if err != nil {
		return nil, err
	}
	return &RemoteTxn{batch: batch.(*RemoteBatch), readOnly: readOnly}, nil
}

// RemoteBatch реализует ds.Batch для удаленного датастора
type RemoteBatch struct {
	client *APIClient
	ctx    context.Context
	ops    []BatchOperation
}

func (rb *RemoteBatch) Put(ctx context.Context, key ds.Key, value []byte) error {
	rb.ops = append(rb.ops, BatchOperation{
		Op:    "put",
		Key:   key.String(),
		Value: string(value),
	})
	return nil
}

func (rb *RemoteBatch) Delete(ctx context.Context, key ds.Key) error {
	rb.ops = append(rb.ops, BatchOperation{
		Op:  "delete",
		Key: key.String(),
	})
	return nil
}

func (rb *RemoteBatch) Commit(ctx context.Context) error {
	if len(rb.ops) == 0 {
		return nil
	}

	err := rb.client.ExecuteBatch(ctx, rb.ops)
	if err == nil {
		// Очищаем операции после коммита
		rb.ops = rb.ops[:0]
	}
	return err
}

// RemoteTxn реализует ds.Txn для удаленного датастора
type RemoteTxn struct {
	batch    *RemoteBatch
	readOnly bool
}

func (rt *RemoteTxn) Get(ctx context.Context, key ds.Key) ([]byte, error) {
	return rt.batch.client.Get(ctx, key)
}

func (rt *RemoteTxn) Put(ctx context.Context, key ds.Key, value []byte) error {
	if rt.readOnly {
		return fmt.Errorf("cannot put in read-only transaction")
	}
	return rt.batch.Put(ctx, key, value)
}

func (rt *RemoteTxn) Delete(ctx context.Context, key ds.Key) error {
	if rt.readOnly {
		return fmt.Errorf("cannot delete in read-only transaction")
	}
	return rt.batch.Delete(ctx, key)
}

func (rt *RemoteTxn) Has(ctx context.Context, key ds.Key) (bool, error) {
	return rt.batch.client.Has(ctx, key)
}

func (rt *RemoteTxn) GetSize(ctx context.Context, key ds.Key) (int, error) {
	return rt.batch.client.GetSize(ctx, key)
}

func (rt *RemoteTxn) Query(ctx context.Context, q query.Query) (query.Results, error) {
	// В транзакции используем обычный запрос
	keys, err := rt.batch.client.ListKeys(ctx, q.Prefix, q.KeysOnly, 0)
	if err != nil {
		return nil, err
	}

	var results []query.Result
	for _, item := range keys {
		if q.KeysOnly {
			if keyStr, ok := item.(string); ok {
				results = append(results, query.Result{
					Entry: query.Entry{Key: keyStr},
				})
			}
		} else {
			if keyInfo, ok := item.(map[string]interface{}); ok {
				if keyStr, ok := keyInfo["key"].(string); ok {
					valueStr := ""
					if val, ok := keyInfo["value"].(string); ok {
						valueStr = val
					}
					results = append(results, query.Result{
						Entry: query.Entry{
							Key:   keyStr,
							Value: []byte(valueStr),
						},
					})
				}
			}
		}
	}

	return &RemoteResults{results: results, index: 0}, nil
}

func (rt *RemoteTxn) Commit(ctx context.Context) error {
	if rt.readOnly {
		return nil
	}
	return rt.batch.Commit(ctx)
}

func (rt *RemoteTxn) Discard(ctx context.Context) {
	// Очищаем операции без коммита
	rt.batch.ops = rt.batch.ops[:0]
}

// RemoteDatastoreAdapter адаптирует RemoteDatastore к полному интерфейсу Datastore
type RemoteDatastoreAdapter struct {
	*RemoteDatastore
}

func NewRemoteDatastoreAdapter(endpoint string) (*RemoteDatastoreAdapter, error) {
	rd, err := NewRemoteDatastore(endpoint)
	if err != nil {
		return nil, err
	}
	return &RemoteDatastoreAdapter{RemoteDatastore: rd}, nil
}

func NewRemoteDatastoreAdapterWithAuth(endpoint, token string) (*RemoteDatastoreAdapter, error) {
	rd, err := NewRemoteDatastoreWithAuth(endpoint, token)
	if err != nil {
		return nil, err
	}
	return &RemoteDatastoreAdapter{RemoteDatastore: rd}, nil
}

// Реализация расширенного интерфейса Datastore

func (r *RemoteDatastoreAdapter) Iterator(ctx context.Context, prefix ds.Key, keysOnly bool) (<-chan KeyValue, <-chan error, error) {
	keys, err := r.client.ListKeys(ctx, prefix.String(), keysOnly, 0)
	if err != nil {
		return nil, nil, err
	}

	kvChan := make(chan KeyValue, len(keys))
	errChan := make(chan error, 1)

	go func() {
		defer close(kvChan)
		defer close(errChan)

		for _, item := range keys {
			if keysOnly {
				if keyStr, ok := item.(string); ok {
					kvChan <- KeyValue{
						Key: ds.NewKey(keyStr),
					}
				}
			} else {
				if keyInfo, ok := item.(map[string]interface{}); ok {
					if keyStr, ok := keyInfo["key"].(string); ok {
						valueStr := ""
						if val, ok := keyInfo["value"].(string); ok {
							valueStr = val
						}
						kvChan <- KeyValue{
							Key:   ds.NewKey(keyStr),
							Value: []byte(valueStr),
						}
					}
				}
			}
		}
	}()

	return kvChan, errChan, nil
}

func (r *RemoteDatastoreAdapter) Keys(ctx context.Context, prefix ds.Key) (<-chan ds.Key, <-chan error, error) {
	keys, err := r.client.ListKeys(ctx, prefix.String(), true, 0)
	if err != nil {
		return nil, nil, err
	}

	keyChan := make(chan ds.Key, len(keys))
	errChan := make(chan error, 1)

	go func() {
		defer close(keyChan)
		defer close(errChan)

		for _, item := range keys {
			if keyStr, ok := item.(string); ok {
				keyChan <- ds.NewKey(keyStr)
			}
		}
	}()

	return keyChan, errChan, nil
}

func (r *RemoteDatastoreAdapter) Merge(ctx context.Context, other Datastore) error {
	batch, err := r.Batch(ctx)
	if err != nil {
		return err
	}

	it, errc, err := other.Iterator(ctx, ds.NewKey("/"), false)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case e, ok := <-errc:
			if ok && e != nil {
				return e
			}
			if !ok {
				return batch.Commit(ctx)
			}
		case kv, ok := <-it:
			if !ok {
				return batch.Commit(ctx)
			}
			if err := batch.Put(ctx, kv.Key, kv.Value); err != nil {
				return err
			}
		}
	}
}

func (r *RemoteDatastoreAdapter) Clear(ctx context.Context) error {
	return r.client.Clear(ctx)
}

func (r *RemoteDatastoreAdapter) SetSilentMode(silent bool) {
	// Устанавливаем режим на удаленном сервере
	r.client.SetMode(context.Background(), silent)
}

// Подписки и события

func (r *RemoteDatastoreAdapter) Subscribe(subscriber Subscriber) {
	// Удаленный датастор не поддерживает локальные подписки
	// События должны обрабатываться через WebSockets или SSE
}

func (r *RemoteDatastoreAdapter) Unsubscribe(subscriberID string) {
	// No-op для удаленного датастора
}

func (r *RemoteDatastoreAdapter) SubscribeFunc(id string, handler EventHandler) {
	// No-op для удаленного датастора
}

func (r *RemoteDatastoreAdapter) SubscribeChannel(id string, buffer int) *ChannelSubscriber {
	// Возвращаем пустой подписчик
	return NewChannelSubscriber(id, buffer)
}

// JS Подписки

func (r *RemoteDatastoreAdapter) ListJSSubscriptions(ctx context.Context) ([]jsSubscription, error) {
	return r.client.ListSubscriptions(ctx)
}

func (r *RemoteDatastoreAdapter) CreateJSSubscription(ctx context.Context, id, script string, config *JSSubscriberConfig) error {
	return r.client.CreateSubscription(ctx, id, script, config)
}

func (r *RemoteDatastoreAdapter) CreateSimpleJSSubscription(ctx context.Context, id, script string) error {
	return r.client.CreateSubscription(ctx, id, script, nil)
}

func (r *RemoteDatastoreAdapter) CreateFilteredJSSubscription(ctx context.Context, id, script string, eventTypes ...EventType) error {
	config := &JSSubscriberConfig{
		ExecutionTimeout: 5 * time.Second,
		EnableNetworking: true,
		EnableLogging:    true,
		EventFilters:     eventTypes,
		StrictMode:       false,
	}
	return r.client.CreateSubscription(ctx, id, script, config)
}

func (r *RemoteDatastoreAdapter) RemoveJSSubscription(ctx context.Context, id string) error {
	return r.client.RemoveSubscription(ctx, id)
}

// JQ запросы

func (r *RemoteDatastoreAdapter) QueryJQ(ctx context.Context, jqQuery string, opts *JQQueryOptions) (<-chan JQResult, <-chan error, error) {
	results, err := r.client.QueryJQ(ctx, jqQuery, opts)
	if err != nil {
		return nil, nil, err
	}

	resultChan := make(chan JQResult, len(results))
	errorChan := make(chan error, 1)

	go func() {
		defer close(resultChan)
		defer close(errorChan)

		for _, result := range results {
			if key, ok := result["key"].(string); ok {
				jqResult := JQResult{
					Key:   ds.NewKey(key),
					Value: result["value"],
				}
				resultChan <- jqResult
			}
		}
	}()

	return resultChan, errorChan, nil
}

func (r *RemoteDatastoreAdapter) AggregateJQ(ctx context.Context, jqQuery string, opts *JQQueryOptions) (interface{}, error) {
	return r.client.AggregateJQ(ctx, jqQuery, opts)
}

func (r *RemoteDatastoreAdapter) QueryJQSingle(ctx context.Context, key ds.Key, jqQuery string) (interface{}, error) {
	return r.client.QueryJQSingle(ctx, key, jqQuery)
}

// Transform операции

func (r *RemoteDatastoreAdapter) Transform(ctx context.Context, key ds.Key, opts *TransformOptions) (*TransformSummary, error) {
	return r.client.Transform(ctx, key, opts)
}

func (r *RemoteDatastoreAdapter) TransformWithJQ(ctx context.Context, key ds.Key, jqExpression string, opts *TransformOptions) (*TransformSummary, error) {
	return r.client.TransformWithJQ(ctx, key, jqExpression, opts)
}

func (r *RemoteDatastoreAdapter) TransformWithPatch(ctx context.Context, key ds.Key, patchOps []PatchOp, opts *TransformOptions) (*TransformSummary, error) {
	return r.client.TransformWithPatch(ctx, key, patchOps, opts)
}

// Views

func (r *RemoteDatastoreAdapter) CreateView(ctx context.Context, config ViewConfig) (View, error) {
	err := r.client.CreateView(ctx, config)
	if err != nil {
		return nil, err
	}
	return &RemoteView{client: r.client, config: config}, nil
}

func (r *RemoteDatastoreAdapter) GetView(id string) (View, bool) {
	config, err := r.client.GetView(context.Background(), id)
	if err != nil {
		return nil, false
	}
	return &RemoteView{client: r.client, config: *config}, true
}

func (r *RemoteDatastoreAdapter) ListViews() []View {
	configs, err := r.client.ListViews(context.Background())
	if err != nil {
		return nil
	}

	views := make([]View, len(configs))
	for i, config := range configs {
		views[i] = &RemoteView{client: r.client, config: config}
	}
	return views
}

func (r *RemoteDatastoreAdapter) RemoveView(ctx context.Context, id string) error {
	return r.client.DeleteView(ctx, id)
}

func (r *RemoteDatastoreAdapter) RefreshView(ctx context.Context, id string) error {
	return r.client.RefreshView(ctx, id)
}

func (r *RemoteDatastoreAdapter) RefreshAllViews(ctx context.Context) error {
	return r.client.RefreshAllViews(ctx)
}

func (r *RemoteDatastoreAdapter) GetViewStats(id string) (ViewStats, bool) {
	stats, err := r.client.GetViewStats(context.Background(), id)
	if err != nil {
		return ViewStats{}, false
	}
	return *stats, true
}

func (r *RemoteDatastoreAdapter) SaveViewConfig(ctx context.Context, config ViewConfig) error {
	return r.client.CreateView(ctx, config)
}

func (r *RemoteDatastoreAdapter) LoadViewConfigs(ctx context.Context) error {
	// No-op для удаленного датастора - конфигурации загружаются автоматически
	return nil
}

func (r *RemoteDatastoreAdapter) ExecuteView(ctx context.Context, id string) ([]ViewResult, error) {
	return r.client.ExecuteView(ctx, id)
}

func (r *RemoteDatastoreAdapter) GetViewCached(ctx context.Context, id string) ([]ViewResult, bool, error) {
	// Для удаленного датастора всегда выполняем запрос
	results, err := r.client.ExecuteView(ctx, id)
	if err != nil {
		return nil, false, err
	}
	return results, true, nil
}

func (r *RemoteDatastoreAdapter) CreateSimpleView(ctx context.Context, id, name, sourcePrefix, script string) (View, error) {
	config := ViewConfig{
		ID:           id,
		Name:         name,
		SourcePrefix: sourcePrefix,
		FilterScript: script,
	}
	return r.CreateView(ctx, config)
}

// TTL мониторинг

func (r *RemoteDatastoreAdapter) EnableTTLMonitoring(config *TTLMonitorConfig) error {
	// Удаленный датастор управляет TTL на сервере
	return nil
}

func (r *RemoteDatastoreAdapter) DisableTTLMonitoring() error {
	// Удаленный датастор управляет TTL на сервере
	return nil
}

func (r *RemoteDatastoreAdapter) GetTTLMonitorConfig() *TTLMonitorConfig {
	// Возвращаем конфигурацию по умолчанию
	return &TTLMonitorConfig{
		CheckInterval: 30 * time.Second,
		Enabled:       false,
		BufferSize:    100,
	}
}

// Streaming

func (r *RemoteDatastoreAdapter) StreamTo(ctx context.Context, writer io.Writer, opts *StreamOptions) error {
	return r.client.StreamTo(ctx, writer, opts)
}

func (r *RemoteDatastoreAdapter) StreamEvents(ctx context.Context, writer io.Writer, opts *StreamOptions) error {
	return r.client.StreamTo(ctx, writer, opts)
}

func (r *RemoteDatastoreAdapter) StreamJSON(ctx context.Context, writer io.Writer, prefix ds.Key, includeKeys bool) error {
	opts := &StreamOptions{
		Format:      StreamFormatJSON,
		Prefix:      prefix,
		IncludeKeys: includeKeys,
	}
	return r.client.StreamTo(ctx, writer, opts)
}

func (r *RemoteDatastoreAdapter) StreamJSONL(ctx context.Context, writer io.Writer, prefix ds.Key, includeKeys bool) error {
	opts := &StreamOptions{
		Format:      StreamFormatJSONL,
		Prefix:      prefix,
		IncludeKeys: includeKeys,
	}
	return r.client.StreamTo(ctx, writer, opts)
}

func (r *RemoteDatastoreAdapter) StreamCSV(ctx context.Context, writer io.Writer, prefix ds.Key, includeKeys bool) error {
	opts := &StreamOptions{
		Format:      StreamFormatCSV,
		Prefix:      prefix,
		IncludeKeys: includeKeys,
	}
	return r.client.StreamTo(ctx, writer, opts)
}

func (r *RemoteDatastoreAdapter) StreamSSE(ctx context.Context, writer io.Writer, headers map[string]string) error {
	opts := &StreamOptions{
		Format:  StreamFormatSSE,
		Headers: headers,
	}
	return r.client.StreamTo(ctx, writer, opts)
}

func (r *RemoteDatastoreAdapter) StreamBinary(ctx context.Context, writer io.Writer, prefix ds.Key) error {
	opts := &StreamOptions{
		Format: StreamFormatBinary,
		Prefix: prefix,
	}
	return r.client.StreamTo(ctx, writer, opts)
}

func (r *RemoteDatastoreAdapter) StreamWithJQ(ctx context.Context, writer io.Writer, jqQuery string, format StreamFormat, prefix ds.Key) error {
	opts := &StreamOptions{
		Format:   format,
		Prefix:   prefix,
		JQFilter: jqQuery,
	}
	return r.client.StreamTo(ctx, writer, opts)
}

func (r *RemoteDatastoreAdapter) NewStreamPipeline(opts *StreamOptions) *StreamPipeline {
	// Для удаленного датастора возвращаем упрощенную реализацию
	return &StreamPipeline{
		opts: opts,
	}
}

// RemoteView реализует интерфейс View для удаленного датастора
type RemoteView struct {
	client *APIClient
	config ViewConfig
}

func (rv *RemoteView) ID() string {
	return rv.config.ID
}

func (rv *RemoteView) Config() ViewConfig {
	return rv.config
}

func (rv *RemoteView) Execute(ctx context.Context) ([]ViewResult, error) {
	return rv.client.ExecuteView(ctx, rv.config.ID)
}

func (rv *RemoteView) ExecuteWithRange(ctx context.Context, start, end ds.Key) ([]ViewResult, error) {
	// Удаленный API не поддерживает диапазоны напрямую, выполняем обычный запрос
	return rv.Execute(ctx)
}

func (rv *RemoteView) Refresh(ctx context.Context) error {
	return rv.client.RefreshView(ctx, rv.config.ID)
}

func (rv *RemoteView) GetCached(ctx context.Context) ([]ViewResult, bool, error) {
	results, err := rv.Execute(ctx)
	if err != nil {
		return nil, false, err
	}
	return results, true, nil
}

func (rv *RemoteView) InvalidateCache(ctx context.Context) error {
	return rv.Refresh(ctx)
}

func (rv *RemoteView) Stats() ViewStats {
	stats, err := rv.client.GetViewStats(context.Background(), rv.config.ID)
	if err != nil {
		return ViewStats{ID: rv.config.ID}
	}
	return *stats
}

func (rv *RemoteView) UpdateConfig(config ViewConfig) error {
	rv.config = config
	return rv.client.UpdateView(context.Background(), config.ID, config)
}

func (rv *RemoteView) Close() error {
	return nil
}

// Проверяем, что RemoteDatastoreAdapter реализует интерфейс Datastore
var _ Datastore = (*RemoteDatastoreAdapter)(nil)
var _ ds.Datastore = (*RemoteDatastoreAdapter)(nil)
var _ ds.TTLDatastore = (*RemoteDatastoreAdapter)(nil)
var _ ds.GCDatastore = (*RemoteDatastoreAdapter)(nil)
var _ ds.TxnDatastore = (*RemoteDatastoreAdapter)(nil)
var _ ds.Batching = (*RemoteDatastoreAdapter)(nil)

func (r *RemoteDatastoreAdapter) DiskUsage(ctx context.Context) (uint64, error) {
	stats, err := r.client.GetStats(ctx)
	if err != nil {
		return 0, err
	}
	if diskUsage, ok := stats["disk_usage"].(float64); ok {
		return uint64(diskUsage), nil
	}
	return 0, nil
}

func (r *RemoteDatastoreAdapter) Sync(ctx context.Context, prefix ds.Key) error {
	// Для удаленного датастора синхронизация происходит автоматически
	return nil
}

func (r *RemoteDatastoreAdapter) GetTTLStats(ctx context.Context, prefix ds.Key) (*TTLStats, error) {
	return r.client.GetTTLStats(ctx, prefix)
}
