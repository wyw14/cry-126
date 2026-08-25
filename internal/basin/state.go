package basin

import (
	"errors"
	"sort"
	"sync"
	"time"
)

type Basin struct {
	ID             string    `json:"id"`
	LevelMeters    float64   `json:"level_meters"`
	TargetMeters   float64   `json:"target_meters"`
	MaximumMeters  float64   `json:"maximum_meters"`
	PreferredPump  string    `json:"preferred_pump"`
	PreferredRoute string    `json:"preferred_route"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type State struct {
	mu     sync.RWMutex
	basins map[string]Basin
}

func NewState(items []Basin) (*State, error) {
	state := &State{basins: make(map[string]Basin, len(items))}
	for _, item := range items {
		if err := validate(item); err != nil {
			return nil, err
		}
		state.basins[item.ID] = item
	}
	return state, nil
}

func (state *State) UpdateLevel(id string, level float64, now time.Time) (Basin, error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	current, exists := state.basins[id]
	if !exists {
		return Basin{}, errors.New("basin is not registered")
	}
	current.LevelMeters = level
	current.UpdatedAt = now.UTC()
	if err := validate(current); err != nil {
		return Basin{}, err
	}
	state.basins[id] = current
	return current, nil
}

func (state *State) Get(id string) (Basin, bool) {
	state.mu.RLock()
	defer state.mu.RUnlock()
	item, exists := state.basins[id]
	return item, exists
}

func (state *State) Snapshot() []Basin {
	state.mu.RLock()
	defer state.mu.RUnlock()
	result := make([]Basin, 0, len(state.basins))
	for _, item := range state.basins {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func Deficit(item Basin) float64 {
	if item.LevelMeters <= item.TargetMeters {
		return 0
	}
	return item.LevelMeters - item.TargetMeters
}

func Urgency(item Basin) float64 {
	span := item.MaximumMeters - item.TargetMeters
	if span <= 0 {
		return 0
	}
	value := Deficit(item) / span
	if value > 1 {
		return 1
	}
	return value
}

func validate(item Basin) error {
	if item.ID == "" || item.PreferredPump == "" || item.PreferredRoute == "" {
		return errors.New("basin routing identity is required")
	}
	if item.MaximumMeters <= item.TargetMeters {
		return errors.New("basin maximum must exceed target")
	}
	if item.UpdatedAt.IsZero() {
		return errors.New("basin update time is required")
	}
	return nil
}
