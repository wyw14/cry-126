package interlock

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wyw14/cry-126/internal/forecast"
	"github.com/wyw14/cry-126/internal/model"
	"github.com/wyw14/cry-126/internal/tide"
)

type Arbiter struct {
	mu       sync.Mutex
	state    *State
	forecast *forecast.Coordinator
	tide     *tide.Coordinator
	policy   PermitPolicy
}

func NewArbiter(state *State, forecastCoordinator *forecast.Coordinator, tideCoordinator *tide.Coordinator, policy PermitPolicy) (*Arbiter, error) {
	if state == nil || forecastCoordinator == nil || tideCoordinator == nil {
		return nil, errors.New("interlock arbiter dependencies are required")
	}
	return &Arbiter{state: state, forecast: forecastCoordinator, tide: tideCoordinator, policy: policy}, nil
}

func (arbiter *Arbiter) Recalculate(operation model.Operation, now time.Time) (model.Permit, error) {
	arbiter.mu.Lock()
	defer arbiter.mu.Unlock()
	window, exists := arbiter.forecast.Current()
	if !exists {
		return model.Permit{}, errors.New("forecast is unavailable")
	}
	assessment, err := arbiter.tide.Assess(now)
	if err != nil {
		return model.Permit{}, err
	}
	permit, err := EvaluatePermit(operation, window.Revision, arbiter.tide.EvidenceRevision(), assessment, arbiter.policy, now)
	if err != nil {
		return model.Permit{}, err
	}
	arbiter.state.SetPermit(permit)
	return permit, nil
}

func (arbiter *Arbiter) Current(operation model.Operation) (model.Permit, error) {
	window, exists := arbiter.forecast.Current()
	if !exists {
		return model.Permit{}, errors.New("forecast is unavailable")
	}
	permit := arbiter.state.Permit()
	if !PermitCurrent(permit, operation, window.Revision, arbiter.tide.EvidenceRevision()) {
		return model.Permit{}, errors.New("safety permit is stale")
	}
	return permit, nil
}

func (arbiter *Arbiter) InvalidateForecast(revision uuid.UUID) bool {
	arbiter.mu.Lock()
	defer arbiter.mu.Unlock()
	permit := arbiter.state.Permit()
	if permit.ForecastRevisionID == revision {
		return false
	}
	permit.Allowed = false
	permit.Reason = "forecast revision changed"
	permit.ForecastRevisionID = revision
	arbiter.state.SetPermit(permit)
	return true
}

func (arbiter *Arbiter) Locks() []Lock {
	return arbiter.state.Locks()
}
