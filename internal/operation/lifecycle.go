package operation

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
	"github.com/wyw14/cry-126/internal/model"
)

type Lifecycle struct {
	mu      sync.Mutex
	workers map[uint64]*generationWorkers
}

type generationWorkers struct {
	operationID uuid.UUID
	context     context.Context
	cancel      context.CancelFunc
	wait        sync.WaitGroup
}

func NewLifecycle() *Lifecycle {
	return &Lifecycle{workers: make(map[uint64]*generationWorkers)}
}

func (lifecycle *Lifecycle) Activate(parent context.Context, operation model.Operation) error {
	if err := operation.Validate(); err != nil {
		return err
	}
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	if _, exists := lifecycle.workers[operation.Generation]; exists {
		return errors.New("operation lifecycle already active")
	}
	ctx, cancel := context.WithCancel(parent)
	lifecycle.workers[operation.Generation] = &generationWorkers{
		operationID: operation.ID,
		context:     ctx,
		cancel:      cancel,
	}
	return nil
}

func (lifecycle *Lifecycle) Launch(operation model.Operation, work func(context.Context)) error {
	lifecycle.mu.Lock()
	workers, exists := lifecycle.workers[operation.Generation]
	if !exists || workers.operationID != operation.ID {
		lifecycle.mu.Unlock()
		return errors.New("operation lifecycle is not active")
	}
	workers.wait.Add(1)
	ctx := workers.context
	lifecycle.mu.Unlock()
	go func() {
		defer workers.wait.Done()
		work(ctx)
	}()
	return nil
}

func (lifecycle *Lifecycle) Stop(generation uint64) bool {
	lifecycle.mu.Lock()
	workers, exists := lifecycle.workers[generation]
	if exists {
		delete(lifecycle.workers, generation)
		workers.cancel()
	}
	lifecycle.mu.Unlock()
	if exists {
		workers.wait.Wait()
	}
	return exists
}

func (lifecycle *Lifecycle) StopBefore(generation uint64) int {
	lifecycle.mu.Lock()
	retired := make([]*generationWorkers, 0)
	for candidate, workers := range lifecycle.workers {
		if candidate < generation {
			delete(lifecycle.workers, candidate)
			workers.cancel()
			retired = append(retired, workers)
		}
	}
	lifecycle.mu.Unlock()
	for _, workers := range retired {
		workers.wait.Wait()
	}
	return len(retired)
}

func (lifecycle *Lifecycle) ActiveGenerations() []uint64 {
	lifecycle.mu.Lock()
	defer lifecycle.mu.Unlock()
	result := make([]uint64, 0, len(lifecycle.workers))
	for generation := range lifecycle.workers {
		result = append(result, generation)
	}
	return result
}
