package datastore

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"ues-lite/js"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	badger4 "github.com/ipfs/go-ds-badger4"
	"github.com/itchyny/gojq"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type Datastore interface {
	ds.Datastore
	ds.BatchingFeature
	ds.TxnFeature
	ds.GCFeature
	ds.PersistentFeature
	ds.TTL
	TTLFeature
	SubscriptionFeatures
	EventFeartures
	//
	Iterator(ctx context.Context, prefix ds.Key, keysOnly bool) (<-chan KeyValue, <-chan error, error)
	Merge(ctx context.Context, other Datastore) error
	Clear(ctx context.Context) error
	Keys(ctx context.Context, prefix ds.Key) (<-chan ds.Key, <-chan error, error)
	Close() error
	SetSilentMode(silent bool)
	QueryJQ(ctx context.Context, jqQuery string, opts *JQQueryOptions) (any, error)
	Transform(ctx context.Context, prefix ds.Key, extract string, patchs []string, jqTransform string) error
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
var _ TTLFeature = (*datastorage)(nil)
var _ SubscriptionFeatures = (*datastorage)(nil)
var _ EventFeartures = (*datastorage)(nil)

type datastorage struct {
	*badger4.Datastore
	subscribers map[string]Subscriber
	mu          sync.RWMutex
	eventQueue  chan Event
	done        chan struct{}
	wg          sync.WaitGroup
	silentMode  bool
	// viewOnce         sync.Once
	ttlMonitorConfig *TTLMonitorConfig
	ttlMu            sync.RWMutex
	ttlDone          chan struct{} // для остановки TTL мониторинга
	ttlWg            sync.WaitGroup
	//
	// viewManager ViewManager
}

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
		ttlDone:     make(chan struct{}),
		//
		// jqCache: newJQQueryCache(), // Инициализируем кэш

	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := ds.loadJSSubscriptions(ctx); err != nil {
		log.Printf("ошибка загрузки JS подписок: %v", err)
	}

	ds.wg.Add(1)
	go ds.eventDispatcher()

	// ds.viewManager = NewViewManager(ds)

	return ds, nil
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
	s.ttlMu.Lock()
	if s.ttlMonitorConfig != nil && s.ttlMonitorConfig.Enabled {
		ttlKey := ds.NewKey(TTLNameSpace).ChildString(key.String())
		err := s.Datastore.Delete(ctx, ttlKey)
		if err != nil && err != ds.ErrNotFound {
			log.Printf("ошибка удаления ключа из TTL мониторинга: %v", err)
		}
	}
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

func (s *datastorage) Close() error {

	s.ttlMu.Lock()
	if s.ttlMonitorConfig != nil && s.ttlMonitorConfig.Enabled {
		s.stopTTLMonitoring()
	}
	s.ttlMu.Unlock()

	close(s.done)
	s.wg.Wait()

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, subscriber := range s.subscribers {
		if chSub, ok := subscriber.(*ChannelSubscriber); ok {
			chSub.Close()
		}
	}

	// if s.viewManager != nil {
	// 	if closer, ok := s.viewManager.(*DefaultViewManager); ok {
	// 		if err := closer.Close(); err != nil {
	// 			log.Printf("ошибка закрытия ViewManager: %v", err)
	// 		}
	// 	}
	// }

	return s.Datastore.Close()
}

// --- Transform

const BatchSize = 100

func (s *datastorage) Transform(ctx context.Context, prefix ds.Key, extract string, patchs []string, jqTransform string) error {

	q := query.Query{
		Prefix: prefix.String(),
	}

	results, err := s.Datastore.Query(ctx, q)
	if err != nil {
		return fmt.Errorf("ошибка выполнения запроса: %w", err)
	}
	defer results.Close()

	batch, err := s.Batch(ctx)
	if err != nil {
		return fmt.Errorf("ошибка создания batch: %w", err)
	}

	batchCount := 0

	for {

		select {

		case <-ctx.Done():
			return ctx.Err()

		case res, ok := <-results.Next():

			if !ok {
				if batch != nil && batchCount > 0 {
					if err := batch.Commit(ctx); err != nil {
						return fmt.Errorf("ошибка коммита batch: %w", err)
					}
				}
				return nil
			}

			if res.Error != nil {
				fmt.Printf("Ошибка в результате запроса: %v\n", res.Error)
				continue
			}

			originalData := res.Value

			transformedValue, err := s.applyTransformation(ctx, originalData, extract, patchs, jqTransform)
			if err != nil {
				fmt.Printf("Ошибка трансформации для ключа %s: %v\n", res.Key, err)
				continue
			}

			if err := batch.Put(ctx, ds.NewKey(res.Key), transformedValue); err != nil {
				fmt.Printf("Ошибка добавления в batch для ключа %s: %v\n", res.Key, err)
				continue
			}

			batchCount++

			if batchCount >= BatchSize {
				if err := batch.Commit(ctx); err != nil {
					return fmt.Errorf("ошибка коммита batch: %w", err)
				}
				batch, err = s.Batch(ctx)
				if err != nil {
					return fmt.Errorf("ошибка создания batch: %w", err)
				}
				batchCount = 0
			}
		}
	}
}

func (s *datastorage) applyTransformation(ctx context.Context, jsonBytes []byte, extract string, patch []string, jqTransform string) ([]byte, error) {

	var out []byte = jsonBytes

	if jqTransform != "" {

		query, err := gojq.Parse(jqTransform)
		if err != nil {
			return nil, fmt.Errorf("ошибка парсинга jq-запроса: %w", err)
		}

		code, err := gojq.Compile(query)
		if err != nil {
			return nil, fmt.Errorf("ошибка компиляции jq-запроса: %w", err)
		}

		var input any
		if err := json.Unmarshal(jsonBytes, &input); err != nil {
			return nil, fmt.Errorf("ошибка парсинга JSON для jq: %w", err)
		}

		resultIter := code.RunWithContext(ctx, input)

		results := []any{}

		for {
			result, ok := resultIter.Next()
			if !ok {
				break
			}
			if err, isErr := result.(error); isErr {
				return nil, fmt.Errorf("ошибка выполнения jq-запроса: %w", err)
			}
			results = append(results, result)
		}

		if len(results) >= 1 {
			out, err = json.Marshal(results[0])
			if err != nil {
				return nil, fmt.Errorf("ошибка маршалинга результата jq: %w", err)
			}
		}

	} else {

		if extract != "" {
			out = []byte(gjson.GetBytes(out, extract).String())

		}

		if len(patch) > 0 {
			for _, p := range patch {
				var err error
				k, v, ok := strings.Cut(p, "=")
				if ok {
					var value any = v
					var t string
					t, v, ok = strings.Cut(v, "#")
					if ok {
						switch t {
						case "int":
							value = parseInt(v)
						case "float":
							value = parseFloat(v)
						case "bool":
							value = parseBool(v)
						case "json":
							var jsonVal any
							if err := json.Unmarshal([]byte(v), &jsonVal); err != nil {
								fmt.Printf("⚠️  Ошибка парсинга JSON значения '%s': %v\n", v, err)
								continue
							}
							value = jsonVal
						default:
							value = v
						}
					}
					out, err = sjson.SetBytes(out, k, value)
					if err != nil {
						fmt.Printf("⚠️  Ошибка применения patch '%s': %v\n", p, err)
						continue
					}
				}
			}
		}
	}

	return out, nil
}

// --- Batch with PubSub support

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

// --- Transform

func parseInt(s string) int64 {
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return i
}

func parseFloat(s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0
	}
	return f
}

