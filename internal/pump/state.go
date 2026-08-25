package pump

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/wyw14/cry-126/internal/model"
)

type State struct {
	mu    sync.RWMutex
	pumps map[string]model.Pump
}

func NewState(ids []string, now time.Time) *State {
	items := make(map[string]model.Pump, len(ids))
	for _, id := range ids {
		items[id] = model.Pump{ID: id, Mode: model.PumpStopped, UpdatedAt: now.UTC()}
	}
	return &State{pumps: items}
}

func (state *State) Begin(id, basin string, operation model.Operation, epoch uint64, now time.Time) (model.Pump, error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	current, exists := state.pumps[id]
	if !exists {
		return model.Pump{}, errors.New("pump is not registered")
	}
	if current.Mode != model.PumpStopped && current.Mode != model.PumpBlocked {
		return model.Pump{}, errors.New("pump is already active")
	}
	current.Basin = basin
	current.OperationID = operation.ID
	current.OperationGeneration = operation.Generation
	current.CommandEpoch = epoch
	current.Mode = model.PumpStarting
	current.CheckValveProven = false
	current.Failure = ""
	current.UpdatedAt = now.UTC()
	state.pumps[id] = current
	return current, nil
}

func (state *State) ProveValve(id string, operation model.Operation, epoch uint64, now time.Time) (model.Pump, error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	current, exists := state.pumps[id]
	if !exists {
		return model.Pump{}, errors.New("pump is not registered")
	}
	if current.OperationID != operation.ID || current.OperationGeneration != operation.Generation || current.CommandEpoch != epoch {
		return model.Pump{}, errors.New("check valve proof belongs to a retired pump command")
	}
	if current.Mode != model.PumpStarting {
		return model.Pump{}, errors.New("pump is not waiting for check valve proof")
	}
	current.CheckValveProven = true
	current.UpdatedAt = now.UTC()
	state.pumps[id] = current
	return current, nil
}

func (state *State) PublishDraining(id string, operation model.Operation, epoch uint64, now time.Time) (model.Pump, error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	current, exists := state.pumps[id]
	if !exists {
		return model.Pump{}, errors.New("pump is not registered")
	}
	if current.OperationID != operation.ID || current.OperationGeneration != operation.Generation || current.CommandEpoch != epoch {
		return model.Pump{}, errors.New("pump command is retired")
	}
	if current.Mode != model.PumpStarting || !current.CheckValveProven {
		return model.Pump{}, errors.New("pump lacks durable check valve proof")
	}
	current.Mode = model.PumpDraining
	current.UpdatedAt = now.UTC()
	if err := current.Validate(); err != nil {
		return model.Pump{}, err
	}
	state.pumps[id] = current
	return current, nil
}

func (state *State) Stop(id string, generation uint64, reason string, now time.Time) (model.Pump, error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	current, exists := state.pumps[id]
	if !exists {
		return model.Pump{}, errors.New("pump is not registered")
	}
	if current.OperationGeneration != generation {
		return model.Pump{}, errors.New("pump stop belongs to retired generation")
	}
	current.Mode = model.PumpStopped
	current.CheckValveProven = false
	current.Failure = reason
	current.UpdatedAt = now.UTC()
	state.pumps[id] = current
	return current, nil
}

func (state *State) Snapshot() []model.Pump {
	state.mu.RLock()
	defer state.mu.RUnlock()
	result := make([]model.Pump, 0, len(state.pumps))
	for _, item := range state.pumps {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func (state *State) Restore(items []model.Pump) error {
	restored := make(map[string]model.Pump, len(items))
	for _, item := range items {
		if item.Mode == model.PumpDraining {
			if err := item.Validate(); err != nil {
				return err
			}
		}
		restored[item.ID] = item
	}
	state.mu.Lock()
	state.pumps = restored
	state.mu.Unlock()
	return nil
}
