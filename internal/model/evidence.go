package model

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

type EvidenceKind string

const (
	EvidencePressure EvidenceKind = "pressure"
	EvidenceLevel    EvidenceKind = "level"
	EvidenceForecast EvidenceKind = "forecast"
	EvidenceGate     EvidenceKind = "gate"
	EvidenceValve    EvidenceKind = "valve"
)

type Evidence struct {
	ID         uuid.UUID    `json:"id"`
	SourceID   string       `json:"source_id"`
	Kind       EvidenceKind `json:"kind"`
	RevisionID uuid.UUID    `json:"revision_id"`
	Value      float64      `json:"value"`
	ObservedAt time.Time    `json:"observed_at"`
	ReceivedAt time.Time    `json:"received_at"`
	Derived    bool         `json:"derived"`
}

type Forecast struct {
	RevisionID uuid.UUID `json:"revision_id"`
	PeakMeters float64   `json:"peak_meters"`
	PeakAt     time.Time `json:"peak_at"`
	IssuedAt   time.Time `json:"issued_at"`
}

type Permit struct {
	ID                  uuid.UUID `json:"id"`
	OperationID         uuid.UUID `json:"operation_id"`
	OperationGeneration uint64    `json:"operation_generation"`
	ForecastRevisionID  uuid.UUID `json:"forecast_revision_id"`
	EvidenceRevisionID  uuid.UUID `json:"evidence_revision_id"`
	Allowed             bool      `json:"allowed"`
	Reason              string    `json:"reason"`
	IssuedAt            time.Time `json:"issued_at"`
}

func NewEvidence(source string, kind EvidenceKind, revision uuid.UUID, value float64, observed, received time.Time, derived bool) Evidence {
	return Evidence{
		ID:         uuid.New(),
		SourceID:   source,
		Kind:       kind,
		RevisionID: revision,
		Value:      value,
		ObservedAt: observed.UTC(),
		ReceivedAt: received.UTC(),
		Derived:    derived,
	}
}

func (e Evidence) Validate() error {
	if e.ID == uuid.Nil || e.RevisionID == uuid.Nil {
		return errors.New("evidence identities are required")
	}
	if e.SourceID == "" {
		return errors.New("evidence source is required")
	}
	if e.ObservedAt.IsZero() || e.ReceivedAt.IsZero() {
		return errors.New("evidence timestamps are required")
	}
	switch e.Kind {
	case EvidencePressure, EvidenceLevel, EvidenceForecast, EvidenceGate, EvidenceValve:
	default:
		return fmt.Errorf("unknown evidence kind %q", e.Kind)
	}
	return nil
}

func (forecast Forecast) Validate() error {
	if forecast.RevisionID == uuid.Nil {
		return errors.New("forecast revision is required")
	}
	if forecast.PeakMeters < -5 || forecast.PeakMeters > 20 {
		return errors.New("forecast peak is outside supported range")
	}
	if forecast.PeakAt.IsZero() || forecast.IssuedAt.IsZero() {
		return errors.New("forecast timestamps are required")
	}
	return nil
}

func SortedEvidence(items []Evidence) []Evidence {
	result := append([]Evidence(nil), items...)
	sort.Slice(result, func(i, j int) bool {
		if result[i].ObservedAt.Equal(result[j].ObservedAt) {
			return result[i].ID.String() < result[j].ID.String()
		}
		return result[i].ObservedAt.Before(result[j].ObservedAt)
	})
	return result
}

func PermitMatches(permit Permit, operation Operation, forecast Forecast, evidenceRevision uuid.UUID) bool {
	return permit.OperationID == operation.ID &&
		permit.OperationGeneration == operation.Generation &&
		permit.ForecastRevisionID == forecast.RevisionID &&
		permit.EvidenceRevisionID == evidenceRevision
}