func parseBool(s string) bool {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return false
	}
	return b
}

// --- JQ Query

type JQQueryOptions struct {
	Prefix  ds.Key
	Limit   int
	Timeout time.Duration
}

type iterator struct {
	opts *JQQueryOptions
	ctx  context.Context
	out  <-chan KeyValue
	errc <-chan error
}

func NewIterator(ctx context.Context, s *datastorage, opts *JQQueryOptions) *iterator {
	if opts == nil {
		opts = &JQQueryOptions{
			Prefix:  ds.NewKey("/"),
			Timeout: 30 * time.Second,
		}
	}
	out, errc, _ := s.Iterator(ctx, opts.Prefix, false)
	return &iterator{opts: opts, ctx: ctx, out: out, errc: errc}
}

func (i *iterator) Next() (any, bool) {

	select {

	case <-i.ctx.Done():
		return nil, false

	case err, ok := <-i.errc:
		if ok && err != nil {
			fmt.Printf("Iterator error: %v\n", err)
		}
		return nil, false

	case kv, ok := <-i.out:
		if !ok {
			return nil, false
		}

		if i.opts.Limit > 0 {
			i.opts.Limit--
			if i.opts.Limit < 0 {
				return nil, false
			}
		}

		var input any
		if err := json.Unmarshal(kv.Value, &input); err != nil {
			fmt.Printf("JSON Unmarshal error for key %s: %v\n", kv.Key.String(), err)
			return nil, false
		}

		return input, true
	}

}

