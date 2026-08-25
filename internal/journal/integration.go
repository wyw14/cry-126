package journal

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/wyw14/cry-126/internal/model"
)

type EquipmentJournal struct {
	coordinator *Coordinator
}

func NewEquipmentJournal(coordinator *Coordinator) (*EquipmentJournal, error) {
	if coordinator == nil {
		return nil, fmt.Errorf("journal coordinator is required")
	}
	return &EquipmentJournal{coordinator: coordinator}, nil
}

func (journal *EquipmentJournal) SaveGate(gate model.Gate, now time.Time) (uint64, error) {
	event, err := NewEvent(gate.OperationID, gate.OperationGeneration, "gate.updated", gate.ID, gate, now)
	if err != nil {
		return 0, err
	}
	result, err := journal.coordinator.Commit(event)
	return result.Cursor, err
}

func (journal *EquipmentJournal) SavePump(pump model.Pump, now time.Time) (uint64, error) {
	event, err := NewEvent(pump.OperationID, pump.OperationGeneration, "pump.updated", pump.ID, pump, now)
	if err != nil {
		return 0, err
	}
	result, err := journal.coordinator.Commit(event)
	return result.Cursor, err
}

func (journal *EquipmentJournal) SaveLease(lease model.CulvertLease, now time.Time) (uint64, error) {
	event, err := NewEvent(lease.OperationID, lease.OperationGeneration, "lease.updated", lease.CulvertID, lease, now)
	if err != nil {
		return 0, err
	}
	result, err := journal.coordinator.Commit(event)
	return result.Cursor, err
}

func (journal *EquipmentJournal) SaveEvidence(operationID uuid.UUID, generation uint64, evidence model.Evidence, now time.Time) (uint64, error) {
	event, err := NewEvent(operationID, generation, "evidence.received", evidence.SourceID, evidence, now)
	if err != nil {
		return 0, err
	}
	result, err := journal.coordinator.Commit(event)
	return result.Cursor, err
}
