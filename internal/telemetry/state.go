package telemetry

import (
	"sort"
	"sync"

	"github.com/google/uuid"
	"github.com/wyw14/cry-126/internal/model"
)

type State struct {
	mu       sync.RWMutex
	bySource map[string][]model.Evidence
	revision uuid.UUID
	limit    int
}

func NewState(limit int) *State {
	if limit < 2 {
		limit = 2
	}
	return &State{bySource: make(map[string][]model.Evidence), revision: uuid.New(), limit: limit}
}

func (state *State) Add(evidence model.Evidence) error {
	if err := evidence.Validate(); err != nil {
		return err
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	items := append(state.bySource[evidence.SourceID], evidence)
	sort.Slice(items, func(i, j int) bool { return items[i].ObservedAt.Before(items[j].ObservedAt) })
	if len(items) > state.limit {
		items = append([]model.Evidence(nil), items[len(items)-state.limit:]...)
	}
	state.bySource[evidence.SourceID] = items
	state.revision = uuid.New()
	return nil
}

func (state *State) Window(source string) []model.Evidence {
	state.mu.RLock()
	defer state.mu.RUnlock()
	return append([]model.Evidence(nil), state.bySource[source]...)
}

func (state *State) AllLatest() []model.Evidence {
	state.mu.RLock()
	defer state.mu.RUnlock()
	result := make([]model.Evidence, 0, len(state.bySource))
	for _, items := range state.bySource {
		if len(items) > 0 {
			result = append(result, items[len(items)-1])
		}
	}
	return model.SortedEvidence(result)
}

func (state *State) Revision() uuid.UUID {
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.revision
}
