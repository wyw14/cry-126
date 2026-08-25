package culvert

import (
	"sync"
	"testing"
	"time"

	"github.com/wyw14/cry-126/internal/model"
)

func fixedClock(now time.Time) func() time.Time {
	return func() time.Time { return now }
}

func validDrainOperation(t *testing.T, generation uint64) model.Operation {
	t.Helper()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	operation := model.NewOperation(generation, "concurrency fixture", now)
	updated, err := model.TransitionOperation(operation, model.PhaseDefending, now.Add(time.Second))
	if err != nil {
		t.Fatalf("transition operation: %v", err)
	}
	return updated
}

// TestLeaseTableAcquireRejectsContendingOwner guards the conflict path
// serially: once east holds culvert-main, west must be rejected at the lease
// boundary so its pump group never starts.
func TestLeaseTableAcquireRejectsContendingOwner(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	table := NewLeaseTable(fixedClock(now))
	operation := validDrainOperation(t, 1)
	duration := 15 * time.Minute

	if _, err := table.Acquire("culvert-main", "east", model.DirectionOutward, operation, duration); err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if _, err := table.Acquire("culvert-main", "west", model.DirectionOutward, operation, duration); err == nil {
		t.Fatal("expected west to be rejected, but it acquired the contested culvert")
	}
}

// TestLeaseTableAcquireContestedCulvertIsMutuallyExclusive reproduces the
// parallel-drain regression: when east and west drain through the same
// culvert at once, the table must hand out exactly one lease so the losing
// basin is stopped before it reaches the pump group. The prior implementation
// dropped the lock between the conflict check and the lease assignment, so
// both basins minted a lease and both pump groups received opposing valve
// commands.
func TestLeaseTableAcquireContestedCulvertIsMutuallyExclusive(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	operation := validDrainOperation(t, 1)
	duration := 15 * time.Minute

	for attempt := 0; attempt < 64; attempt++ {
		table := NewLeaseTable(fixedClock(now))
		start := make(chan struct{})
		var wg sync.WaitGroup
		var mu sync.Mutex
		var winners []string
		wg.Add(2)
		for _, owner := range []string{"east", "west"} {
			owner := owner
			go func() {
				defer wg.Done()
				<-start
				lease, err := table.Acquire("culvert-main", owner, model.DirectionOutward, operation, duration)
				if err != nil {
					return
				}
				mu.Lock()
				winners = append(winners, lease.Owner)
				mu.Unlock()
			}()
		}
		close(start)
		wg.Wait()

		if len(winners) != 1 {
			t.Fatalf("attempt %d: expected exactly one lease holder, got %d (%v)", attempt, len(winners), winners)
		}
		current, ok := table.Current("culvert-main")
		if !ok || current.Owner != winners[0] {
			t.Fatalf("attempt %d: current owner %q does not match winner %q", attempt, current.Owner, winners[0])
		}
	}
}

// TestLeaseTableAcquireReissuesSameOwner confirms the happy path still works:
// the same owner re-acquiring an identical lease gets the existing lease back
// instead of a conflict.
func TestLeaseTableAcquireReissuesSameOwner(t *testing.T) {
	now := time.Date(2026, 8, 25, 10, 0, 0, 0, time.UTC)
	table := NewLeaseTable(fixedClock(now))
	operation := validDrainOperation(t, 1)
	duration := 15 * time.Minute

	first, err := table.Acquire("culvert-main", "east", model.DirectionOutward, operation, duration)
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	second, err := table.Acquire("culvert-main", "east", model.DirectionOutward, operation, duration)
	if err != nil {
		t.Fatalf("reissue acquire: %v", err)
	}
	if second.ID != first.ID || second.FencingToken != first.FencingToken {
		t.Fatalf("expected reissued lease, got %v then %v", first, second)
	}
}
