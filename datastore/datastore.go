package datastore

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	badger4 "github.com/ipfs/go-ds-badger4"
)

// TTLMonitorConfig - конфигурация TTL мониторинга
type TTLMonitorConfig struct {
	CheckInterval time.Duration // Интервал проверки истекших ключей
	Enabled       bool          // Включен ли мониторинг
	BufferSize    int           // Размер буфера для TTL событий
}

// TTLKeyInfo - информация о ключе с TTL
type TTLKeyInfo struct {
	Key       ds.Key
	ExpiresAt time.Time
	LastValue []byte // Сохраняем последнее значение для события
}

type Datastore interface {
	ds.Datastore
	ds.BatchingFeature
	ds.TxnFeature
	ds.GCFeature
	ds.PersistentFeature
	ds.TTL
	ViewManager
	//
	Iterator(ctx context.Context, prefix ds.Key, keysOnly bool) (<-chan KeyValue, <-chan error, error)
	Merge(ctx context.Context, other Datastore) error
	Clear(ctx context.Context) error
	Keys(ctx context.Context, prefix ds.Key) (<-chan ds.Key, <-chan error, error)
	//
	Subscribe(subscriber Subscriber)
	Unsubscribe(subscriberID string)
	SubscribeFunc(id string, handler EventHandler)
	SubscribeChannel(id string, buffer int) *ChannelSubscriber
	//
	ListJSSubscriptions(ctx context.Context) ([]jsSubscription, error)
	CreateJSSubscription(ctx context.Context, id, script string, config *JSSubscriberConfig) error
	CreateSimpleJSSubscription(ctx context.Context, id, script string) error
	CreateFilteredJSSubscription(ctx context.Context, id, script string, eventTypes ...EventType) error
	RemoveJSSubscription(ctx context.Context, id string) error
	//
	Close() error
	SetSilentMode(silent bool)
	//
	QueryJQ(ctx context.Context, jqQuery string, opts *JQQueryOptions) (<-chan JQResult, <-chan error, error)
	AggregateJQ(ctx context.Context, jqQuery string, opts *JQQueryOptions) (any, error)
	QueryJQSingle(ctx context.Context, key ds.Key, jqQuery string) (interface{}, error)
	//
	Transform(ctx context.Context, key ds.Key, opts *TransformOptions) (*TransformSummary, error)
	TransformWithJQ(ctx context.Context, key ds.Key, jqExpression string, opts *TransformOptions) (*TransformSummary, error)
	TransformWithPatch(ctx context.Context, key ds.Key, patchOps []PatchOp, opts *TransformOptions) (*TransformSummary, error)
	//
	ExecuteView(ctx context.Context, id string) ([]ViewResult, error)
	GetViewCached(ctx context.Context, id string) ([]ViewResult, bool, error)
	CreateSimpleView(ctx context.Context, id, name, sourcePrefix, script string) (View, error)
	//
	// TTL мониторинг
	EnableTTLMonitoring(config *TTLMonitorConfig) error
	DisableTTLMonitoring() error
	GetTTLMonitorConfig() *TTLMonitorConfig
	GetTTLStats(ctx context.Context, prefix ds.Key) (*TTLStats, error)
	//
	// Методы стримминга
	StreamTo(ctx context.Context, writer io.Writer, opts *StreamOptions) error
	StreamEvents(ctx context.Context, writer io.Writer, opts *StreamOptions) error
	StreamJSON(ctx context.Context, writer io.Writer, prefix ds.Key, includeKeys bool) error
	StreamJSONL(ctx context.Context, writer io.Writer, prefix ds.Key, includeKeys bool) error
	StreamCSV(ctx context.Context, writer io.Writer, prefix ds.Key, includeKeys bool) error
	StreamSSE(ctx context.Context, writer io.Writer, headers map[string]string) error
	StreamBinary(ctx context.Context, writer io.Writer, prefix ds.Key) error
	StreamWithJQ(ctx context.Context, writer io.Writer, jqQuery string, format StreamFormat, prefix ds.Key) error
	NewStreamPipeline(opts *StreamOptions) *StreamPipeline
}

type KeyValue struct {
	Key   ds.Key
	Value []byte
}

