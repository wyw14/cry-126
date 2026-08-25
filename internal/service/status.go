package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/wyw14/cry-126/internal/basin"
	"github.com/wyw14/cry-126/internal/emergency"
	"github.com/wyw14/cry-126/internal/forecast"
	"github.com/wyw14/cry-126/internal/interlock"
	"github.com/wyw14/cry-126/internal/journal"
	"github.com/wyw14/cry-126/internal/model"
	"github.com/wyw14/cry-126/internal/telemetry"
	"github.com/wyw14/cry-126/internal/tide"
)

type Status struct {
	Service             string                          `json:"service"`
	StartedAt           time.Time                       `json:"started_at"`
	Operation           model.Operation                 `json:"operation"`
	Forecast            model.Forecast                  `json:"forecast"`
	Permit              model.Permit                    `json:"permit"`
	Assessment          tide.Assessment                 `json:"assessment"`
	TideVotes           []tide.Vote                     `json:"tide_votes"`
	Gates               []model.Gate                    `json:"gates"`
	Pumps               []model.Pump                    `json:"pumps"`
	Leases              []model.CulvertLease            `json:"leases"`
	Basins              []basin.Basin                   `json:"basins"`
	DrainPlans          []basin.Plan                    `json:"drain_plans"`
	Locks               []interlock.Lock                `json:"locks"`
	Closures            []emergency.Closure             `json:"closures"`
	Evidence            []model.Evidence                `json:"evidence"`
	TelemetryRevision   uuid.UUID                       `json:"telemetry_revision"`
	CalmWindows         map[string]telemetry.CalmWindow `json:"calm_windows"`
	TelemetryWindows    map[string][]model.Evidence     `json:"telemetry_windows"`
	Incidents           []model.Incident                `json:"incidents"`
	ForecastHistory     []forecast.Window               `json:"forecast_history"`
	CompletedOperations []model.Operation               `json:"completed_operations"`
	ActiveGenerations   []uint64                        `json:"active_generations"`
	Cursor              uint64                          `json:"cursor"`
	Barrier             interlock.BarrierDecision       `json:"barrier"`
}

func (runtime *Runtime) Status(now time.Time) Status {
	window, _ := runtime.forecasts.Current()
	assessment, _ := runtime.tide.Assess(now)
	snapshot := runtime.Snapshot()
	latest := runtime.telemetry.Latest()
	calmWindows := make(map[string]telemetry.CalmWindow, len(latest))
	telemetryWindows := make(map[string][]model.Evidence, len(latest))
	for _, item := range latest {
		calmWindows[item.SourceID] = runtime.telemetryFlow.Current(item.SourceID)
		telemetryWindows[item.SourceID] = runtime.telemetryFlow.Samples(item.SourceID)
	}
	runtime.mu.RLock()
	decision := runtime.lastDecision
	runtime.mu.RUnlock()
	return Status{
		Service: "TideShield", StartedAt: runtime.startedAt, Operation: runtime.operations.Current(), Forecast: window.Revision,
		Permit: runtime.interlockState.Permit(), Assessment: assessment, TideVotes: runtime.tide.Votes(), Gates: runtime.gates.Gates(), Pumps: runtime.pumps.State(),
		Leases: runtime.culverts.Snapshot(), Basins: runtime.basins.Basins(), DrainPlans: runtime.basins.Plans(runtime.operations.Current(), now), Locks: runtime.interlocks.Locks(),
		Closures: runtime.emergency.Closures(), Evidence: latest, TelemetryRevision: runtime.telemetry.Revision(), CalmWindows: calmWindows, TelemetryWindows: telemetryWindows, Incidents: snapshot.Incidents,
		ForecastHistory: runtime.forecasts.History(), CompletedOperations: runtime.operationState.Completed(), ActiveGenerations: runtime.lifecycle.ActiveGenerations(),
		Cursor: runtime.journal.Cursor(), Barrier: decision,
	}
}

