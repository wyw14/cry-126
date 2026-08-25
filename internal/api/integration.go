package api

import "github.com/wyw14/cry-126/internal/service"

func equipmentResponse(status service.Status) map[string]any {
	return map[string]any{
		"gates":       status.Gates,
		"pumps":       status.Pumps,
		"leases":      status.Leases,
		"basins":      status.Basins,
		"drain_plans": status.DrainPlans,
	}
}
