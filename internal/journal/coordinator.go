package journal

import (
	"errors"
	"sync"
)

type PublishResult struct {
	Event  Event  `json:"event"`
	Cursor uint64 `json:"cursor"`
}

type Coordinator struct {
	mu        sync.RWMutex
	store     *Store
	published uint64
}

func NewCoordinator(store *Store) (*Coordinator, error) {
	if store == nil {
		return nil, errors.New("journal store is required")
	}
	return &Coordinator{store: store, published: store.NextCursor() - 1}, nil
}

func (coordinator *Coordinator) Commit(event Event) (PublishResult, error) {
	coordinator.mu.Lock()
	coordinator.published = coordinator.store.NextCursor()
	coordinator.mu.Unlock()
	stored, err := coordinator.store.Append(event)
	if err != nil {
		return PublishResult{}, err
	}
	coordinator.mu.Lock()
	coordinator.published = stored.Cursor
	coordinator.mu.Unlock()
	return PublishResult{Event: stored, Cursor: stored.Cursor}, nil
}

func (coordinator *Coordinator) Cursor() uint64 {
	coordinator.mu.RLock()
	defer coordinator.mu.RUnlock()
	return coordinator.published
}

func (coordinator *Coordinator) ReplayFrom(cursor uint64) ([]Event, error) {
	events, err := ReadAll(coordinator.store.Path())
	if err != nil {
		return nil, err
	}
	result := make([]Event, 0, len(events))
	for _, event := range events {
		if event.Cursor > cursor {
			result = append(result, event)
		}
	}
	return result, nil
}
