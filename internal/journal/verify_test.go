package journal

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestStatusCursorDoesNotSkipUndurableBarrierEvent(t *testing.T) {
	store, err := Open(t.TempDir() + "/events.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	coordinator, err := NewCoordinator(store)
	if err != nil {
		t.Fatal(err)
	}
	operation := uuid.New()
	event, err := NewEvent(operation, 1, "operation.updated", "barrier", map[string]string{"state": "preparing"}, time.Unix(1700000000, 0))
	if err != nil {
		t.Fatal(err)
	}
	store.path = filepath.Join(t.TempDir(), "missing", "events.jsonl")
	if _, err := coordinator.Commit(event); err == nil {
		t.Fatal("expected storage error")
	}
	if coordinator.Cursor() != 0 {
		t.Fatalf("published cursor advanced to %d after failed durable append", coordinator.Cursor())
	}
}
