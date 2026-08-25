package telemetry

import (
	"errors"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/wyw14/cry-126/internal/journal"
	"github.com/wyw14/cry-126/internal/model"
)

type Sample struct {
	SourceID   string    `json:"source_id"`
	Pressure   float64   `json:"pressure"`
	ObservedAt time.Time `json:"observed_at"`
	ReceivedAt time.Time `json:"received_at"`
}

type Receiver struct {
	state   *State
	journal *journal.EquipmentJournal
}

func NewReceiver(state *State, equipmentJournal *journal.EquipmentJournal) (*Receiver, error) {
	if state == nil || equipmentJournal == nil {
		return nil, errors.New("telemetry receiver dependencies are required")
	}
	return &Receiver{state: state, journal: equipmentJournal}, nil
}

func (receiver *Receiver) Receive(operation model.Operation, sample Sample) ([]model.Evidence, error) {
	if sample.SourceID == "" || sample.ObservedAt.IsZero() || sample.ReceivedAt.IsZero() {
		return nil, errors.New("telemetry sample is incomplete")
	}
	if math.IsNaN(sample.Pressure) || math.IsInf(sample.Pressure, 0) {
		return nil, errors.New("telemetry pressure is invalid")
	}
	revision := uuid.New()
	pressure := model.NewEvidence(sample.SourceID, model.EvidencePressure, revision, sample.Pressure, sample.ObservedAt, sample.ReceivedAt, false)
	level := model.NewEvidence(sample.SourceID, model.EvidenceLevel, revision, pressureToMeters(sample.Pressure), sample.ObservedAt, sample.ReceivedAt, true)
	for _, evidence := range []model.Evidence{pressure, level} {
		if err := receiver.state.Add(evidence); err != nil {
			return nil, err
		}
		if _, err := receiver.journal.SaveEvidence(operation.ID, operation.Generation, evidence, sample.ReceivedAt); err != nil {
			return nil, err
		}
	}
	return []model.Evidence{pressure, level}, nil
}

func (receiver *Receiver) Latest() []model.Evidence {
	return receiver.state.AllLatest()
}

func (receiver *Receiver) Revision() uuid.UUID {
	return receiver.state.Revision()
}

func pressureToMeters(pressure float64) float64 {
	return pressure / 9.80665
}
