package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type OperationPhase string

const (
	PhaseStandby    OperationPhase = "standby"
	PhasePreparing  OperationPhase = "preparing"
	PhaseDefending  OperationPhase = "defending"
	PhaseDraining   OperationPhase = "draining"
	PhaseRecovering OperationPhase = "recovering"
	PhaseClosed     OperationPhase = "closed"
)

type Operation struct {
	ID         uuid.UUID      `json:"id"`
	Generation uint64         `json:"generation"`
	Phase      OperationPhase `json:"phase"`
	StartedAt  time.Time      `json:"started_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	Reason     string         `json:"reason"`
}

func NewOperation(generation uint64, reason string, now time.Time) Operation {
	return Operation{
		ID:         uuid.New(),
		Generation: generation,
		Phase:      PhasePreparing,
		StartedAt:  now.UTC(),
		UpdatedAt:  now.UTC(),
		Reason:     reason,
	}
}

func (o Operation) Validate() error {
	if o.ID == uuid.Nil {
		return errors.New("operation id is required")
	}
	if o.Generation == 0 {
		return errors.New("operation generation is required")
	}
	if o.StartedAt.IsZero() || o.UpdatedAt.IsZero() {
		return errors.New("operation timestamps are required")
	}
	if o.UpdatedAt.Before(o.StartedAt) {
		return errors.New("operation update precedes start")
	}
	if !ValidPhase(o.Phase) {
		return fmt.Errorf("unknown operation phase %q", o.Phase)
	}
	return nil
}

func ValidPhase(phase OperationPhase) bool {
	switch phase {
	case PhaseStandby, PhasePreparing, PhaseDefending, PhaseDraining, PhaseRecovering, PhaseClosed:
		return true
	default:
		return false
	}
}

func CanTransition(from, to OperationPhase) bool {
	switch from {
	case PhaseStandby:
		return to == PhasePreparing
	case PhasePreparing:
		return to == PhaseDefending || to == PhaseClosed
	case PhaseDefending:
		return to == PhaseDraining || to == PhaseRecovering || to == PhaseClosed
	case PhaseDraining:
		return to == PhaseRecovering || to == PhaseClosed
	case PhaseRecovering:
		return to == PhaseStandby || to == PhaseClosed
	case PhaseClosed:
		return false
	default:
		return false
	}
}

func TransitionOperation(current Operation, next OperationPhase, now time.Time) (Operation, error) {
	if err := current.Validate(); err != nil {
		return Operation{}, err
	}
	if !CanTransition(current.Phase, next) {
		return Operation{}, fmt.Errorf("cannot transition operation from %s to %s", current.Phase, next)
	}
	updated := current
	updated.Phase = next
	updated.UpdatedAt = now.UTC()
	if err := updated.Validate(); err != nil {
		return Operation{}, err
	}
	return updated, nil
}

func OperationIdentity(operation Operation) string {
	return fmt.Sprintf("%s/%d", operation.ID.String(), operation.Generation)
}