var _ Datastore = (*datastorage)(nil)

var _ ds.Datastore = (*datastorage)(nil)
var _ ds.PersistentDatastore = (*datastorage)(nil)
var _ ds.TxnDatastore = (*datastorage)(nil)
var _ ds.TTLDatastore = (*datastorage)(nil)
var _ ds.GCDatastore = (*datastorage)(nil)
var _ ds.Batching = (*datastorage)(nil)

// Обновленная структура datastorage с поддержкой jq
type datastorage struct {
	*badger4.Datastore
	subscribers map[string]Subscriber
	mu          sync.RWMutex
	eventQueue  chan Event
	done        chan struct{}
	wg          sync.WaitGroup
	silentMode  bool
	jqCache     *jqQueryCache
	viewManager ViewManager
	viewOnce    sync.Once
	// TTL мониторинг
	ttlMonitor *TTLMonitorConfig
	ttlKeys    map[string]*TTLKeyInfo // ключи с TTL
	ttlMu      sync.RWMutex
	ttlDone    chan struct{} // для остановки TTL мониторинга
	ttlWg      sync.WaitGroup
}

// Обновляем конструктор
func NewDatastorage(path string, opts *badger4.Options) (Datastore, error) {

	badgerDS, err := badger4.NewDatastore(path, opts)
	if err != nil {
		return nil, err
	}

	ds := &datastorage{
		Datastore:   badgerDS,
		subscribers: make(map[string]Subscriber),
		eventQueue:  make(chan Event, 1000), // Buffer for event queue
		done:        make(chan struct{}),
		jqCache:     newJQQueryCache(), // Инициализируем кэш
		ttlKeys:     make(map[string]*TTLKeyInfo),
		ttlDone:     make(chan struct{}),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := ds.loadJSSubscriptions(ctx); err != nil {
		log.Printf("ошибка загрузки JS подписок: %v", err)
	}

	ds.wg.Add(1)
	go ds.eventDispatcher()

	// Initialize ViewManager with lazy loading
	ds.initViewManager()

	return ds, nil
}

// EnableTTLMonitoring - включает мониторинг TTL
func (s *datastorage) EnableTTLMonitoring(config *TTLMonitorConfig) error {
	if config == nil {
		config = &TTLMonitorConfig{
			CheckInterval: 30 * time.Second,
			Enabled:       true,
			BufferSize:    100,
		}
	}

	s.ttlMu.Lock()
	defer s.ttlMu.Unlock()

	// Останавливаем предыдущий мониторинг если был активен
	if s.ttlMonitor != nil && s.ttlMonitor.Enabled {
		s.stopTTLMonitoring()
	}

	s.ttlMonitor = config

	if config.Enabled {
		s.ttlWg.Add(1)
		go s.ttlMonitorLoop()
	}

	return nil
}

// DisableTTLMonitoring - отключает мониторинг TTL
func (s *datastorage) DisableTTLMonitoring() error {
	s.ttlMu.Lock()
	defer s.ttlMu.Unlock()

	if s.ttlMonitor != nil {
		s.stopTTLMonitoring()
		s.ttlMonitor.Enabled = false
	}

	return nil
}

// GetTTLMonitorConfig - возвращает текущую конфигурацию TTL мониторинга
func (s *datastorage) GetTTLMonitorConfig() *TTLMonitorConfig {
	s.ttlMu.RLock()
	defer s.ttlMu.RUnlock()

	if s.ttlMonitor == nil {
		return nil
	}

	// Возвращаем копию
	return &TTLMonitorConfig{
		CheckInterval: s.ttlMonitor.CheckInterval,
		Enabled:       s.ttlMonitor.Enabled,
		BufferSize:    s.ttlMonitor.BufferSize,
	}
}

// stopTTLMonitoring - внутренний метод для остановки мониторинга (вызывать под mutex)
func (s *datastorage) stopTTLMonitoring() {
	close(s.ttlDone)
	s.ttlWg.Wait()
	s.ttlDone = make(chan struct{}) // создаем новый канал для возможного перезапуска
}

// ttlMonitorLoop - основной цикл мониторинга TTL
func (s *datastorage) ttlMonitorLoop() {
	defer s.ttlWg.Done()

	s.ttlMu.RLock()
	interval := s.ttlMonitor.CheckInterval
	s.ttlMu.RUnlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ttlDone:
			return
		case <-ticker.C:
			s.checkExpiredTTLKeys()
		}
	}
}

