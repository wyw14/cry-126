package basin

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/wyw14/cry-126/internal/culvert"
	"github.com/wyw14/cry-126/internal/journal"
	"github.com/wyw14/cry-126/internal/model"
	"github.com/wyw14/cry-126/internal/pump"
)

func drainFixture(t *testing.T, now time.Time) (model.Operation, *culvert.Coordinator, *pump.Coordinator, *culvert.LeaseTable) {
	t.Helper()
	tempDir := t.TempDir()
	store, err := journal.Open(filepath.Join(tempDir, "events.jsonl"))
	if err != nil {
		t.Fatalf("open journal: %v", err)
	}
	coordinator, err := journal.NewCoordinator(store)
	if err != nil {
		t.Fatalf("new journal coordinator: %v", err)
	}
	equipmentJournal, err := journal.NewEquipmentJournal(coordinator)
	if err != nil {
		t.Fatalf("new equipment journal: %v", err)
	}
	table := culvert.NewLeaseTable(func() time.Time { return now })
	culverts, err := culvert.NewCoordinator(table, equipmentJournal)
	if err != nil {
		t.Fatalf("new culvert coordinator: %v", err)
	}
	pumpState := pump.NewState([]string{"pump-east", "pump-west"}, now)
	pumps, err := pump.NewCoordinator(pumpState, equipmentJournal, culverts)
	if err != nil {
		t.Fatalf("new pump coordinator: %v", err)
	}
	operation := model.NewOperation(1, "parallel drain fixture", now)
	updated, err := model.TransitionOperation(operation, model.PhaseDefending, now.Add(time.Second))
	if err != nil {
		t.Fatalf("transition operation: %v", err)
	}
	return updated, culverts, pumps, table
}

// TestStartDrainContestedCulvertBlocksLosingBasin reproduces the field
// symptom: east and west drain through the shared culvert-main at the same
// instant. Exactly one pump group may reach draining; the losing basin must
// be rejected before its pump starts, so no two owners share the culvert and
// no opposing valve commands are issued.
func TestStartDrainContestedCulvertBlocksLosingBasin(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	operation, culverts, pumps, _ := drainFixture(t, now)

	basins := []Basin{
		{ID: "east", LevelMeters: 2.2, TargetMeters: 1.6, MaximumMeters: 3.2, PreferredPump: "pump-east", PreferredRoute: "culvert-main", UpdatedAt: now.UTC()},
		{ID: "west", LevelMeters: 2.0, TargetMeters: 1.5, MaximumMeters: 3.0, PreferredPump: "pump-west", PreferredRoute: "culvert-main", UpdatedAt: now.UTC()},
	}
	basinState, err := NewState(basins)
	if err != nil {
		t.Fatalf("new basin state: %v", err)
	}
	coordinator, err := NewCoordinator(basinState, culverts, pumps)
	if err != nil {
		t.Fatalf("new basin coordinator: %v", err)
	}

	start := make(chan struct{})
	var wg sync.WaitGroup
	type outcome struct {
		basin     string
		succeeded bool
		err       error
	}
	var results []outcome
	var mu sync.Mutex
	wg.Add(2)
	for _, basin := range []string{"east", "west"} {
		basin := basin
		go func() {
			defer wg.Done()
			<-start
			_, err := coordinator.StartDrain(basin, operation, now)
			mu.Lock()
			results = append(results, outcome{basin: basin, succeeded: err == nil, err: err})
			mu.Unlock()
		}()
	}
	close(start)
	wg.Wait()

	succeeded := 0
	for _, result := range results {
		if result.succeeded {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("expected exactly one basin to start draining, got %d (results: %v)", succeeded, results)
	}

	// The culvert must report a single authoritative owner; both groups
	// issuing valve commands implies two owners, which the fix prevents.
	snapshot := culverts.Snapshot()
	if len(snapshot) != 1 {
		t.Fatalf("expected one culvert lease, got %d", len(snapshot))
	}
	owner := snapshot[0].Owner
	for _, result := range results {
		if result.succeeded && result.basin != owner {
			t.Fatalf("winning basin %q does not match lease owner %q", result.basin, owner)
		}
	}

	// Exactly one pump must be draining; the losing pump must remain stopped.
	pumpSnapshot := pumps.State()
	draining := 0
	for _, item := range pumpSnapshot {
		if item.Mode == model.PumpDraining {
			draining++
		}
	}
	if draining != 1 {
		t.Fatalf("expected exactly one draining pump, got %d", draining)
	}
}
