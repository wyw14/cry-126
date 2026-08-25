package service

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"time"

	"github.com/wyw14/cry-126/internal/basin"
	"github.com/wyw14/cry-126/internal/culvert"
	"github.com/wyw14/cry-126/internal/emergency"
	"github.com/wyw14/cry-126/internal/forecast"
	"github.com/wyw14/cry-126/internal/gate"
	"github.com/wyw14/cry-126/internal/interlock"
	"github.com/wyw14/cry-126/internal/journal"
	"github.com/wyw14/cry-126/internal/model"
	"github.com/wyw14/cry-126/internal/operation"
	"github.com/wyw14/cry-126/internal/pump"
	"github.com/wyw14/cry-126/internal/telemetry"
	"github.com/wyw14/cry-126/internal/tide"
)

type Runtime struct {
	mu              sync.RWMutex
	startedAt       time.Time
	journal         *journal.Coordinator
	snapshots       *journal.SnapshotStore
	barriers        *journal.BarrierState
	operations      *operation.Manager
	operationState  *operation.State
	lifecycle       *operation.Lifecycle
	basins          *basin.Coordinator
	culverts        *culvert.Coordinator
	culvertRecovery *culvert.Recovery
	pumps           *pump.Coordinator
	pumpRecovery    *pump.Recovery
	gates           *gate.Coordinator
	forecasts       *forecast.Coordinator
	telemetry       *telemetry.Receiver
	telemetryFlow   *telemetry.Coordinator
	tide            *tide.Coordinator
	interlocks      *interlock.Arbiter
	interlockState  *interlock.State
	emergency       *emergency.Coordinator
	emergencyState  *emergency.State
	lastDecision    interlock.BarrierDecision
}

func NewRuntime(ctx context.Context, dataDir string, now time.Time) (*Runtime, error) {
	if dataDir == "" {
		return nil, errors.New("data directory is required")
	}
	store, err := journal.Open(filepath.Join(dataDir, "events.jsonl"))
	if err != nil {
		return nil, err
	}
	journalCoordinator, err := journal.NewCoordinator(store)
	if err != nil {
		return nil, err
	}
	equipmentJournal, err := journal.NewEquipmentJournal(journalCoordinator)
	if err != nil {
		return nil, err
	}
	snapshotStore, err := journal.NewSnapshotStore(filepath.Join(dataDir, "snapshot.json"))
	if err != nil {
		return nil, err
	}
	operationState := operation.NewState(now)
	lifecycle := operation.NewLifecycle()
	operationManager, err := operation.NewManager(operationState, lifecycle, journalCoordinator)
	if err != nil {
		return nil, err
	}
	leaseTable := culvert.NewLeaseTable(time.Now)
	culvertCoordinator, err := culvert.NewCoordinator(leaseTable, equipmentJournal)
	if err != nil {
		return nil, err
	}
	culvertRecovery, err := culvert.NewRecovery(leaseTable)
	if err != nil {
		return nil, err
	}
	pumpState := pump.NewState([]string{"pump-east", "pump-west"}, now)
	pumpCoordinator, err := pump.NewCoordinator(pumpState, equipmentJournal, culvertCoordinator)
	if err != nil {
		return nil, err
	}
	pumpRecovery, err := pump.NewRecovery(pumpState, culvertRecovery)
	if err != nil {
		return nil, err
	}
	gateIDs := []string{"gate-1", "gate-2", "gate-3", "gate-4", "gate-5", "gate-6"}
	gateState := gate.NewState(gateIDs, now)
	gateController, err := gate.NewController(&gate.SimulatedExecutor{Delay: time.Millisecond, Failures: make(map[string]string)})
	if err != nil {
		return nil, err
	}
	gateCoordinator, err := gate.NewCoordinator(gateState, gateController, equipmentJournal)
	if err != nil {
		return nil, err
	}
	forecastState := forecast.NewState()
	forecastCoordinator, err := forecast.NewCoordinator(forecastState)
	if err != nil {
		return nil, err
	}
	telemetryState := telemetry.NewState(128)
	telemetryReceiver, err := telemetry.NewReceiver(telemetryState, equipmentJournal)
	if err != nil {
		return nil, err
	}
	telemetryCoordinator, err := telemetry.NewCoordinator(telemetryState, 0.08, 5*time.Minute)
	if err != nil {
		return nil, err
	}
	tideState := tide.NewState()
	tideCoordinator, err := tide.NewCoordinator(tideState, 2, 10*time.Minute)
	if err != nil {
		return nil, err
	}
	interlockState := interlock.NewState()
	arbiter, err := interlock.NewArbiter(interlockState, forecastCoordinator, tideCoordinator, interlock.PermitPolicy{DefenseThreshold: 3.8, MaximumSpread: 0.35, RequireQuorum: true})
	if err != nil {
		return nil, err
	}
	basinState, err := basin.NewState([]basin.Basin{
		{ID: "east", LevelMeters: 2.2, TargetMeters: 1.6, MaximumMeters: 3.2, PreferredPump: "pump-east", PreferredRoute: "culvert-main", UpdatedAt: now.UTC()},
		{ID: "west", LevelMeters: 2.0, TargetMeters: 1.5, MaximumMeters: 3.0, PreferredPump: "pump-west", PreferredRoute: "culvert-main", UpdatedAt: now.UTC()},
	})
	if err != nil {
		return nil, err
	}
	basinCoordinator, err := basin.NewCoordinator(basinState, culvertCoordinator, pumpCoordinator)
	if err != nil {
		return nil, err
	}
	emergencyState := emergency.NewState()
	emergencyCoordinator, err := emergency.NewCoordinator(emergencyState, interlockState, gateCoordinator, operationManager, gateIDs)
	if err != nil {
		return nil, err
	}
	runtime := &Runtime{
		startedAt: now.UTC(), journal: journalCoordinator, snapshots: snapshotStore, barriers: journal.NewBarrierState(),
		operations: operationManager, operationState: operationState, lifecycle: lifecycle,
		basins: basinCoordinator, culverts: culvertCoordinator, culvertRecovery: culvertRecovery,
		pumps: pumpCoordinator, pumpRecovery: pumpRecovery, gates: gateCoordinator,
		forecasts: forecastCoordinator, telemetry: telemetryReceiver, telemetryFlow: telemetryCoordinator,
		tide: tideCoordinator, interlocks: arbiter, interlockState: interlockState,
		emergency: emergencyCoordinator, emergencyState: emergencyState,
	}
	if err := runtime.bootstrap(ctx, now); err != nil {
		return nil, err
	}
	return runtime, nil
}