func (runtime *Runtime) Snapshot() model.SystemSnapshot {
	window, _ := runtime.forecasts.Current()
	return model.SystemSnapshot{
		Operation: runtime.operations.Current(), Forecast: window.Revision, Permit: runtime.interlockState.Permit(),
		Gates: runtime.gates.Gates(), Pumps: runtime.pumps.State(), Leases: runtime.culverts.Snapshot(),
		Evidence: runtime.telemetry.Latest(), Incidents: model.ActiveIncidents(runtime.StatusIncidents()), Cursor: runtime.journal.Cursor(), SavedAt: time.Now().UTC(),
	}
}

func (runtime *Runtime) StartOperation(ctx context.Context, reason string, now time.Time) (model.Operation, error) {
	current := runtime.operations.Current()
	if current.Generation != 0 && current.Phase != model.PhaseClosed && current.Phase != model.PhaseStandby {
		return model.Operation{}, errors.New("an operation is already active")
	}
	for _, item := range runtime.telemetry.Latest() {
		runtime.telemetryFlow.Reset(item.SourceID)
	}
	operationValue, err := runtime.operations.Start(ctx, reason, now)
	if err != nil {
		return model.Operation{}, err
	}
	return runtime.operations.Transition(operationValue, model.PhaseDefending, now.Add(time.Millisecond))
}

func (runtime *Runtime) UpdateForecast(points []forecast.Point, now time.Time) (forecast.Window, error) {
	previous := runtime.forecasts.CurrentRevision()
	window, err := runtime.forecasts.Update(points, now)
	if err != nil {
		return forecast.Window{}, err
	}
	if previous != uuid.Nil && previous != window.RevisionID() {
		runtime.interlocks.InvalidateForecast(window.RevisionID())
	}
	if runtime.operations.Current().Generation != 0 && runtime.tide.EvidenceRevision() != uuid.Nil {
		if _, err := runtime.interlocks.Recalculate(runtime.operations.Current(), now); err != nil {
			return forecast.Window{}, err
		}
	}
	return window, nil
}

func (runtime *Runtime) ReceiveSample(sample telemetry.Sample) ([]model.Evidence, error) {
	operationValue := runtime.operations.Current()
	items, err := runtime.telemetry.Receive(operationValue, sample)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.Kind == model.EvidenceLevel {
			if _, err := runtime.telemetryFlow.Observe(item); err != nil {
				return nil, err
			}
		}
	}
	if _, err := runtime.tide.Ingest(items, sample.ReceivedAt); err != nil {
		return nil, err
	}
	if _, err := runtime.interlocks.Recalculate(operationValue, sample.ReceivedAt); err != nil {
		return nil, err
	}
	return items, nil
}

func (runtime *Runtime) Drain(basinID string, now time.Time) (basin.DrainResult, error) {
	permit, err := runtime.interlocks.Current(runtime.operations.Current())
	if err != nil {
		return basin.DrainResult{}, err
	}
	window, exists := runtime.forecasts.Current()
	if !exists {
		return basin.DrainResult{}, errors.New("forecast is unavailable")
	}
	if err := interlock.RequirePermit(permit, runtime.operations.Current(), window.Revision, runtime.tide.EvidenceRevision()); err != nil {
		return basin.DrainResult{}, err
	}
	return runtime.basins.StartDrain(basinID, runtime.operations.Current(), now)
}

func (runtime *Runtime) UpdateBasinLevel(basinID string, meters float64, now time.Time) (basin.Basin, error) {
	return runtime.basins.UpdateLevel(basinID, meters, now)
}

func (runtime *Runtime) ReverseGate(ctx context.Context, gateID string, now time.Time) (model.Gate, error) {
	return runtime.gates.Reverse(ctx, gateID, runtime.operations.Current(), now)
}

func (runtime *Runtime) StopPump(pumpID, reason string, now time.Time) (model.Pump, error) {
	return runtime.pumps.Stop(pumpID, runtime.operations.Current().Generation, reason, now)
}

func (runtime *Runtime) StatusIncidents() []model.Incident {
	snapshot, err := runtime.snapshots.Load()
	if err != nil {
		return nil
	}
	events, err := runtime.journal.ReplayFrom(snapshot.Cursor)
	if err != nil {
		return nil
	}
	replayed, _, err := journal.Recover(snapshot, events)
	if err != nil {
		return nil
	}
	return replayed.Incidents
}
