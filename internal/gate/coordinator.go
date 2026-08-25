package gate

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/wyw14/cry-126/internal/journal"
	"github.com/wyw14/cry-126/internal/model"
)

type Coordinator struct {
	mu         sync.Mutex
	state      *State
	controller *Controller
	journal    *journal.EquipmentJournal
	epochs     map[string]uint64
}

func NewCoordinator(state *State, controller *Controller, equipmentJournal *journal.EquipmentJournal) (*Coordinator, error) {
	if state == nil || controller == nil || equipmentJournal == nil {
		return nil, errors.New("gate coordinator dependencies are required")
	}
	return &Coordinator{state: state, controller: controller, journal: equipmentJournal, epochs: make(map[string]uint64)}, nil
}

func (coordinator *Coordinator) Move(ctx context.Context, gateID string, operation model.Operation, target model.GatePosition, opening int, now time.Time) (model.Gate, error) {
	coordinator.mu.Lock()
	coordinator.epochs[gateID]++
	epoch := coordinator.epochs[gateID]
	coordinator.mu.Unlock()
	commanded, err := coordinator.state.Command(gateID, operation, epoch, target, opening, now)
	if err != nil {
		return model.Gate{}, err
	}
	if _, err := coordinator.journal.SaveGate(commanded, now); err != nil {
		return model.Gate{}, err
	}
	command := MotorCommand{GateID: gateID, Operation: operation, Epoch: epoch, Target: target, Opening: opening, IssuedAt: now.UTC()}
	result := <-coordinator.controller.Execute(ctx, command)
	acknowledged, err := coordinator.state.Acknowledge(gateID, operation, result.Epoch, result.Position, result.Opening, result.Failure, result.Finished)
	if err != nil {
		return model.Gate{}, err
	}
	if _, err := coordinator.journal.SaveGate(acknowledged, result.Finished); err != nil {
		return model.Gate{}, err
	}
	if acknowledged.Position == model.GateFailed {
		return acknowledged, errors.New(acknowledged.Failure)
	}
	return acknowledged, nil
}

func (coordinator *Coordinator) Reverse(ctx context.Context, gateID string, operation model.Operation, now time.Time) (model.Gate, error) {
	current, exists := coordinator.state.Get(gateID)
	if !exists {
		return model.Gate{}, errors.New("gate is not registered")
	}
	if !model.GateCanReverse(current, current.CommandEpoch) || current.OperationID != operation.ID || current.OperationGeneration != operation.Generation {
		return model.Gate{}, errors.New("current operation has no closed acknowledgement")
	}
	return coordinator.Move(ctx, gateID, operation, model.GateOpen, 100, now)
}

func (coordinator *Coordinator) CancelRetired(generation uint64) int {
	return coordinator.controller.CancelBefore(generation)
}

func (coordinator *Coordinator) Gates() []model.Gate {
	return coordinator.state.Snapshot()
}

func (coordinator *Coordinator) Restore(items []model.Gate) error {
	if err := coordinator.state.Restore(items); err != nil {
		return err
	}
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	for _, gate := range items {
		if gate.CommandEpoch > coordinator.epochs[gate.ID] {
			coordinator.epochs[gate.ID] = gate.CommandEpoch
		}
	}
	return nil
}

func (coordinator *Coordinator) Wait() {
	coordinator.controller.Wait()
}
