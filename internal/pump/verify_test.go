package pump

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/wyw14/cry-126/internal/culvert"
	"github.com/wyw14/cry-126/internal/journal"
	"github.com/wyw14/cry-126/internal/model"
)

func TestDrainageStateWaitsForDurableCheckValveProof(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	equipment, journalPath := testEquipmentJournal(t)
	table := culvert.NewLeaseTable(func() time.Time { return now })
	culverts, err := culvert.NewCoordinator(table, equipment)
	if err != nil {
		t.Fatal(err)
	}
	state := NewState([]string{"pump-east"}, now)
	coordinator, err := NewCoordinator(state, equipment, culverts)
	if err != nil {
		t.Fatal(err)
	}
	operation := model.NewOperation(1, "proof ordering", now)
	request, err := culvert.PrepareRequest("east", "culvert-main", model.DirectionOutward, operation, time.Minute, now)
	if err != nil {
		t.Fatal(err)
	}
	grant, err := culverts.AcquireForDrain("culvert-main", "east", model.DirectionOutward, operation, time.Minute, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(journalPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Dir(journalPath)); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Dir(journalPath), []byte("blocked"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := coordinator.Start("pump-east", request, grant, now); err == nil {
		t.Fatal("expected durable proof failure")
	}
	pump := state.Snapshot()[0]
	if pump.Mode == model.PumpDraining {
		t.Fatal("pump became draining before proof was durable")
	}
}

func testEquipmentJournal(t *testing.T) (*journal.EquipmentJournal, string) {
	t.Helper()
	journalPath := filepath.Join(t.TempDir(), "journal", "events.jsonl")
	store, err := journal.Open(journalPath)
	if err != nil {
		t.Fatal(err)
	}
	coordinator, err := journal.NewCoordinator(store)
	if err != nil {
		t.Fatal(err)
	}
	equipment, err := journal.NewEquipmentJournal(coordinator)
	if err != nil {
		t.Fatal(err)
	}
	return equipment, journalPath
}