func (runtime *Runtime) bootstrap(ctx context.Context, now time.Time) error {
	snapshot, err := runtime.snapshots.Load()
	if err != nil {
		return err
	}
	events, err := runtime.journal.ReplayFrom(snapshot.Cursor)
	if err != nil {
		return err
	}
	if snapshot.Operation.Generation != 0 || len(events) > 0 {
		if _, err := runtime.Recover(ctx, now); err != nil {
			return err
		}
		return runtime.ensureEvidence(now)
	}
	operationValue, err := runtime.operations.Start(ctx, "service bootstrap", now)
	if err != nil {
		return err
	}
	if _, err := runtime.operations.Transition(operationValue, model.PhaseDefending, now.Add(time.Millisecond)); err != nil {
		return err
	}
	return runtime.ensureEvidence(now)
}

func (runtime *Runtime) ensureEvidence(now time.Time) error {
	points := []forecast.Point{
		{At: now.UTC(), Meters: 2.8},
		{At: now.Add(2 * time.Hour).UTC(), Meters: 3.2},
		{At: now.Add(4 * time.Hour).UTC(), Meters: 2.6},
	}
	if _, exists := runtime.forecasts.Current(); !exists {
		if _, err := runtime.UpdateForecast(points, now); err != nil {
			return err
		}
	}
	operationValue := runtime.operations.Current()
	for index, source := range []string{"outer-a", "outer-b"} {
		sample := telemetry.Sample{SourceID: source, Pressure: 27.5 + float64(index), ObservedAt: now.Add(time.Duration(index) * time.Second), ReceivedAt: now.Add(time.Duration(index) * time.Second)}
		items, err := runtime.telemetry.Receive(operationValue, sample)
		if err != nil {
			return err
		}
		for _, item := range items {
			if item.Kind == model.EvidenceLevel {
				if _, err := runtime.telemetryFlow.Observe(item); err != nil {
					return err
				}
			}
		}
		if _, err := runtime.tide.Ingest(items, sample.ReceivedAt); err != nil {
			return err
		}
	}
	_, err := runtime.interlocks.Recalculate(operationValue, now.Add(2*time.Second))
	return err
}

func (runtime *Runtime) restoreEquipment(snapshot model.SystemSnapshot, events []journal.Event, now time.Time) error {
	replayed, _, err := journal.Recover(snapshot, events)
	if err != nil {
		return err
	}
	if err := runtime.culvertRecovery.Load(replayed); err != nil {
		return err
	}
	if err := runtime.pumpRecovery.Load(replayed, now); err != nil {
		return err
	}
	pumps, err := runtime.pumpRecovery.Pumps()
	if err != nil {
		return err
	}
	if err := runtime.pumps.Restore(pumps); err != nil {
		return err
	}
	if err := runtime.gates.Restore(replayed.Gates); err != nil {
		return err
	}
	return nil
}

func (runtime *Runtime) Close() error {
	runtime.gates.Wait()
	snapshot := runtime.Snapshot()
	if err := runtime.snapshots.Save(snapshot); err != nil {
		return err
	}
	if operationValue := runtime.operations.Current(); operationValue.Generation != 0 {
		runtime.lifecycle.Stop(operationValue.Generation)
	}
	return nil
}

func (runtime *Runtime) StartedAt() time.Time {
	return runtime.startedAt
}
