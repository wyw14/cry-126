package service

import (
	"context"
	"errors"
	"time"

	"github.com/wyw14/cry-126/internal/emergency"
	"github.com/wyw14/cry-126/internal/interlock"
	"github.com/wyw14/cry-126/internal/journal"
)

func (runtime *Runtime) EmergencyClose(ctx context.Context, now time.Time) (interlock.BarrierDecision, error) {
	operationValue := runtime.operations.Current()
	decision, closeErr := runtime.emergency.Close(ctx, operationValue, now)
	closure, exists := runtime.emergencyState.Get(operationValue.Generation)
	if !exists {
		return decision, errors.New("emergency closure state was not recorded")
	}
	_, persistErr := emergency.PersistDecision(runtime.journal, closure, decision, time.Now())
	runtime.mu.Lock()
	runtime.lastDecision = decision
	runtime.mu.Unlock()
	if closeErr != nil {
		return decision, closeErr
	}
	if persistErr != nil {
		return decision, persistErr
	}
	durable, err := journal.NewCompletedBarrier(operationValue.ID, operationValue.Generation, "emergency-closure", runtime.journal.Cursor(), time.Now())
	if err != nil {
		return decision, err
	}
	if err := journal.RequireDurableCursor(runtime.journal.Cursor(), durable.DurableCursor); err != nil {
		return decision, err
	}
	if err := runtime.barriers.Put(durable); err != nil {
		return decision, err
	}
	return decision, nil
}

func (runtime *Runtime) ReleaseClosure(generation uint64) bool {
	return runtime.emergency.Release(generation)
}

func (runtime *Runtime) RecoveryBarriers() []journal.RecoveryBarrier {
	return runtime.barriers.List()
}