// checkExpiredTTLKeys - проверяет истекшие TTL ключи
func (s *datastorage) checkExpiredTTLKeys() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Получаем все ключи и проверяем их TTL
	keysCh, errCh, err := s.Keys(ctx, ds.NewKey("/"))
	if err != nil {
		log.Printf("ошибка получения ключей для TTL проверки: %v", err)
		return
	}

	now := time.Now()
	expiredKeys := make([]ds.Key, 0)

	for {
		select {
		case <-ctx.Done():
			return
		case err, ok := <-errCh:
			if ok && err != nil {
				log.Printf("ошибка при проверке TTL ключей: %v", err)
				return
			}
		case key, ok := <-keysCh:
			if !ok {
				// Генерируем события для всех истекших ключей
				s.processExpiredKeys(ctx, expiredKeys, now)
				return
			}

			// Проверяем TTL для этого ключа
			expiration, err := s.Datastore.GetExpiration(ctx, key)
			if err != nil {
				// Ключ не имеет TTL или ошибка получения
				continue
			}

			if now.After(expiration) {
				expiredKeys = append(expiredKeys, key)
			}
		}
	}
}

// processExpiredKeys - обрабатывает истекшие ключи
func (s *datastorage) processExpiredKeys(ctx context.Context, expiredKeys []ds.Key, expiredAt time.Time) {
	for _, key := range expiredKeys {
		// Пытаемся получить последнее значение перед удалением
		var lastValue []byte
		if value, err := s.Datastore.Get(ctx, key); err == nil {
			lastValue = value
		}

		// Генерируем событие TTL истечения
		s.publishTTLExpiredEvent(key, lastValue, expiredAt)
	}
}

// publishTTLExpiredEvent - публикует событие истечения TTL
func (s *datastorage) publishTTLExpiredEvent(key ds.Key, lastValue []byte, expiredAt time.Time) {
	event := Event{
		Type:      EventTTLExpired,
		Key:       key,
		Value:     lastValue,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"expired_at": expiredAt.Format(time.RFC3339),
			"ttl_event":  true,
		},
	}

	select {
	case s.eventQueue <- event:
	default:
		// Drop event if queue is full to prevent blocking
		log.Printf("TTL event queue full, dropping event for key: %s", key.String())
	}
}

// Переопределяем PutWithTTL для отслеживания TTL ключей
func (s *datastorage) PutWithTTL(ctx context.Context, key ds.Key, value []byte, ttl time.Duration) error {
	err := s.Datastore.PutWithTTL(ctx, key, value, ttl)
	if err != nil {
		return err
	}

	// Публикуем обычное событие Put
	if !s.silentMode {
		s.publishEvent(EventPut, key, value)
	}

	// Если TTL мониторинг включен, регистрируем ключ
	s.ttlMu.RLock()
	monitorEnabled := s.ttlMonitor != nil && s.ttlMonitor.Enabled
	s.ttlMu.RUnlock()

	if monitorEnabled {
		s.registerTTLKey(key, time.Now().Add(ttl), value)
	}

	return nil
}

// registerTTLKey - регистрирует ключ с TTL для мониторинга
func (s *datastorage) registerTTLKey(key ds.Key, expiresAt time.Time, value []byte) {
	s.ttlMu.Lock()
	defer s.ttlMu.Unlock()

	s.ttlKeys[key.String()] = &TTLKeyInfo{
		Key:       key,
		ExpiresAt: expiresAt,
		LastValue: value,
	}
}

func (s *datastorage) eventDispatcher() {
	defer s.wg.Done()
	for {
		select {
		case <-s.done:
			return
		case event := <-s.eventQueue:
			s.mu.RLock()
			subscribers := make(map[string]Subscriber)
			for id, subscriber := range s.subscribers {
				subscribers[id] = subscriber
			}
			s.mu.RUnlock()
			for id, subscriber := range subscribers {
				s.wg.Add(1)
				go func(subID string, sub Subscriber, evt Event) {
					defer s.wg.Done()
					defer func() {
						if r := recover(); r != nil {
							fmt.Printf("panic in subscriber %s: %v\n", subID, r)
						}
					}()
					sub.OnEvent(context.Background(), evt)
				}(id, subscriber, event)
			}
		}
	}
}

