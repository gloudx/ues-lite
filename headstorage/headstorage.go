package headstorage

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ipfs/go-cid"
	ds "github.com/ipfs/go-datastore"
)

type HeadStorage interface {
	LoadHead(ctx context.Context, repoID string) (RepositoryState, error)
	SaveHead(ctx context.Context, repoID string, state RepositoryState) error
	WatchHead(ctx context.Context, repoID string) (<-chan RepositoryState, error)
	Close() error
}
type RepositoryState struct {
	Head      cid.Cid `json:"head"`
	Prev      cid.Cid `json:"prev"`
	RootIndex cid.Cid `json:"root"`
	Version   int     `json:"version"`
	RepoID    string  `json:"repo_id"`
}
type datastoreHeadStorage struct {
	ds       ds.Datastore
	watchers map[string][]chan RepositoryState
	mu       sync.RWMutex
}

func NewHeadStorage(store ds.Datastore) HeadStorage {
	return &datastoreHeadStorage{
		ds:       store,
		watchers: make(map[string][]chan RepositoryState),
	}
}
func (h *datastoreHeadStorage) LoadHead(ctx context.Context, repoID string) (RepositoryState, error) {
	key := ds.NewKey("repository").ChildString(repoID).ChildString("head")
	data, err := h.ds.Get(ctx, key)
	if err != nil {
		if err == ds.ErrNotFound {
			return RepositoryState{
				Head:    cid.Undef,
				Prev:    cid.Undef,
				Version: 1,
				RepoID:  repoID,
			}, nil
		}
		return RepositoryState{}, fmt.Errorf("failed to load head state: %w", err)
	}
	var state RepositoryState
	if err := json.Unmarshal(data, &state); err != nil {
		return RepositoryState{}, fmt.Errorf("failed to unmarshal head state: %w", err)
	}
	return state, nil
}
func (h *datastoreHeadStorage) SaveHead(ctx context.Context, repoID string, state RepositoryState) error {
	key := ds.NewKey("repository").ChildString(repoID).ChildString("head")
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal head state: %w", err)
	}
	if err := h.ds.Put(ctx, key, data); err != nil {
		return fmt.Errorf("failed to save head state: %w", err)
	}
	h.notifyWatchers(repoID, state)
	return nil
}
func (h *datastoreHeadStorage) WatchHead(ctx context.Context, repoID string) (<-chan RepositoryState, error) {
	ch := make(chan RepositoryState, 10)
	h.mu.Lock()
	h.watchers[repoID] = append(h.watchers[repoID], ch)
	h.mu.Unlock()
	go func() {
		<-ctx.Done()
		h.removeWatcher(repoID, ch)
		close(ch)
	}()
	return ch, nil
}
func (h *datastoreHeadStorage) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, watchers := range h.watchers {
		for _, ch := range watchers {
			close(ch)
		}
	}
	h.watchers = make(map[string][]chan RepositoryState)
	return nil
}
func (h *datastoreHeadStorage) notifyWatchers(repoID string, state RepositoryState) {
	h.mu.RLock()
	watchers := h.watchers[repoID]
	h.mu.RUnlock()
	for _, ch := range watchers {
		select {
		case ch <- state:
		default:
		}
	}
}
func (h *datastoreHeadStorage) removeWatcher(repoID string, target chan RepositoryState) {
	h.mu.Lock()
	defer h.mu.Unlock()
	watchers := h.watchers[repoID]
	for i, ch := range watchers {
		if ch == target {
			h.watchers[repoID] = append(watchers[:i], watchers[i+1:]...)
			break
		}
	}
}
