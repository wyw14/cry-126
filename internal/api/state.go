package api

import (
	"github.com/wyw14/cry-126/internal/journal"
	"github.com/wyw14/cry-126/internal/service"
)

func incidentResponse(status service.Status, barriers []journal.RecoveryBarrier) map[string]any {
	return map[string]any{
		"incidents":         status.Incidents,
		"closures":          status.Closures,
		"cursor":            status.Cursor,
		"recovery_barriers": barriers,
	}
}