func (s *datastorage) QueryJQ(ctx context.Context, jqQuery string, opts *JQQueryOptions) (any, error) {

	query, err := gojq.Parse(jqQuery)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга jq-запроса: %w", err)
	}

	code, err := gojq.Compile(query, gojq.WithInputIter(NewIterator(ctx, s, opts)))
	if err != nil {
		return nil, fmt.Errorf("ошибка компиляции jq-запроса: %w", err)
	}

	resultIter := code.Run(nil)

	results := []any{}

	for {
		result, ok := resultIter.Next()
		if !ok {
			break
		}
		if err, isErr := result.(error); isErr {
			return nil, fmt.Errorf("ошибка выполнения jq-запроса: %w", err)
		}
		results = append(results, result)
	}

	if len(results) == 0 {
		return nil, nil
	}

	if len(results) == 1 {
		return results[0], nil
	}

	return results, nil
}

// --- Events and Subscribers

type EventType int

const (
	EventPut EventType = iota
	EventDelete
	EventBatch
	EventTTLExpired
)

type Event struct {
	Type      EventType
	Key       ds.Key
	Value     []byte
	Timestamp time.Time
	Metadata  map[string]any
}

type Subscriber interface {
	ID() string
	OnEvent(context.Context, Event)
}

type EventHandler func(Event)

type FuncSubscriber struct {
	id      string
	handler EventHandler
}

// --- NewFuncSubscriber

var _ Subscriber = (*FuncSubscriber)(nil)

func NewFuncSubscriber(id string, handler EventHandler) *FuncSubscriber {
	return &FuncSubscriber{
		id:      id,
		handler: handler,
	}
}

func (fs *FuncSubscriber) OnEvent(ctx context.Context, event Event) {
	fs.handler(event)
}

func (fs *FuncSubscriber) ID() string {
	return fs.id
}

type ChannelSubscriber struct {
	id     string
	events chan Event
	buffer int
}

// --- NewChannelSubscriber

var _ Subscriber = (*ChannelSubscriber)(nil)

func NewChannelSubscriber(id string, buffer int) *ChannelSubscriber {
	return &ChannelSubscriber{
		id:     id,
		events: make(chan Event, buffer),
		buffer: buffer,
	}
}

func (cs *ChannelSubscriber) OnEvent(ctx context.Context, event Event) {
	select {
	case cs.events <- event:
	default:
		// Drop event if buffer is full to prevent blocking
	}
}

func (cs *ChannelSubscriber) ID() string {
	return cs.id
}

func (cs *ChannelSubscriber) Events() <-chan Event {
	return cs.events
}

func (cs *ChannelSubscriber) Close() {
	select {
	case <-cs.events:
		// Уже закрыт
	default:
		close(cs.events)
	}
}

// --- EventFeatures

