package datastore

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	ds "github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	badger4 "github.com/ipfs/go-ds-badger4"
)

type Datastore interface {
	ds.Datastore
	ds.BatchingFeature
	ds.TxnFeature
	ds.GCFeature
	ds.PersistentFeature
	ds.TTL
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
}

type KeyValue struct {
	Key   ds.Key
	Value []byte
}

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
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := ds.loadJSSubscriptions(ctx); err != nil {
		log.Printf("ошибка загрузки JS подписок: %v", err)
	}

	ds.wg.Add(1)
	go ds.eventDispatcher()

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
