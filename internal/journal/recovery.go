package journal

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/wyw14/cry-126/internal/model"
)

type RecoveryReport struct {
	OperationGeneration uint64   `json:"operation_generation"`
	AppliedCursors      []uint64 `json:"applied_cursors"`
	SkippedCursors      []uint64 `json:"skipped_cursors"`
	FinalCursor         uint64   `json:"final_cursor"`
	BarrierSealed       bool     `json:"barrier_sealed"`
}

func Recover(snapshot model.SystemSnapshot, events []Event) (model.SystemSnapshot, RecoveryReport, error) {
	state, err := Replay(snapshot, events)
	if err != nil {
		return model.SystemSnapshot{}, RecoveryReport{}, err
	}
	report := RecoveryReport{
		OperationGeneration: state.Snapshot.Operation.Generation,
		FinalCursor:         state.Snapshot.Cursor,
		BarrierSealed:       barrierSealed(state.Snapshot.Gates),
	}
	for _, event := range state.Applied {
		report.AppliedCursors = append(report.AppliedCursors, event.Cursor)
	}
	for _, event := range state.Skipped {
		report.SkippedCursors = append(report.SkippedCursors, event.Cursor)
	}
	return state.Snapshot, report, nil
}

func NewCompletedBarrier(operationID uuid.UUID, generation uint64, name string, cursor uint64, now time.Time) (RecoveryBarrier, error) {
	barrier := RecoveryBarrier{
		OperationID:         operationID,
		OperationGeneration: generation,
		Name:                name,
		DurableCursor:       cursor,
		Completed:           true,
		UpdatedAt:           now.UTC(),
	}
	if err := barrier.Validate(); err != nil {
		return RecoveryBarrier{}, err
	}
	return barrier, nil
}

func RequireDurableCursor(published, durable uint64) error {
	if published > durable {
		return errors.New("published cursor exceeds durable journal cursor")
	}
	return nil
}

func barrierSealed(gates []model.Gate) bool {
	if len(gates) == 0 {
		return false
	}
	for _, gate := range gates {
		if gate.Position != model.GateClosed {
			return false
		}
	}
	return true
}