type EventFeartures interface {
	Subscribe(subscriber Subscriber)
	Unsubscribe(subscriberID string)
	SubscribeFunc(id string, handler EventHandler)
	SubscribeChannel(id string, buffer int) *ChannelSubscriber
	ListJSSubscriptions(ctx context.Context) ([]jsSubscription, error)
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

// --- SubscriptionFeatures

type SubscriptionFeatures interface {
	ListJSSubscriptions(ctx context.Context) ([]jsSubscription, error)
	CreateJSSubscription(ctx context.Context, id, script string, config *JSSubscriberConfig) error
	CreateSimpleJSSubscription(ctx context.Context, id, script string) error
	CreateFilteredJSSubscription(ctx context.Context, id, script string, eventTypes ...EventType) error
	RemoveJSSubscription(ctx context.Context, id string) error
}

const (
	SubscriptionsNamespace = "/_system/ds-subscriptions"
)

type jsSubscription struct {
	ID               string      `json:"id"`
	EventFilters     []EventType `json:"event_filters"`
	Script           string      `json:"script"`
	ExecutionTimeout int64       `json:"execution_timeout"` // milliseconds
	EnableNetworking bool        `json:"enable_networking"`
	EnableLogging    bool        `json:"enable_logging"`
	StrictMode       bool        `json:"strict_mode"`
	CreatedAt        time.Time   `json:"created_at"`
}

func (s *datastorage) CreateJSSubscription(ctx context.Context, id, script string, config *JSSubscriberConfig) error {

	if id == "" {
		return fmt.Errorf("subscription ID cannot be empty")
	}

	if config == nil {
		config = &JSSubscriberConfig{
			ExecutionTimeout: 5 * time.Second,
			EnableNetworking: true,
			EnableLogging:    true,
			StrictMode:       false,
		}
	}

	config.ID = id
	config.Script = script

	subscriber, err := NewJSSubscriber(config)
	if err != nil {
		return fmt.Errorf("failed to create JS subscriber: %w", err)
	}

	savedSub := jsSubscription{
		ID:               id,
		Script:           script,
		ExecutionTimeout: config.ExecutionTimeout.Milliseconds(),
		EnableNetworking: config.EnableNetworking,
		EnableLogging:    config.EnableLogging,
		EventFilters:     config.EventFilters,
		StrictMode:       config.StrictMode,
		CreatedAt:        time.Now(),
	}

	data, err := json.Marshal(savedSub)
	if err != nil {
		return fmt.Errorf("failed to marshal subscription: %w", err)
	}

	key := ds.NewKey(SubscriptionsNamespace).ChildString(id)
	if err := s.Datastore.Put(ctx, key, data); err != nil {
		return fmt.Errorf("failed to save subscription: %w", err)
	}

	s.mu.Lock()
	if existing, exists := s.subscribers[id]; exists {
		if chSub, ok := existing.(*ChannelSubscriber); ok {
			chSub.Close()
		}
		delete(s.subscribers, id)
	}

	s.subscribers[id] = subscriber
	s.mu.Unlock()

	return nil
}

func (s *datastorage) CreateSimpleJSSubscription(ctx context.Context, id, script string) error {
	return s.CreateJSSubscription(ctx, id, script, nil)
}

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

func (s *datastorage) RemoveJSSubscription(ctx context.Context, id string) error {
	if id == "" {
		return fmt.Errorf("subscription ID cannot be empty")
	}
	if _, ok := s.subscribers[id]; !ok {
		return fmt.Errorf("subscription with ID %s does not exist", id)
	}
	key := ds.NewKey(SubscriptionsNamespace).ChildString(id)
	if err := s.Datastore.Delete(ctx, key); err != nil {
		return fmt.Errorf("failed to delete subscription: %w", err)
	}
	s.mu.Lock()
	delete(s.subscribers, id)
	s.mu.Unlock()
	return nil
}

func (s *datastorage) ListJSSubscriptions(ctx context.Context) ([]jsSubscription, error) {

	q := query.Query{
		Prefix: SubscriptionsNamespace,
	}

	results, err := s.Datastore.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("failed to query subscriptions: %w", err)
	}
	defer results.Close()

	var subscriptions []jsSubscription

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case result, ok := <-results.Next():
			if !ok {
				return subscriptions, nil
			}
			if result.Error != nil {
				return nil, result.Error
			}
			var savedSub jsSubscription
			if err := json.Unmarshal(result.Value, &savedSub); err != nil {
				// Skip invalid entries
				continue
			}

			subscriptions = append(subscriptions, savedSub)
		}
	}
}

