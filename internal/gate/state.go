package gate

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/wyw14/cry-126/internal/model"
)

type State struct {
	mu    sync.RWMutex
	gates map[string]model.Gate
}

func NewState(ids []string, now time.Time) *State {
	items := make(map[string]model.Gate, len(ids))
	for _, id := range ids {
		items[id] = model.Gate{ID: id, Position: model.GateOpen, OpeningPercent: 100, LastFeedbackAt: now.UTC()}
	}
	return &State{gates: items}
}

func (state *State) Command(id string, operation model.Operation, epoch uint64, target model.GatePosition, opening int, now time.Time) (model.Gate, error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	current, exists := state.gates[id]
	if !exists {
		return model.Gate{}, errors.New("gate is not registered")
	}
	if epoch <= current.CommandEpoch && current.OperationGeneration == operation.Generation {
		return model.Gate{}, errors.New("gate command epoch must advance")
	}
	current.OperationID = operation.ID
	current.OperationGeneration = operation.Generation
	current.CommandEpoch = epoch
	current.Position = model.GateMoving
	current.OpeningPercent = opening
	current.LastFeedbackAt = now.UTC()
	current.Failure = ""
	if target == model.GateClosed {
		current.OpeningPercent = 0
	}
	if target == model.GateOpen {
		current.OpeningPercent = 100
	}
	state.gates[id] = current
	return current, nil
}

func (state *State) Acknowledge(id string, operation model.Operation, epoch uint64, position model.GatePosition, opening int, failure string, now time.Time) (model.Gate, error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	current, exists := state.gates[id]
	if !exists {
		return model.Gate{}, errors.New("gate is not registered")
	}
	if current.OperationID != operation.ID || current.OperationGeneration != operation.Generation || current.CommandEpoch != epoch {
		return model.Gate{}, errors.New("gate feedback belongs to a retired command")
	}
	current.Position = position
	current.OpeningPercent = opening
	current.Failure = failure
	current.LastFeedbackAt = now.UTC()
	if position == model.GateFailed && failure == "" {
		current.Failure = "gate command failed"
	}
	if err := current.Validate(); err != nil {
		return model.Gate{}, err
	}
	state.gates[id] = current
	return current, nil
}

func (state *State) Get(id string) (model.Gate, bool) {
	state.mu.RLock()
	defer state.mu.RUnlock()
	gate, exists := state.gates[id]
	return gate, exists
}

func (state *State) Snapshot() []model.Gate {
	state.mu.RLock()
	defer state.mu.RUnlock()
	result := make([]model.Gate, 0, len(state.gates))
	for _, item := range state.gates {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (state *State) Restore(items []model.Gate) error {
	restored := make(map[string]model.Gate, len(items))
	for _, item := range items {
		if item.OperationGeneration != 0 {
			if err := item.Validate(); err != nil {
				return err
			}
		}
		restored[item.ID] = item
	}
	state.mu.Lock()
	state.gates = restored
	state.mu.Unlock()
	return nil
}
