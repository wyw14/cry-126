package culvert

import (
	"errors"
	"time"

	"github.com/wyw14/cry-126/internal/model"
)

type DrainageRequest struct {
	Basin       string               `json:"basin"`
	CulvertID   string               `json:"culvert_id"`
	Direction   model.LeaseDirection `json:"direction"`
	Duration    time.Duration        `json:"duration"`
	Operation   model.Operation      `json:"operation"`
	RequestedAt time.Time            `json:"requested_at"`
}

func (request DrainageRequest) Validate() error {
	if request.Basin == "" || request.CulvertID == "" {
		return errors.New("drainage request requires basin and culvert")
	}
	if request.Direction != model.DirectionInward && request.Direction != model.DirectionOutward {
		return errors.New("drainage request direction is invalid")
	}
	if request.Duration <= 0 || request.RequestedAt.IsZero() {
		return errors.New("drainage request duration and time are required")
	}
	return request.Operation.Validate()
}

func PrepareRequest(basin, culvertID string, direction model.LeaseDirection, operation model.Operation, duration time.Duration, now time.Time) (DrainageRequest, error) {
	request := DrainageRequest{
		Basin:       basin,
		CulvertID:   culvertID,
		Direction:   direction,
		Duration:    duration,
		Operation:   operation,
		RequestedAt: now.UTC(),
	}
	if err := request.Validate(); err != nil {
		return DrainageRequest{}, err
	}
	return request, nil
}

func GrantRequest(coordinator *Coordinator, request DrainageRequest) (StartGrant, error) {
	if coordinator == nil {
		return StartGrant{}, errors.New("culvert coordinator is required")
	}
	if err := request.Validate(); err != nil {
		return StartGrant{}, err
	}
	return coordinator.AcquireForDrain(request.CulvertID, request.Basin, request.Direction, request.Operation, request.Duration, request.RequestedAt)
}

func SameOwnership(grant StartGrant, request DrainageRequest) bool {
	return grant.Lease.CulvertID == request.CulvertID &&
		grant.Lease.Owner == request.Basin &&
		grant.Lease.Direction == request.Direction &&
		grant.Lease.OperationID == request.Operation.ID &&
		grant.Lease.OperationGeneration == request.Operation.Generation
}