func (s *datastorage) loadJSSubscriptions(ctx context.Context) error {

	subscriptions, err := s.ListJSSubscriptions(ctx)
	if err != nil {
		return fmt.Errorf("failed to list subscriptions: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	loadedCount := 0
	for _, savedSub := range subscriptions {
		config := &JSSubscriberConfig{
			ID:               savedSub.ID,
			Script:           savedSub.Script,
			ExecutionTimeout: time.Duration(savedSub.ExecutionTimeout) * time.Millisecond,
			EnableNetworking: savedSub.EnableNetworking,
			EnableLogging:    savedSub.EnableLogging,
			EventFilters:     savedSub.EventFilters,
			StrictMode:       savedSub.StrictMode,
		}

		jsSubscriber, err := NewJSSubscriber(config)
		if err != nil {
			// Skip invalid subscriptions but continue loading others
			log.Printf("failed to recreate subscription %s: %v", savedSub.ID, err)
			continue
		}

		if existing, exists := s.subscribers[savedSub.ID]; exists {
			if chSub, ok := existing.(*ChannelSubscriber); ok {
				chSub.Close()
			}
			delete(s.subscribers, savedSub.ID)
		}

		s.subscribers[savedSub.ID] = jsSubscriber

		loadedCount++
	}

	return nil
}

// --- JSSubscriber

type jsEvent struct {
	Type      string
	Key       string
	Value     string
	Timestamp time.Time
	Metadata  map[string]interface{} `json:"metadata"`
}

type JSSubscriberConfig struct {
	ID               string
	Script           string
	ExecutionTimeout time.Duration
	EnableNetworking bool
	EnableLogging    bool
	CustomLibraries  map[string]interface{}
	EventFilters     []EventType
	StrictMode       bool
}

type jsSubscriber struct {
	id     string
	script string
	config *JSSubscriberConfig
	mu     sync.RWMutex
	logger *log.Logger
}

var _ Subscriber = (*jsSubscriber)(nil)

func NewJSSubscriber(config *JSSubscriberConfig) (*jsSubscriber, error) {
	if config.ID == "" {
		return nil, fmt.Errorf("subscriber ID cannot be empty")
	}

	if config.ExecutionTimeout <= 0 {
		config.ExecutionTimeout = 5 * time.Second
	}

	subscriber := &jsSubscriber{
		id:     config.ID,
		script: config.Script,
		config: config,
	}

	if config.EnableLogging {
		subscriber.logger = log.New(log.Writer(), fmt.Sprintf("[JS-%s] ", config.ID), log.LstdFlags)
	}

	return subscriber, nil
}

func NewSimpleJSSubscriber(id, script string) (*jsSubscriber, error) {
	config := &JSSubscriberConfig{
		ID:               id,
		Script:           script,
		ExecutionTimeout: 5 * time.Second,
		EnableLogging:    true,
		EnableNetworking: true,
	}
	return NewJSSubscriber(config)
}

func NewFilteredJSSubscriber(id, script string, eventTypes ...EventType) (*jsSubscriber, error) {
	config := &JSSubscriberConfig{
		ID:               id,
		Script:           script,
		ExecutionTimeout: 5 * time.Second,
		EnableLogging:    true,
		EnableNetworking: true,
		EventFilters:     eventTypes,
	}
	return NewJSSubscriber(config)
}

func (s *jsSubscriber) ID() string {
	return s.id
}

func (s *jsSubscriber) OnEvent(ctx context.Context, event Event) {
	// Проверяем фильтры событий
	if len(s.config.EventFilters) > 0 {
		found := false
		for _, filter := range s.config.EventFilters {
			if event.Type == filter {
				found = true
				break
			}
		}
		if !found {
			return
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, s.config.ExecutionTimeout)
	defer cancel()

	done := make(chan bool, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if s.config.EnableLogging && s.logger != nil {
					s.logger.Printf("Script execution panic: %v", r)
				}
			}
			done <- true
		}()
		s.executeScript(ctx, event)
	}()

	select {
	case <-done:
		// Script completed
	case <-ctx.Done():
		if s.config.EnableLogging && s.logger != nil {
			s.logger.Printf("Script execution timeout for event %s", event.Key.String())
		}
	}
}

func (s *jsSubscriber) executeScript(ctx context.Context, event Event) {
	eventType := s.eventTypeToString(event.Type)

	var valueStr string
	if event.Value != nil {
		valueStr = string(event.Value)
	}

	// Создаем JS событие
	e := jsEvent{
		Type:      eventType,
		Key:       event.Key.String(),
		Value:     valueStr,
		Timestamp: event.Timestamp,
		Metadata:  make(map[string]interface{}),
	}

	// Копируем метаданные из события, если они есть
	if event.Metadata != nil {
		for k, v := range event.Metadata {
			e.Metadata[k] = v
		}
	}

	// Для TTL событий добавляем специальные метаданные
	if event.Type == EventTTLExpired {
		e.Metadata["is_ttl_expired"] = true
		// expired_at уже должен быть в event.Metadata
	}

	if s.script != "" {
		_, err := js.Eval(ctx, s.script, map[string]any{
			"event": e,
		})
		if err != nil && s.config.EnableLogging && s.logger != nil {
			s.logger.Printf("Script execution error: %v", err)
		}
	}
}

// eventTypeToString - конвертирует тип события в строку
func (s *jsSubscriber) eventTypeToString(eventType EventType) string {
	switch eventType {
	case EventPut:
		return "put"
	case EventDelete:
		return "delete"
	case EventBatch:
		return "batch"
	case EventTTLExpired:
		return "ttl_expired"
	default:
		return "unknown"
	}
}

// --- TTLFeature

const (
	TTLNameSpace = "/_system/ds-ttls"
)

type TTLFeature interface {
	EnableTTLMonitoring(config *TTLMonitorConfig) error
	ListTTLKeys(ctx context.Context) ([]TTLKeyStatus, error)
	ExtendTTL(ctx context.Context, key ds.Key, extension time.Duration) error
	CleanupExpiredKeys(ctx context.Context) (int, error)
	SetTTLBatch(ctx context.Context, keys []ds.Key, ttl time.Duration) error
	GetExpiringKeys(ctx context.Context, prefix ds.Key, within time.Duration) ([]TTLKeyStatus, error)
}

type TTLMonitorConfig struct {
	CheckInterval time.Duration // Интервал проверки истекших ключей
	Enabled       bool          // Включен ли мониторинг
	BufferSize    int           // Размер буфера для TTL событий
}

// TTLKeyStatus - статус ключа с TTL
type TTLKeyStatus struct {
	Key       ds.Key
	ExpiresAt *time.Time
	TimeLeft  time.Duration
	IsExpired bool
	HasTTL    bool
}

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
	if s.ttlMonitorConfig != nil && s.ttlMonitorConfig.Enabled {
		s.stopTTLMonitoring()
	}
	s.ttlMonitorConfig = config
	if config.Enabled {
		s.ttlWg.Add(1)
		go s.ttlMonitorLoop()
	}
	return nil
}