func (s *datastorage) publishEvent(eventType EventType, key ds.Key, value []byte) {
	event := Event{
		Type:      eventType,
		Key:       key,
		Value:     value,
		Timestamp: time.Now(),
	}

	select {
	case s.eventQueue <- event:
	default:
		// Drop event if queue is full to prevent blocking
	}
}

func (s *datastorage) SetSilentMode(silent bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.silentMode = silent
}

func (s *datastorage) Subscribe(subscriber Subscriber) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscribers[subscriber.ID()] = subscriber
}

func (s *datastorage) Unsubscribe(subscriberID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subscribers, subscriberID)
}

func (s *datastorage) SubscribeFunc(id string, handler EventHandler) {
	s.Subscribe(NewFuncSubscriber(id, handler))
}

func (s *datastorage) SubscribeChannel(id string, buffer int) *ChannelSubscriber {
	sub := NewChannelSubscriber(id, buffer)
	s.Subscribe(sub)
	return sub
}

func (s *datastorage) Put(ctx context.Context, key ds.Key, value []byte) error {
	err := s.Datastore.Put(ctx, key, value)
	if err == nil {
		if !s.silentMode {
			s.publishEvent(EventPut, key, value)
		}
	}
	return err
}

func (s *datastorage) Delete(ctx context.Context, key ds.Key) error {
	err := s.Datastore.Delete(ctx, key)
	if err == nil {
		if !s.silentMode {
			s.publishEvent(EventDelete, key, nil)
		}
	}

	// Удаляем из TTL мониторинга если есть
	s.ttlMu.Lock()
	delete(s.ttlKeys, key.String())
	s.ttlMu.Unlock()
	return err
}

func (s *datastorage) Iterator(ctx context.Context, prefix ds.Key, keysOnly bool) (<-chan KeyValue, <-chan error, error) {
	q := query.Query{
		Prefix:   prefix.String(),
		KeysOnly: keysOnly,
	}
	result, err := s.Datastore.Query(ctx, q)
	if err != nil {
		return nil, nil, err
	}
	out := make(chan KeyValue)
	errc := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errc)
		defer result.Close()
		for {
			select {
			case <-ctx.Done():
				errc <- ctx.Err()
				return
			case res, ok := <-result.Next():
				if !ok {
					return
				}
				if res.Error != nil {
					errc <- res.Error
					return
				}
				out <- KeyValue{
					Key:   ds.NewKey(res.Key),
					Value: res.Value,
				}
			}
		}
	}()
	return out, errc, nil
}

func (s *datastorage) Merge(ctx context.Context, other Datastore) error {
	batch, err := s.Batch(ctx)
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
			errc = nil
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

func (s *datastorage) Clear(ctx context.Context) error {
	q, err := s.Query(ctx, query.Query{
		KeysOnly: true,
	})
	if err != nil {
		return err
	}
	defer q.Close()
	b, err := s.Batch(ctx)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case res, ok := <-q.Next():
			if !ok {
				return b.Commit(ctx)
			}
			if res.Error != nil {
				return res.Error
			}
			if err := b.Delete(ctx, ds.NewKey(res.Key)); err != nil {
				return err
			}
		}
	}
}

func (s *datastorage) Keys(ctx context.Context, prefix ds.Key) (<-chan ds.Key, <-chan error, error) {
	q := query.Query{
		Prefix:   prefix.String(),
		KeysOnly: true,
	}
	result, err := s.Datastore.Query(ctx, q)
	if err != nil {
		return nil, nil, err
	}
	out := make(chan ds.Key)
	errc := make(chan error, 1)
	go func() {
		defer close(out)
		defer close(errc)
		defer result.Close()
		for {
			select {
			case <-ctx.Done():
				errc <- ctx.Err()
				return
			case res, ok := <-result.Next():
				if !ok {
					return
				}
				if res.Error != nil {
					errc <- res.Error
					return
				}
				out <- ds.NewKey(res.Key)
			}
		}
	}()
	return out, errc, nil
}

