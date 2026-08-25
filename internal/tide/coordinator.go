package tide

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/wyw14/cry-126/internal/model"
)

type Coordinator struct {
	state    *State
	required int
	maxAge   time.Duration
}

func NewCoordinator(state *State, required int, maxAge time.Duration) (*Coordinator, error) {
	if state == nil || required < 1 || maxAge <= 0 {
		return nil, errors.New("tide coordinator settings are invalid")
	}
	return &Coordinator{state: state, required: required, maxAge: maxAge}, nil
}

func (coordinator *Coordinator) Ingest(items []model.Evidence, now time.Time) (Assessment, error) {
	if len(items) == 0 {
		return Assessment{}, errors.New("tide evidence is required")
	}
	physical := make(map[string]model.Evidence)
	coordinator.state.ClearBefore(now.Add(-coordinator.maxAge))
	for _, item := range items {
		if err := item.Validate(); err != nil {
			return Assessment{}, err
		}
		candidate := item
		if item.Kind == model.EvidencePressure {
			candidate.Kind = model.EvidenceLevel
			candidate.Value = item.Value / 9.80665
		}
		current, exists := physical[item.SourceID]
		if !exists || current.Derived && !candidate.Derived || candidate.ObservedAt.After(current.ObservedAt) {
			physical[item.SourceID] = candidate
		}
	}
	for _, item := range physical {
		coordinator.state.Put(item)
	}
	return coordinator.Assess(now)
}

func (coordinator *Coordinator) Assess(now time.Time) (Assessment, error) {
	return Assess(coordinator.state.Revision(), coordinator.state.Votes(), coordinator.required, coordinator.maxAge, now)
}

func (coordinator *Coordinator) EvidenceRevision() uuid.UUID {
	return coordinator.state.Revision()
}

func (coordinator *Coordinator) Votes() []Vote {
	return coordinator.state.Votes()
}
