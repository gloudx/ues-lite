package datastore

import (
	"time"

	ds "github.com/ipfs/go-datastore"
)

// EventType represents the type of datastore event
type EventType int

const (
	EventPut EventType = iota
	EventDelete
	EventBatch
)

// Event represents a datastore event
type Event struct {
	Type      EventType
	Key       ds.Key
	Value     []byte
	Timestamp time.Time
}

// Subscriber represents an event subscriber
type Subscriber interface {
	OnEvent(event Event)
	ID() string
}

// EventHandler is a function type for handling events
type EventHandler func(Event)

// FuncSubscriber implements Subscriber with a function
type FuncSubscriber struct {
	id      string
	handler EventHandler
}

func NewFuncSubscriber(id string, handler EventHandler) *FuncSubscriber {
	return &FuncSubscriber{
		id:      id,
		handler: handler,
	}
}

func (fs *FuncSubscriber) OnEvent(event Event) {
	fs.handler(event)
}

func (fs *FuncSubscriber) ID() string {
	return fs.id
}

// ChannelSubscriber implements Subscriber with a channel
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

func (cs *ChannelSubscriber) OnEvent(event Event) {
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
