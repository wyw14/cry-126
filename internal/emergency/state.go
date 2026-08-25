package emergency

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

type ClosureStatus string

const (
	ClosurePreparing ClosureStatus = "preparing"
	ClosureRunning   ClosureStatus = "running"
	ClosureSealed    ClosureStatus = "sealed"
	ClosureFailed    ClosureStatus = "failed"
	ClosureRetired   ClosureStatus = "retired"
)

type Closure struct {
	ID                  uuid.UUID     `json:"id"`
	OperationID         uuid.UUID     `json:"operation_id"`
	OperationGeneration uint64        `json:"operation_generation"`
	Status              ClosureStatus `json:"status"`
	RequestedAt         time.Time     `json:"requested_at"`
	CompletedAt         *time.Time    `json:"completed_at,omitempty"`
	Failure             string        `json:"failure,omitempty"`
}

type State struct {
	mu       sync.RWMutex
	closures map[uint64]Closure
}

func NewState() *State {
	return &State{closures: make(map[uint64]Closure)}
}

func (state *State) Begin(operationID uuid.UUID, generation uint64, now time.Time) (Closure, error) {
	if operationID == uuid.Nil || generation == 0 {
		return Closure{}, errors.New("closure operation identity is required")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if current, exists := state.closures[generation]; exists && current.Status != ClosureFailed && current.Status != ClosureRetired {
		return Closure{}, errors.New("closure is already active")
	}
	closure := Closure{ID: uuid.New(), OperationID: operationID, OperationGeneration: generation, Status: ClosurePreparing, RequestedAt: now.UTC()}
	state.closures[generation] = closure
	return closure, nil
}

func (state *State) Update(generation uint64, status ClosureStatus, failure string, now time.Time) (Closure, error) {
	state.mu.Lock()
	defer state.mu.Unlock()
	closure, exists := state.closures[generation]
	if !exists {
		return Closure{}, errors.New("closure is not registered")
	}
	closure.Status = status
	closure.Failure = failure
	if status == ClosureSealed || status == ClosureFailed || status == ClosureRetired {
		completed := now.UTC()
		closure.CompletedAt = &completed
	}
	state.closures[generation] = closure
	return closure, nil
}

func (state *State) Get(generation uint64) (Closure, bool) {
	state.mu.RLock()
	defer state.mu.RUnlock()
	closure, exists := state.closures[generation]
	return closure, exists
}

func (state *State) Snapshot() []Closure {
	state.mu.RLock()
	defer state.mu.RUnlock()
	result := make([]Closure, 0, len(state.closures))
	for _, closure := range state.closures {
		result = append(result, closure)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].OperationGeneration < result[j].OperationGeneration })
	return result
}
