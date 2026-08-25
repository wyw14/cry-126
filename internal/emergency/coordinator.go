package emergency

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/wyw14/cry-126/internal/gate"
	"github.com/wyw14/cry-126/internal/interlock"
	"github.com/wyw14/cry-126/internal/model"
	"github.com/wyw14/cry-126/internal/operation"
)

type Coordinator struct {
	mu         sync.Mutex
	state      *State
	locks      *interlock.State
	gates      *gate.Coordinator
	operations *operation.Manager
	gateIDs    []string
	held       map[uint64]interlock.Lock
}

func NewCoordinator(state *State, locks *interlock.State, gates *gate.Coordinator, operations *operation.Manager, gateIDs []string) (*Coordinator, error) {
	if state == nil || locks == nil || gates == nil || operations == nil || len(gateIDs) == 0 {
		return nil, errors.New("emergency coordinator dependencies are required")
	}
	return &Coordinator{state: state, locks: locks, gates: gates, operations: operations, gateIDs: append([]string(nil), gateIDs...), held: make(map[uint64]interlock.Lock)}, nil
}

func (coordinator *Coordinator) Close(ctx context.Context, operationValue model.Operation, now time.Time) (interlock.BarrierDecision, error) {
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	if coordinator.operations.Current().Generation != operationValue.Generation {
		return interlock.BarrierDecision{}, errors.New("emergency closure operation is retired")
	}
	closure, err := coordinator.state.Begin(operationValue.ID, operationValue.Generation, now)
	if err != nil {
		return interlock.BarrierDecision{}, err
	}
	lock, err := interlock.AcquireStormLock(coordinator.locks, operationValue, now)
	if err != nil {
		coordinator.state.Update(operationValue.Generation, ClosureFailed, err.Error(), now)
		return interlock.BarrierDecision{}, err
	}
	coordinator.held[operationValue.Generation] = lock
	coordinator.operations.RetireBefore(operationValue.Generation)
	StopRetiredGateWorkers(coordinator.gates, operationValue.Generation)
	coordinator.state.Update(closure.OperationGeneration, ClosureRunning, "", now)
	resultStream := make(chan gate.FleetResult, 1)
	if err := coordinator.operations.Launch(operationValue, func(operationContext context.Context) {
		closureContext, cancel := context.WithCancel(operationContext)
		stop := context.AfterFunc(ctx, cancel)
		defer stop()
		defer cancel()
		resultStream <- gate.CloseFleet(closureContext, coordinator.gates, operationValue, coordinator.gateIDs, now)
	}); err != nil {
		coordinator.state.Update(operationValue.Generation, ClosureFailed, err.Error(), time.Now())
		return interlock.BarrierDecision{}, err
	}
	result := <-resultStream
	decision := interlock.DecideBarrier(result, time.Now().UTC())
	if err := interlock.ValidateBarrierDecision(decision); err != nil {
		coordinator.state.Update(operationValue.Generation, ClosureFailed, strings.Join(decision.Failures, "; "), time.Now())
		return decision, err
	}
	coordinator.state.Update(operationValue.Generation, ClosureSealed, "", time.Now())
	return decision, nil
}

func StopRetiredGateWorkers(gates *gate.Coordinator, generation uint64) int {
	if gates == nil {
		return 0
	}
	return gates.CancelRetired(generation)
}

func (coordinator *Coordinator) Release(generation uint64) bool {
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	lock, exists := coordinator.held[generation]
	if !exists {
		return false
	}
	current, exists := coordinator.locks.Held(lock.Name)
	if !exists || current.ID != lock.ID || current.OperationID != lock.OperationID || current.OperationGeneration != lock.OperationGeneration || current.LockGeneration != lock.LockGeneration {
		return false
	}
	if !interlock.ReleaseStormLock(coordinator.locks, lock) {
		return false
	}
	delete(coordinator.held, generation)
	coordinator.state.Update(generation, ClosureRetired, "", time.Now())
	return true
}

func (coordinator *Coordinator) Closures() []Closure {
	return coordinator.state.Snapshot()
}