// Close method to clean up resources
func (s *datastorage) Close() error {

	// Останавливаем TTL мониторинг
	s.ttlMu.Lock()
	if s.ttlMonitor != nil && s.ttlMonitor.Enabled {
		s.stopTTLMonitoring()
	}
	s.ttlMu.Unlock()

	close(s.done)
	s.wg.Wait()

	s.mu.Lock()
	defer s.mu.Unlock()

	// for event := range s.eventQueue {
	// 	s.mu.RLock()
	// 	subscribers := make(map[string]Subscriber)
	// 	for id, subscriber := range s.subscribers {
	// 		subscribers[id] = subscriber
	// 	}
	// 	s.mu.RUnlock()
	// 	for id, subscriber := range subscribers {
	// 		s.wg.Add(1)
	// 		go func(subID string, sub Subscriber, evt Event) {
	// 			defer s.wg.Done()
	// 			defer func() {
	// 				if r := recover(); r != nil {
	// 					fmt.Printf("panic in subscriber %s: %v\n", subID, r)
	// 				}
	// 			}()
	// 			sub.OnEvent(context.Background(), evt)
	// 		}(id, subscriber, event)
	// 	}
	// }

	// Close all channel subscribers
	for _, subscriber := range s.subscribers {
		if chSub, ok := subscriber.(*ChannelSubscriber); ok {
			chSub.Close()
		}
	}

	// Close ViewManager if initialized
	if s.viewManager != nil {
		if closer, ok := s.viewManager.(*DefaultViewManager); ok {
			if err := closer.Close(); err != nil {
				log.Printf("ошибка закрытия ViewManager: %v", err)
			}
		}
	}

	return s.Datastore.Close()
}

type pubsubBatch struct {
	ds.Batch
	parent     *datastorage
	ops        []batchOp
	silentMode bool
}

type batchOp struct {
	isDelete bool
	key      ds.Key
	value    []byte
}

func (s *datastorage) Batch(ctx context.Context) (ds.Batch, error) {
	batch, err := s.Datastore.Batch(ctx)
	if err != nil {
		return nil, err
	}
	return &pubsubBatch{
		Batch:      batch,
		parent:     s,
		ops:        make([]batchOp, 0),
		silentMode: s.silentMode,
	}, nil
}

func (b *pubsubBatch) Put(ctx context.Context, key ds.Key, value []byte) error {
	err := b.Batch.Put(ctx, key, value)
	if err == nil {
		b.ops = append(b.ops, batchOp{
			isDelete: false,
			key:      key,
			value:    value,
		})
	}
	return err
}

func (b *pubsubBatch) Delete(ctx context.Context, key ds.Key) error {
	err := b.Batch.Delete(ctx, key)
	if err == nil {
		b.ops = append(b.ops, batchOp{
			isDelete: true,
			key:      key,
		})
	}
	return err
}

func (b *pubsubBatch) Commit(ctx context.Context) error {
	err := b.Batch.Commit(ctx)
	if err == nil {
		if !b.silentMode {
			for _, op := range b.ops {
				if op.isDelete {
					b.parent.publishEvent(EventDelete, op.key, nil)
				} else {
					b.parent.publishEvent(EventPut, op.key, op.value)
				}
			}
			b.parent.publishEvent(EventBatch, ds.NewKey("/batch"), nil)
		}
	}
	return err
}

// CreateSimpleJSSubscription creates a JS subscription with default settings
func (s *datastorage) CreateSimpleJSSubscription(ctx context.Context, id, script string) error {
	return s.CreateJSSubscription(ctx, id, script, nil)
}

// CreateFilteredJSSubscription creates a JS subscription for specific event types
func (s *datastorage) CreateFilteredJSSubscription(ctx context.Context, id, script string, eventTypes ...EventType) error {
	config := &JSSubscriberConfig{
		ExecutionTimeout: 5 * time.Second,
		EnableNetworking: true,
		EnableLogging:    true,
		StrictMode:       false,
		EventFilters:     eventTypes,
	}
	return s.CreateJSSubscription(ctx, id, script, config)
}
