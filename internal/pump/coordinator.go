package pump

import (
	"errors"
	"sync"
	"time"

	"github.com/wyw14/cry-126/internal/culvert"
	"github.com/wyw14/cry-126/internal/journal"
	"github.com/wyw14/cry-126/internal/model"
)

type Coordinator struct {
	mu       sync.Mutex
	state    *State
	journal  *journal.EquipmentJournal
	culverts *culvert.Coordinator
	epochs   map[string]uint64
}

func NewCoordinator(state *State, equipmentJournal *journal.EquipmentJournal, culverts *culvert.Coordinator) (*Coordinator, error) {
	if state == nil || equipmentJournal == nil || culverts == nil {
		return nil, errors.New("pump coordinator dependencies are required")
	}
	return &Coordinator{state: state, journal: equipmentJournal, culverts: culverts, epochs: make(map[string]uint64)}, nil
}

func (coordinator *Coordinator) Start(pumpID string, request culvert.DrainageRequest, grant culvert.StartGrant, now time.Time) (model.Pump, error) {
	if !culvert.SameOwnership(grant, request) {
		return model.Pump{}, errors.New("culvert grant does not match drainage request")
	}
	if !coordinator.culverts.Authorize(grant.Lease.CulvertID, request.Basin, grant.Lease.FencingToken, request.Direction) {
		return model.Pump{}, errors.New("culvert grant is no longer authoritative")
	}
	coordinator.mu.Lock()
	coordinator.epochs[pumpID]++
	epoch := coordinator.epochs[pumpID]
	coordinator.mu.Unlock()
	_, err := coordinator.state.Begin(pumpID, request.Basin, request.Operation, epoch, now)
	if err != nil {
		return model.Pump{}, err
	}
	proof, err := coordinator.state.ProveValve(pumpID, request.Operation, epoch, now)
	if err != nil {
		return model.Pump{}, err
	}
	if _, err := coordinator.journal.SavePump(proof, now); err != nil {
		// The check valve proof is not durable, so draining must neither be
		// published nor recoverable: roll the pump back to stopped instead of
		// advancing it, otherwise a later snapshot would advertise a draining
		// pump whose valve evidence is absent from the journal.
		coordinator.state.Stop(pumpID, request.Operation.Generation, err.Error(), now)
		return model.Pump{}, err
	}
	draining, err := coordinator.state.PublishDraining(pumpID, request.Operation, epoch, now)
	if err != nil {
		return model.Pump{}, err
	}
	if _, err := coordinator.journal.SavePump(draining, now); err != nil {
		coordinator.state.Stop(pumpID, request.Operation.Generation, err.Error(), now)
		return model.Pump{}, err
	}
	return draining, nil
}

func (coordinator *Coordinator) Stop(pumpID string, generation uint64, reason string, now time.Time) (model.Pump, error) {
	stopped, err := coordinator.state.Stop(pumpID, generation, reason, now)
	if err != nil {
		return model.Pump{}, err
	}
	if _, err := coordinator.journal.SavePump(stopped, now); err != nil {
		return model.Pump{}, err
	}
	return stopped, nil
}

func (coordinator *Coordinator) State() []model.Pump {
	return coordinator.state.Snapshot()
}

func (coordinator *Coordinator) Restore(items []model.Pump) error {
	if err := coordinator.state.Restore(items); err != nil {
		return err
	}
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	for _, item := range items {
		if item.CommandEpoch > coordinator.epochs[item.ID] {
			coordinator.epochs[item.ID] = item.CommandEpoch
		}
	}
	return nil
}
