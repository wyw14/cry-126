package basin

import (
	"errors"
	"time"

	"github.com/wyw14/cry-126/internal/culvert"
	"github.com/wyw14/cry-126/internal/model"
	"github.com/wyw14/cry-126/internal/pump"
)

type DrainResult struct {
	Plan  Plan               `json:"plan"`
	Grant culvert.StartGrant `json:"grant"`
	Pump  model.Pump         `json:"pump"`
}

type Coordinator struct {
	state    *State
	culverts *culvert.Coordinator
	pumps    *pump.Coordinator
}

func NewCoordinator(state *State, culverts *culvert.Coordinator, pumps *pump.Coordinator) (*Coordinator, error) {
	if state == nil || culverts == nil || pumps == nil {
		return nil, errors.New("basin coordinator dependencies are required")
	}
	return &Coordinator{state: state, culverts: culverts, pumps: pumps}, nil
}

func (coordinator *Coordinator) StartDrain(basinID string, operation model.Operation, now time.Time) (DrainResult, error) {
	item, exists := coordinator.state.Get(basinID)
	if !exists {
		return DrainResult{}, errors.New("basin is not registered")
	}
	plan, err := BuildPlan(item, operation, model.DirectionOutward, 15*time.Minute, now)
	if err != nil {
		return DrainResult{}, err
	}
	grant, err := culvert.GrantRequest(coordinator.culverts, plan.Request)
	if err != nil {
		return DrainResult{}, err
	}
	started, err := coordinator.pumps.Start(plan.PumpID, plan.Request, grant, now)
	if err != nil {
		coordinator.culverts.Release(grant)
		return DrainResult{}, err
	}
	return DrainResult{Plan: plan, Grant: grant, Pump: started}, nil
}

func (coordinator *Coordinator) UpdateLevel(basinID string, meters float64, now time.Time) (Basin, error) {
	return coordinator.state.UpdateLevel(basinID, meters, now)
}

func (coordinator *Coordinator) Basins() []Basin {
	return coordinator.state.Snapshot()
}

func (coordinator *Coordinator) Plans(operation model.Operation, now time.Time) []Plan {
	return PlansFor(coordinator.state.Snapshot(), operation, 15*time.Minute, now)
}
