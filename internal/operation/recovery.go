package operation

import (
	"context"
	"errors"

	"github.com/wyw14/cry-126/internal/journal"
	"github.com/wyw14/cry-126/internal/model"
)

type Recovery struct {
	state     *State
	lifecycle *Lifecycle
}

func NewRecovery(state *State, lifecycle *Lifecycle) (*Recovery, error) {
	if state == nil || lifecycle == nil {
		return nil, errors.New("operation recovery dependencies are required")
	}
	return &Recovery{state: state, lifecycle: lifecycle}, nil
}

func (recovery *Recovery) Restore(ctx context.Context, snapshot model.SystemSnapshot, events []journal.Event) (journal.RecoveryReport, error) {
	restored, report, err := journal.Recover(snapshot, events)
	if err != nil {
		return journal.RecoveryReport{}, err
	}
	completed := make([]model.Operation, 0)
	if restored.Operation.Phase == model.PhaseClosed {
		completed = append(completed, restored.Operation)
	}
	if err := recovery.state.Restore(restored.Operation, completed); err != nil {
		return journal.RecoveryReport{}, err
	}
	if restored.Operation.Generation != 0 && restored.Operation.Phase != model.PhaseClosed {
		if err := recovery.lifecycle.Activate(ctx, restored.Operation); err != nil {
			return journal.RecoveryReport{}, err
		}
	}
	return report, nil
}