func (s *datastorage) stopTTLMonitoring() {
	select {
	case <-s.ttlDone:
	default:
		close(s.ttlDone)
	}
	s.ttlWg.Wait()
	s.ttlDone = make(chan struct{})
}

func (s *datastorage) ttlMonitorLoop() {
	defer s.ttlWg.Done()
	ticker := time.NewTicker(s.ttlMonitorConfig.CheckInterval)
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

func (s *datastorage) checkExpiredTTLKeys() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	keysCh, errCh, err := s.Keys(ctx, ds.NewKey(TTLNameSpace))
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
				s.processExpiredKeys(ctx, expiredKeys, now)
				return
			}
			originalKeyStr := key.String()[len(TTLNameSpace)+1:] // +1 для слэша
			key = ds.NewKey(originalKeyStr)
			expiration, err := s.Datastore.GetExpiration(ctx, key)
			if err != nil {
				if err == ds.ErrNotFound {
					expiredKeys = append(expiredKeys, key)
				}
				continue
			}
			if now.After(expiration) {
				expiredKeys = append(expiredKeys, key)
			}
		}
	}
}

func (s *datastorage) processExpiredKeys(ctx context.Context, expiredKeys []ds.Key, expiredAt time.Time) {
	for _, key := range expiredKeys {
		var lastValue []byte
		if value, err := s.Datastore.Get(ctx, key); err == nil {
			lastValue = value
		}
		err := s.Datastore.Delete(ctx, key)
		if err != nil {
			log.Printf("ошибка удаления истекшего ключа %s: %v", key.String(), err)
		}
		ttlKey := ds.NewKey(TTLNameSpace).ChildString(key.String())
		err = s.Datastore.Delete(ctx, ttlKey)
		if err != nil {
			log.Printf("ошибка удаления TTL ключа %s: %v", ttlKey.String(), err)
		}
		if s.silentMode {
			continue
		}
		s.publishTTLExpiredEvent(key, lastValue, expiredAt)
	}
}

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
		log.Printf("TTL event queue full, dropping event for key: %s", key.String())
	}
}

