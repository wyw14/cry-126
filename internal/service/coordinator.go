package service

import (
	"context"
	"errors"
	"time"

	"github.com/wyw14/cry-126/internal/journal"
	"github.com/wyw14/cry-126/internal/operation"
)

type RecoverySequence struct {
	SnapshotLoaded  bool   `json:"snapshot_loaded"`
	JournalReplayed bool   `json:"journal_replayed"`
	OperationLoaded bool   `json:"operation_loaded"`
	FencingLoaded   bool   `json:"fencing_loaded"`
	PumpsLoaded     bool   `json:"pumps_loaded"`
	GatesLoaded     bool   `json:"gates_loaded"`
	Cursor          uint64 `json:"cursor"`
}

func (runtime *Runtime) Recover(ctx context.Context, now time.Time) (RecoverySequence, error) {
	snapshot, err := runtime.snapshots.Load()
	if err != nil {
		return RecoverySequence{}, err
	}
	sequence := RecoverySequence{SnapshotLoaded: true, Cursor: snapshot.Cursor}
	events, err := runtime.journal.ReplayFrom(snapshot.Cursor)
	if err != nil {
		return RecoverySequence{}, err
	}
	_, report, err := journal.Recover(snapshot, events)
	if err != nil {
		return RecoverySequence{}, err
	}
	sequence.JournalReplayed = true
	sequence.Cursor = report.FinalCursor
	operationRecovery, err := operation.NewRecovery(runtime.operationState, runtime.lifecycle)
	if err != nil {
		return RecoverySequence{}, err
	}
	if _, err := operationRecovery.Restore(ctx, snapshot, events); err != nil {
		return RecoverySequence{}, err
	}
	sequence.OperationLoaded = true
	if err := runtime.restoreEquipment(snapshot, events, now); err != nil {
		return RecoverySequence{}, err
	}
	sequence.FencingLoaded = true
	sequence.PumpsLoaded = true
	sequence.GatesLoaded = true
	if !runtime.culvertRecovery.Loaded() || !runtime.pumpRecovery.Loaded() {
		return RecoverySequence{}, errors.New("recovery dependencies did not complete")
	}
	return sequence, nil
}
