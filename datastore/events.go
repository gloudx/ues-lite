package datastore

import (
	"context"
	"time"

	ds "github.com/ipfs/go-datastore"
)

type EventType int

const (
	EventPut EventType = iota
	EventDelete
	EventBatch
)

type Event struct {
	Type      EventType
	Key       ds.Key
	Value     []byte
	Timestamp time.Time
}

type Subscriber interface {
	OnEvent(ctx context.Context, event Event)
	ID() string
}

type EventHandler func(Event)

type FuncSubscriber struct {
	id      string
	handler EventHandler
}

var _ Subscriber = (*FuncSubscriber)(nil)
var _ Subscriber = (*ChannelSubscriber)(nil)

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
	close(cs.events)
}
