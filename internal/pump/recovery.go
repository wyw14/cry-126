package pump

import (
	"errors"
	"time"

	"github.com/wyw14/cry-126/internal/culvert"
	"github.com/wyw14/cry-126/internal/model"
)

type Recovery struct {
	state    *State
	culverts *culvert.Recovery
	loaded   bool
}

func NewRecovery(state *State, culverts *culvert.Recovery) (*Recovery, error) {
	if state == nil || culverts == nil {
		return nil, errors.New("pump recovery dependencies are required")
	}
	return &Recovery{state: state, culverts: culverts}, nil
}

func (recovery *Recovery) Load(snapshot model.SystemSnapshot, now time.Time) error {
	if err := recovery.culverts.RequireLoaded(); err != nil {
		return err
	}
	valid := make([]model.Pump, 0, len(snapshot.Pumps))
	leases, err := recovery.culverts.Leases()
	if err != nil {
		return err
	}
	for _, pump := range snapshot.Pumps {
		if pump.Mode != model.PumpDraining {
			valid = append(valid, pump)
			continue
		}
		if !model.PumpCanRecover(pump, snapshot.Operation.Generation) || !matchingLease(pump, leases, now) {
			pump.Mode = model.PumpBlocked
			pump.Failure = "recovery fencing is unavailable"
			pump.UpdatedAt = now.UTC()
		}
		valid = append(valid, pump)
	}
	if err := recovery.state.Restore(valid); err != nil {
		return err
	}
	recovery.loaded = true
	return nil
}

func (recovery *Recovery) Loaded() bool {
	return recovery.loaded
}

func (recovery *Recovery) Pumps() ([]model.Pump, error) {
	if !recovery.loaded {
		return nil, errors.New("pump recovery has not completed")
	}
	return recovery.state.Snapshot(), nil
}

func matchingLease(pump model.Pump, leases []model.CulvertLease, now time.Time) bool {
	for _, lease := range leases {
		if lease.Owner == pump.Basin && lease.OperationID == pump.OperationID && lease.OperationGeneration == pump.OperationGeneration && lease.ExpiresAt.After(now) {
			return true
		}
	}
	return false
}
