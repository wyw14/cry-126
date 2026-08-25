package interlock

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/wyw14/cry-126/internal/model"
	"github.com/wyw14/cry-126/internal/tide"
)

type PermitPolicy struct {
	DefenseThreshold float64
	MaximumSpread    float64
	RequireQuorum    bool
}

func EvaluatePermit(operation model.Operation, forecast model.Forecast, evidenceRevision uuid.UUID, assessment tide.Assessment, policy PermitPolicy, now time.Time) (model.Permit, error) {
	if err := operation.Validate(); err != nil {
		return model.Permit{}, err
	}
	if forecast.RevisionID == uuid.Nil || evidenceRevision == uuid.Nil || assessment.RevisionID != evidenceRevision {
		return model.Permit{}, errors.New("permit evidence revisions are incomplete")
	}
	allowed := true
	reason := "evidence permits controlled movement"
	if policy.RequireQuorum && !assessment.Quorum {
		allowed = false
		reason = "independent tide quorum is unavailable"
	}
	if assessment.Quorum && !tide.Stable(assessment, policy.MaximumSpread) {
		allowed = false
		reason = "tide sources disagree beyond tolerance"
	}
	if tide.AboveThreshold(assessment, policy.DefenseThreshold) || forecast.PeakMeters >= policy.DefenseThreshold {
		allowed = false
		reason = fmt.Sprintf("forecast crest %.2f requires barrier defense", forecast.PeakMeters)
	}
	return model.Permit{
		ID:                  uuid.New(),
		OperationID:         operation.ID,
		OperationGeneration: operation.Generation,
		ForecastRevisionID:  forecast.RevisionID,
		EvidenceRevisionID:  evidenceRevision,
		Allowed:             allowed,
		Reason:              reason,
		IssuedAt:            now.UTC(),
	}, nil
}

func PermitCurrent(permit model.Permit, operation model.Operation, forecast model.Forecast, evidenceRevision uuid.UUID) bool {
	return permit.ID != uuid.Nil && model.PermitMatches(permit, operation, forecast, evidenceRevision)
}

func RequirePermit(permit model.Permit, operation model.Operation, forecast model.Forecast, evidenceRevision uuid.UUID) error {
	if !PermitCurrent(permit, operation, forecast, evidenceRevision) {
		return errors.New("safety permit is stale")
	}
	if !permit.Allowed {
		return errors.New(permit.Reason)
	}
	return nil
}
