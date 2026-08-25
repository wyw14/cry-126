package emergency

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/wyw14/cry-126/internal/interlock"
	"github.com/wyw14/cry-126/internal/journal"
	"github.com/wyw14/cry-126/internal/model"
)

type DurableClosure struct {
	Closure  Closure                   `json:"closure"`
	Decision interlock.BarrierDecision `json:"decision"`
	Cursor   uint64                    `json:"cursor"`
}

func PersistDecision(coordinator *journal.Coordinator, closure Closure, decision interlock.BarrierDecision, now time.Time) (DurableClosure, error) {
	if coordinator == nil {
		return DurableClosure{}, errors.New("journal coordinator is required")
	}
	if closure.OperationID == uuid.Nil || closure.OperationGeneration == 0 {
		return DurableClosure{}, errors.New("closure identity is incomplete")
	}
	incident := model.Incident{
		ID:                  uuid.New(),
		OperationID:         closure.OperationID,
		OperationGeneration: closure.OperationGeneration,
		Severity:            model.SeverityInfo,
		Code:                "barrier-sealed",
		Message:             "all required gates are durably sealed",
		RaisedAt:            now.UTC(),
	}
	if !decision.Sealed {
		incident.Severity = model.SeverityCritical
		incident.Code = "barrier-closure-failed"
		incident.Message = "barrier closure retained unresolved equipment failure"
	}
	event, err := journal.NewEvent(closure.OperationID, closure.OperationGeneration, "incident.raised", closure.ID.String(), incident, now)
	if err != nil {
		return DurableClosure{}, err
	}
	result, err := coordinator.Commit(event)
	if err != nil {
		return DurableClosure{}, err
	}
	return DurableClosure{Closure: closure, Decision: decision, Cursor: result.Cursor}, nil
}
