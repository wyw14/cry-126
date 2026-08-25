package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type GatePosition string

const (
	GateOpen    GatePosition = "open"
	GateClosed  GatePosition = "closed"
	GateMoving  GatePosition = "moving"
	GatePending GatePosition = "pending"
	GateFailed  GatePosition = "failed"
)

type Gate struct {
	ID                  string       `json:"id"`
	OperationID         uuid.UUID    `json:"operation_id"`
	OperationGeneration uint64       `json:"operation_generation"`
	CommandEpoch        uint64       `json:"command_epoch"`
	Position            GatePosition `json:"position"`
	OpeningPercent      int          `json:"opening_percent"`
	LastFeedbackAt      time.Time    `json:"last_feedback_at"`
	Failure             string       `json:"failure,omitempty"`
}

type PumpMode string

const (
	PumpStopped  PumpMode = "stopped"
	PumpStarting PumpMode = "starting"
	PumpDraining PumpMode = "draining"
	PumpStopping PumpMode = "stopping"
	PumpBlocked  PumpMode = "blocked"
)

type Pump struct {
	ID                  string    `json:"id"`
	Basin               string    `json:"basin"`
	OperationID         uuid.UUID `json:"operation_id"`
	OperationGeneration uint64    `json:"operation_generation"`
	CommandEpoch        uint64    `json:"command_epoch"`
	Mode                PumpMode  `json:"mode"`
	CheckValveProven    bool      `json:"check_valve_proven"`
	UpdatedAt           time.Time `json:"updated_at"`
	Failure             string    `json:"failure,omitempty"`
}

func (gate Gate) Validate() error {
	if gate.ID == "" {
		return errors.New("gate id is required")
	}
	if gate.OperationID == uuid.Nil || gate.OperationGeneration == 0 {
		return errors.New("gate operation identity is required")
	}
	if gate.CommandEpoch == 0 {
		return errors.New("gate command epoch is required")
	}
	if gate.OpeningPercent < 0 || gate.OpeningPercent > 100 {
		return fmt.Errorf("gate opening outside range: %d", gate.OpeningPercent)
	}
	switch gate.Position {
	case GateOpen, GateClosed, GateMoving, GatePending, GateFailed:
	default:
		return fmt.Errorf("unknown gate position %q", gate.Position)
	}
	return nil
}

func (pump Pump) Validate() error {
	if pump.ID == "" || pump.Basin == "" {
		return errors.New("pump identity is required")
	}
	if pump.OperationID == uuid.Nil || pump.OperationGeneration == 0 {
		return errors.New("pump operation identity is required")
	}
	if pump.CommandEpoch == 0 {
		return errors.New("pump command epoch is required")
	}
	switch pump.Mode {
	case PumpStopped, PumpStarting, PumpDraining, PumpStopping, PumpBlocked:
	default:
		return fmt.Errorf("unknown pump mode %q", pump.Mode)
	}
	if pump.Mode == PumpDraining && !pump.CheckValveProven {
		return errors.New("draining pump lacks check valve proof")
	}
	return nil
}

func GateCanReverse(gate Gate, acknowledgedEpoch uint64) bool {
	return gate.Position == GateClosed && gate.CommandEpoch == acknowledgedEpoch
}

func PumpCanRecover(pump Pump, generation uint64) bool {
	return pump.OperationGeneration == generation && pump.CheckValveProven && pump.Mode == PumpDraining
}
