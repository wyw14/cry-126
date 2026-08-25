package interlock

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wyw14/cry-126/internal/model"
)

type Lock struct {
	ID                  uuid.UUID `json:"id"`
	Name                string    `json:"name"`
	Holder              string    `json:"holder"`
	OperationID         uuid.UUID `json:"operation_id"`
	OperationGeneration uint64    `json:"operation_generation"`
	LockGeneration      uint64    `json:"lock_generation"`
	AcquiredAt          time.Time `json:"acquired_at"`
}

type State struct {
	mu         sync.RWMutex
	locks      map[string]Lock
	generation map[string]uint64
	permit     model.Permit
}

func NewState() *State {
	return &State{locks: make(map[string]Lock), generation: make(map[string]uint64)}
}

func (state *State) Acquire(name, holder string, operation model.Operation, now time.Time) (Lock, error) {
	if name == "" || holder == "" {
		return Lock{}, errors.New("interlock name and holder are required")
	}
	if err := operation.Validate(); err != nil {
		return Lock{}, err
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if current, exists := state.locks[name]; exists {
		if current.OperationID == operation.ID && current.OperationGeneration == operation.Generation && current.Holder == holder {
			return current, nil
		}
		return Lock{}, errors.New("interlock is already held")
	}
	state.generation[name]++
	lock := Lock{
		ID:                  uuid.New(),
		Name:                name,
		Holder:              holder,
		OperationID:         operation.ID,
		OperationGeneration: operation.Generation,
		LockGeneration:      state.generation[name],
		AcquiredAt:          now.UTC(),
	}
	state.locks[name] = lock
	return lock, nil
}

func (state *State) Release(lock Lock) bool {
	state.mu.Lock()
	defer state.mu.Unlock()
	current, exists := state.locks[lock.Name]
	if !exists || current.ID != lock.ID || current.OperationID != lock.OperationID || current.OperationGeneration != lock.OperationGeneration || current.LockGeneration != lock.LockGeneration {
		return false
	}
	delete(state.locks, lock.Name)
	return true
}

func (state *State) Held(name string) (Lock, bool) {
	state.mu.RLock()
	defer state.mu.RUnlock()
	lock, exists := state.locks[name]
	return lock, exists
}

func (state *State) Locks() []Lock {
	state.mu.RLock()
	defer state.mu.RUnlock()
	result := make([]Lock, 0, len(state.locks))
	for _, lock := range state.locks {
		result = append(result, lock)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

func (state *State) SetPermit(permit model.Permit) {
	state.mu.Lock()
	state.permit = permit
	state.mu.Unlock()
}

func (state *State) Permit() model.Permit {
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.permit
}
