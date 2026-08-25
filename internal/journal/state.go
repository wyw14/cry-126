package journal

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type RecoveryBarrier struct {
	OperationID         uuid.UUID `json:"operation_id"`
	OperationGeneration uint64    `json:"operation_generation"`
	Name                string    `json:"name"`
	DurableCursor       uint64    `json:"durable_cursor"`
	Completed           bool      `json:"completed"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type BarrierState struct {
	mu       sync.RWMutex
	barriers map[string]RecoveryBarrier
}

func NewBarrierState() *BarrierState {
	return &BarrierState{barriers: make(map[string]RecoveryBarrier)}
}

func (state *BarrierState) Put(barrier RecoveryBarrier) error {
	if err := barrier.Validate(); err != nil {
		return err
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	current, exists := state.barriers[barrier.Name]
	if exists && current.OperationGeneration > barrier.OperationGeneration {
		return errors.New("cannot replace recovery barrier with retired generation")
	}
	if exists && current.OperationGeneration == barrier.OperationGeneration && current.DurableCursor > barrier.DurableCursor {
		return errors.New("cannot move recovery barrier cursor backward")
	}
	state.barriers[barrier.Name] = barrier
	return nil
}

func (state *BarrierState) List() []RecoveryBarrier {
	state.mu.RLock()
	defer state.mu.RUnlock()
	result := make([]RecoveryBarrier, 0, len(state.barriers))
	for _, barrier := range state.barriers {
		result = append(result, barrier)
	}
	return result
}

func (barrier RecoveryBarrier) Validate() error {
	if barrier.OperationID == uuid.Nil || barrier.OperationGeneration == 0 {
		return errors.New("barrier operation identity is required")
	}
	if barrier.Name == "" || barrier.DurableCursor == 0 {
		return errors.New("barrier name and durable cursor are required")
	}
	if barrier.UpdatedAt.IsZero() {
		return fmt.Errorf("barrier %s has no timestamp", barrier.Name)
	}
	return nil
}
