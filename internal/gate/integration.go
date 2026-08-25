package gate

import (
	"context"
	"sync"
	"time"

	"github.com/wyw14/cry-126/internal/model"
)

type FleetResult struct {
	Operation model.Operation `json:"operation"`
	Gates     []model.Gate    `json:"gates"`
	Sealed    bool            `json:"sealed"`
	Failures  []string        `json:"failures"`
}

func CloseFleet(ctx context.Context, coordinator *Coordinator, operation model.Operation, gateIDs []string, now time.Time) FleetResult {
	result := FleetResult{Operation: operation, Gates: make([]model.Gate, len(gateIDs))}
	if coordinator == nil {
		result.Failures = append(result.Failures, "gate coordinator is required")
		return result
	}
	var wait sync.WaitGroup
	var mu sync.Mutex
	for index, id := range gateIDs {
		wait.Add(1)
		go func(position int, gateID string) {
			defer wait.Done()
			gate, err := coordinator.Move(ctx, gateID, operation, model.GateClosed, 0, now)
			mu.Lock()
			result.Gates[position] = gate
			if err != nil {
				result.Failures = append(result.Failures, gateID+": "+err.Error())
			}
			mu.Unlock()
		}(index, id)
	}
	wait.Wait()
	result.Sealed = FleetSealed(result.Gates, result.Failures)
	return result
}

func FleetSealed(gates []model.Gate, failures []string) bool {
	if len(gates) == 0 || len(failures) != 0 {
		return false
	}
	for _, item := range gates {
		if item.Position != model.GateClosed || item.Failure != "" {
			return false
		}
	}
	return true
}
