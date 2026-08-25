package culvert

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wyw14/cry-126/internal/model"
)

type LeaseTable struct {
	mu      sync.Mutex
	leases  map[string]model.CulvertLease
	fencing map[string]uint64
	clock   func() time.Time
}

func NewLeaseTable(clock func() time.Time) *LeaseTable {
	if clock == nil {
		clock = time.Now
	}
	return &LeaseTable{
		leases:  make(map[string]model.CulvertLease),
		fencing: make(map[string]uint64),
		clock:   clock,
	}
}

func (table *LeaseTable) Acquire(culvertID, owner string, direction model.LeaseDirection, operation model.Operation, duration time.Duration) (model.CulvertLease, error) {
	if culvertID == "" || owner == "" {
		return model.CulvertLease{}, errors.New("culvert and owner are required")
	}
	if duration <= 0 {
		return model.CulvertLease{}, errors.New("lease duration must be positive")
	}
	if direction != model.DirectionInward && direction != model.DirectionOutward {
		return model.CulvertLease{}, errors.New("lease direction is invalid")
	}
	if err := operation.Validate(); err != nil {
		return model.CulvertLease{}, err
	}
	table.mu.Lock()
	defer table.mu.Unlock()
	now := table.clock().UTC()
	current, exists := table.leases[culvertID]
	if exists && current.ExpiresAt.After(now) {
		if current.Owner == owner && current.OperationID == operation.ID && current.OperationGeneration == operation.Generation && current.Direction == direction {
			return current, nil
		}
		return model.CulvertLease{}, fmt.Errorf("culvert %s is owned by %s", culvertID, current.Owner)
	}
	table.fencing[culvertID]++
	lease := model.CulvertLease{
		ID:                  uuid.New(),
		CulvertID:           culvertID,
		Owner:               owner,
		Direction:           direction,
		OperationID:         operation.ID,
		OperationGeneration: operation.Generation,
		FencingToken:        table.fencing[culvertID],
		AcquiredAt:          now,
		ExpiresAt:           now.Add(duration),
	}
	if err := lease.Validate(); err != nil {
		return model.CulvertLease{}, err
	}
	table.leases[culvertID] = lease
	return lease, nil
}

func (table *LeaseTable) Release(culvertID string, leaseID uuid.UUID, fencing uint64) bool {
	table.mu.Lock()
	defer table.mu.Unlock()
	current, exists := table.leases[culvertID]
	if !exists || current.ID != leaseID || current.FencingToken != fencing {
		return false
	}
	delete(table.leases, culvertID)
	return true
}

func (table *LeaseTable) Current(culvertID string) (model.CulvertLease, bool) {
	table.mu.Lock()
	defer table.mu.Unlock()
	current, exists := table.leases[culvertID]
	if exists && !current.ExpiresAt.After(table.clock().UTC()) {
		delete(table.leases, culvertID)
		return model.CulvertLease{}, false
	}
	return current, exists
}

func (table *LeaseTable) Snapshot() []model.CulvertLease {
	table.mu.Lock()
	defer table.mu.Unlock()
	now := table.clock().UTC()
	result := make([]model.CulvertLease, 0, len(table.leases))
	for id, lease := range table.leases {
		if lease.ExpiresAt.After(now) {
			result = append(result, lease)
		} else {
			delete(table.leases, id)
		}
	}
	return result
}

func (table *LeaseTable) Restore(leases []model.CulvertLease) error {
	table.mu.Lock()
	defer table.mu.Unlock()
	restored := make(map[string]model.CulvertLease, len(leases))
	for _, lease := range leases {
		if err := lease.Validate(); err != nil {
			return err
		}
		if current, exists := restored[lease.CulvertID]; exists && current.FencingToken >= lease.FencingToken {
			return errors.New("culvert snapshot has overlapping fencing tokens")
		}
		restored[lease.CulvertID] = lease
		if lease.FencingToken > table.fencing[lease.CulvertID] {
			table.fencing[lease.CulvertID] = lease.FencingToken
		}
	}
	table.leases = restored
	return nil
}
