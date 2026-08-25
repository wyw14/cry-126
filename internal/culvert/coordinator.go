package culvert

import (
	"errors"
	"time"

	"github.com/wyw14/cry-126/internal/journal"
	"github.com/wyw14/cry-126/internal/model"
)

type StartGrant struct {
	Lease       model.CulvertLease `json:"lease"`
	PersistedAt uint64             `json:"persisted_at"`
}

type Coordinator struct {
	table   *LeaseTable
	journal *journal.EquipmentJournal
}

func NewCoordinator(table *LeaseTable, equipmentJournal *journal.EquipmentJournal) (*Coordinator, error) {
	if table == nil || equipmentJournal == nil {
		return nil, errors.New("culvert coordinator dependencies are required")
	}
	return &Coordinator{table: table, journal: equipmentJournal}, nil
}

func (coordinator *Coordinator) AcquireForDrain(culvertID, basin string, direction model.LeaseDirection, operation model.Operation, duration time.Duration, now time.Time) (StartGrant, error) {
	lease, err := coordinator.table.Acquire(culvertID, basin, direction, operation, duration)
	if err != nil {
		return StartGrant{}, err
	}
	cursor, err := coordinator.journal.SaveLease(lease, now)
	if err != nil {
		coordinator.table.Release(lease.CulvertID, lease.ID, lease.FencingToken)
		return StartGrant{}, err
	}
	return StartGrant{Lease: lease, PersistedAt: cursor}, nil
}

func (coordinator *Coordinator) Release(grant StartGrant) bool {
	return coordinator.table.Release(grant.Lease.CulvertID, grant.Lease.ID, grant.Lease.FencingToken)
}

func (coordinator *Coordinator) Authorize(culvertID, owner string, fencing uint64, direction model.LeaseDirection) bool {
	lease, exists := coordinator.table.Current(culvertID)
	return exists && lease.Owner == owner && lease.FencingToken == fencing && lease.Direction == direction
}

func (coordinator *Coordinator) Snapshot() []model.CulvertLease {
	return coordinator.table.Snapshot()
}
