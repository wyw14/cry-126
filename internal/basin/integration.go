package basin

import (
	"errors"
	"sort"
	"time"

	"github.com/wyw14/cry-126/internal/culvert"
	"github.com/wyw14/cry-126/internal/model"
)

type Plan struct {
	Basin     Basin                   `json:"basin"`
	Deficit   float64                 `json:"deficit"`
	Urgency   float64                 `json:"urgency"`
	PumpID    string                  `json:"pump_id"`
	Request   culvert.DrainageRequest `json:"request"`
	CreatedAt time.Time               `json:"created_at"`
}

func BuildPlan(item Basin, operation model.Operation, direction model.LeaseDirection, duration time.Duration, now time.Time) (Plan, error) {
	if err := validate(item); err != nil {
		return Plan{}, err
	}
	if Deficit(item) <= 0 {
		return Plan{}, errors.New("basin does not require drainage")
	}
	request, err := culvert.PrepareRequest(item.ID, item.PreferredRoute, direction, operation, duration, now)
	if err != nil {
		return Plan{}, err
	}
	return Plan{
		Basin:     item,
		Deficit:   Deficit(item),
		Urgency:   Urgency(item),
		PumpID:    item.PreferredPump,
		Request:   request,
		CreatedAt: now.UTC(),
	}, nil
}

func PlansFor(items []Basin, operation model.Operation, duration time.Duration, now time.Time) []Plan {
	result := make([]Plan, 0, len(items))
	for _, item := range items {
		plan, err := BuildPlan(item, operation, model.DirectionOutward, duration, now)
		if err == nil {
			result = append(result, plan)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Urgency == result[j].Urgency {
			return result[i].CreatedAt.Before(result[j].CreatedAt)
		}
		return result[i].Urgency > result[j].Urgency
	})
	return result
}
