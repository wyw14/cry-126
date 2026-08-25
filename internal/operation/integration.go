package operation

import (
	"context"
	"errors"
	"time"

	"github.com/wyw14/cry-126/internal/journal"
	"github.com/wyw14/cry-126/internal/model"
)

type Manager struct {
	state     *State
	lifecycle *Lifecycle
	journal   *journal.Coordinator
}

func NewManager(state *State, lifecycle *Lifecycle, coordinator *journal.Coordinator) (*Manager, error) {
	if state == nil || lifecycle == nil || coordinator == nil {
		return nil, errors.New("operation manager dependencies are required")
	}
	return &Manager{state: state, lifecycle: lifecycle, journal: coordinator}, nil
}

func (manager *Manager) Start(ctx context.Context, reason string, now time.Time) (model.Operation, error) {
	operation, err := manager.state.Start(reason, now)
	if err != nil {
		return model.Operation{}, err
	}
	if err := manager.lifecycle.Activate(ctx, operation); err != nil {
		return model.Operation{}, err
	}
	if err := manager.persist(operation, now); err != nil {
		manager.lifecycle.Stop(operation.Generation)
		return model.Operation{}, err
	}
	return operation, nil
}

func (manager *Manager) Transition(operation model.Operation, phase model.OperationPhase, now time.Time) (model.Operation, error) {
	if !manager.state.IsCurrent(operation) {
		return model.Operation{}, errors.New("cannot transition retired operation")
	}
	updated, err := manager.state.Transition(operation.Generation, phase, now)
	if err != nil {
		return model.Operation{}, err
	}
	if err := manager.persist(updated, now); err != nil {
		return model.Operation{}, err
	}
	if phase == model.PhaseClosed {
		manager.lifecycle.Stop(updated.Generation)
	}
	return updated, nil
}

func (manager *Manager) Launch(operation model.Operation, work func(context.Context)) error {
	if !manager.state.IsCurrent(operation) {
		return errors.New("cannot launch worker for retired operation")
	}
	return manager.lifecycle.Launch(operation, work)
}

func (manager *Manager) RetireBefore(generation uint64) int {
	return manager.lifecycle.StopBefore(generation)
}

func (manager *Manager) Current() model.Operation {
	return manager.state.Current()
}

func (manager *Manager) persist(operation model.Operation, now time.Time) error {
	event, err := journal.NewEvent(operation.ID, operation.Generation, "operation.updated", operation.ID.String(), operation, now)
	if err != nil {
		return err
	}
	_, err = manager.journal.Commit(event)
	return err
}
