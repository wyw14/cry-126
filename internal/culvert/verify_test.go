package culvert

import (
	"sync"
	"testing"
	"time"

	"github.com/wyw14/cry-126/internal/model"
)

func TestConcurrentDrainRequestsKeepCulvertExclusive(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	table := NewLeaseTable(func() time.Time { return now })
	operation := model.NewOperation(1, "parallel drain", now)
	var wait sync.WaitGroup
	results := make(chan error, 2)
	for _, owner := range []string{"east", "west"} {
		wait.Add(1)
		go func(owner string) {
			defer wait.Done()
			_, err := table.Acquire("culvert-main", owner, model.DirectionOutward, operation, time.Minute)
			results <- err
		}(owner)
	}
	wait.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("expected exactly one drainage owner, got %d", successes)
	}
}
