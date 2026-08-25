package interlock

import (
	"errors"
	"time"

	"github.com/wyw14/cry-126/internal/gate"
	"github.com/wyw14/cry-126/internal/model"
)

type BarrierDecision struct {
	Operation model.Operation `json:"operation"`
	Sealed    bool            `json:"sealed"`
	Status    string          `json:"status"`
	Failures  []string        `json:"failures"`
	DecidedAt time.Time       `json:"decided_at"`
}

func DecideBarrier(result gate.FleetResult, decidedAt time.Time) BarrierDecision {
	decision := BarrierDecision{
		Operation: result.Operation,
		Sealed:    result.Sealed,
		Failures:  append([]string(nil), result.Failures...),
		DecidedAt: decidedAt.UTC(),
		Status:    "failed",
	}
	if result.Sealed && len(result.Failures) == 0 {
		decision.Status = "sealed"
		return decision
	}
	for _, item := range result.Gates {
		if item.Position == model.GatePending || item.Position == model.GateMoving {
			decision.Status = "pending"
			break
		}
	}
	return decision
}

func ValidateBarrierDecision(decision BarrierDecision) error {
	if decision.Status == "sealed" && (!decision.Sealed || len(decision.Failures) != 0) {
		return errors.New("sealed barrier decision contains unresolved failures")
	}
	if decision.Sealed {
		return nil
	}
	return errors.New("barrier is not sealed")
}
