package operation

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/wyw14/cry-126/internal/model"
)

type State struct {
	mu        sync.RWMutex
	current   model.Operation
	completed []model.Operation
	next      uint64
}

func NewState(now time.Time) *State {
	return &State{
		current: model.Operation{
			Phase:     model.PhaseStandby,
			StartedAt: now.UTC(),
			UpdatedAt: now.UTC(),
		},
		next: 1,
	}
}

func (state *State) Start(reason string, now time.Time) (model.Operation, error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if reason == "" {
		return model.Operation{}, errors.New("operation reason is required")
	}
	if state.current.Generation != 0 && state.current.Phase != model.PhaseStandby && state.current.Phase != model.PhaseClosed {
		return model.Operation{}, fmt.Errorf("operation %s is still active", model.OperationIdentity(state.current))
	}
	operation := model.NewOperation(state.next, reason, now)
	state.next++
	state.current = operation
	return operation, nil
}

func (state *State) Transition(generation uint64, phase model.OperationPhase, now time.Time) (model.Operation, error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.current.Generation != generation {
		return model.Operation{}, errors.New("operation generation is retired")
	}
	updated, err := model.TransitionOperation(state.current, phase, now)
	if err != nil {
		return model.Operation{}, err
	}
	state.current = updated
	if phase == model.PhaseClosed {
		state.completed = append(state.completed, updated)
	}
	return updated, nil
}

func (state *State) Current() model.Operation {
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.current
}

func (state *State) IsCurrent(operation model.Operation) bool {
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.current.ID == operation.ID && state.current.Generation == operation.Generation
}

func (state *State) Completed() []model.Operation {
	state.mu.RLock()
	defer state.mu.RUnlock()
	return append([]model.Operation(nil), state.completed...)
}

func (state *State) Restore(current model.Operation, completed []model.Operation) error {
	if current.Generation != 0 {
		if err := current.Validate(); err != nil {
			return err
		}
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	state.current = current
	state.completed = append([]model.Operation(nil), completed...)
	state.next = current.Generation + 1
	for _, operation := range completed {
		if operation.Generation >= state.next {
			state.next = operation.Generation + 1
		}
	}
	return nil
}