func (s *datastorage) PutWithTTL(ctx context.Context, key ds.Key, value []byte, ttl time.Duration) error {
	err := s.Datastore.PutWithTTL(ctx, key, value, ttl)
	if err != nil {
		return err
	}
	if !s.silentMode {
		s.publishEvent(EventPut, key, value)
	}
	if s.ttlMonitorConfig != nil && s.ttlMonitorConfig.Enabled {
		s.registerTTLKey(key, time.Now().Add(ttl))
	}
	return nil
}

func (s *datastorage) registerTTLKey(key ds.Key, expiresAt time.Time) {
	s.ttlMu.Lock()
	defer s.ttlMu.Unlock()
	ttlKey := ds.NewKey(TTLNameSpace).ChildString(key.String())
	err := s.Datastore.Put(context.Background(), ttlKey, []byte(expiresAt.Format(time.RFC3339)))
	if err != nil {
		log.Printf("ошибка регистрации TTL ключа %s: %v", key.String(), err)
	}
}

func (s *datastorage) ListTTLKeys(ctx context.Context) ([]TTLKeyStatus, error) {
	var results []TTLKeyStatus
	keysCh, errCh, err := s.Keys(ctx, ds.NewKey(TTLNameSpace))
	if err != nil {
		return nil, fmt.Errorf("ошибка получения ключей: %w", err)
	}
	now := time.Now()
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case err, ok := <-errCh:
			if ok && err != nil {
				return nil, err
			}
		case key, ok := <-keysCh:
			if !ok {
				sort.Slice(results, func(i, j int) bool {
					if results[i].ExpiresAt == nil && results[j].ExpiresAt == nil {
						return results[i].Key.String() < results[j].Key.String()
					}
					if results[i].ExpiresAt == nil {
						return false
					}
					if results[j].ExpiresAt == nil {
						return true
					}
					return results[i].ExpiresAt.Before(*results[j].ExpiresAt)
				})
				return results, nil
			}
			originalKeyStr := key.String()[len(TTLNameSpace)+1:] // +1 для слэша
			key = ds.NewKey(originalKeyStr)
			status := TTLKeyStatus{
				Key: key,
			}
			expiration, err := s.Datastore.GetExpiration(ctx, key)
			if err != nil {
				continue
			}
			status.HasTTL = true
			status.ExpiresAt = &expiration
			status.TimeLeft = time.Until(expiration)
			status.IsExpired = now.After(expiration)
			results = append(results, status)
		}
	}
}

func (s *datastorage) ExtendTTL(ctx context.Context, key ds.Key, extension time.Duration) error {
	exists, err := s.Datastore.Has(ctx, key)
	if err != nil {
		return fmt.Errorf("ошибка проверки существования ключа: %w", err)
	}
	if !exists {
		return fmt.Errorf("ключ %s не существует", key.String())
	}
	currentExpiration, err := s.Datastore.GetExpiration(ctx, key)
	if err != nil {
		return fmt.Errorf("ошибка получения текущего TTL: %w", err)
	}
	now := time.Now()
	currentTTL := time.Until(currentExpiration)
	newTTL := currentTTL + extension
	if currentTTL <= 0 {
		newTTL = extension
	}
	err = s.Datastore.SetTTL(ctx, key, newTTL)
	if err != nil {
		return fmt.Errorf("ошибка установки нового TTL: %w", err)
	}
	if s.ttlMonitorConfig != nil && s.ttlMonitorConfig.Enabled {
		if _, err := s.Datastore.Get(ctx, key); err == nil {
			s.registerTTLKey(key, now.Add(newTTL))
		}
	}
	return nil
}

func (s *datastorage) SetTTL(ctx context.Context, key ds.Key, originalTTL time.Duration) error {
	exists, err := s.Datastore.Has(ctx, key)
	if err != nil {
		return fmt.Errorf("ошибка проверки существования ключа: %w", err)
	}
	if !exists {
		return fmt.Errorf("ключ %s не существует", key.String())
	}
	err = s.Datastore.SetTTL(ctx, key, originalTTL)
	if err != nil {
		return fmt.Errorf("ошибка обновления TTL: %w", err)
	}
	s.ttlMu.RLock()
	monitorEnabled := s.ttlMonitorConfig != nil && s.ttlMonitorConfig.Enabled
	s.ttlMu.RUnlock()
	if monitorEnabled {
		if _, err := s.Datastore.Get(ctx, key); err == nil {
			s.registerTTLKey(key, time.Now().Add(originalTTL))
		}
	}
	return nil
}

func (s *datastorage) CleanupExpiredKeys(ctx context.Context) (int, error) {
	keysCh, errCh, err := s.Keys(ctx, ds.NewKey(TTLNameSpace))
	if err != nil {
		return 0, fmt.Errorf("ошибка получения ключей: %w", err)
	}
	now := time.Now()
	expiredKeys := []ds.Key{}
	for {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case err, ok := <-errCh:
			if ok && err != nil {
				return 0, err
			}
		case key, ok := <-keysCh:
			if !ok {
				goto cleanup
			}
			originalKeyStr := key.String()[len(TTLNameSpace)+1:] // +1 для слэша
			key = ds.NewKey(originalKeyStr)
			expiration, err := s.Datastore.GetExpiration(ctx, key)
			if err != nil {
				if err == ds.ErrNotFound {
					expiredKeys = append(expiredKeys, key)
				}
				continue
			}
			if now.After(expiration) {
				expiredKeys = append(expiredKeys, key)
			}
		}
	}

cleanup:
	if len(expiredKeys) == 0 {
		return 0, nil
	}
	batch, err := s.Batch(ctx)
	if err != nil {
		return 0, fmt.Errorf("ошибка создания batch: %w", err)
	}
	for _, key := range expiredKeys {
		var lastValue []byte
		if value, err := s.Datastore.Get(ctx, key); err == nil {
			lastValue = value
		}
		err = batch.Delete(ctx, key)
		if err != nil {
			return len(expiredKeys), fmt.Errorf("ошибка добавления в batch: %w", err)
		}
		ttlKey := ds.NewKey(TTLNameSpace).ChildString(key.String())
		err = batch.Delete(ctx, ttlKey)
		if err != nil {
			return len(expiredKeys), fmt.Errorf("ошибка удаления TTL ключа из batch: %w", err)
		}
		if !s.silentMode {
			s.publishTTLExpiredEvent(key, lastValue, now)
		}
	}
	err = batch.Commit(ctx)
	if err != nil {
		return len(expiredKeys), fmt.Errorf("ошибка коммита batch: %w", err)
	}
	return len(expiredKeys), nil
}

func (s *datastorage) SetTTLBatch(ctx context.Context, keys []ds.Key, ttl time.Duration) error {
	for _, key := range keys {
		err := s.Datastore.SetTTL(ctx, key, ttl)
		if err != nil {
			return fmt.Errorf("ошибка установки TTL для ключа %s: %w", key.String(), err)
		}
		if s.ttlMonitorConfig != nil && s.ttlMonitorConfig.Enabled {
			if _, err := s.Datastore.Get(ctx, key); err == nil {
				s.registerTTLKey(key, time.Now().Add(ttl))
			}
		}
	}
	return nil
}

func (s *datastorage) GetExpiringKeys(ctx context.Context, prefix ds.Key, within time.Duration) ([]TTLKeyStatus, error) {
	allKeys, err := s.ListTTLKeys(ctx) // только ключи с TTL
	if err != nil {
		return nil, err
	}
	var expiringKeys []TTLKeyStatus
	now := time.Now()
	for _, keyStatus := range allKeys {
		if keyStatus.HasTTL && keyStatus.ExpiresAt != nil {
			if keyStatus.ExpiresAt.After(now) && keyStatus.TimeLeft <= within {
				expiringKeys = append(expiringKeys, keyStatus)
			}
		}
	}
	return expiringKeys, nil
}
