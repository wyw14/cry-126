package forecast

import (
	"errors"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/wyw14/cry-126/internal/model"
)

type Point struct {
	At     time.Time `json:"at"`
	Meters float64   `json:"meters"`
}

type Window struct {
	Revision model.Forecast `json:"revision"`
	Points   []Point        `json:"points"`
	FrozenAt time.Time      `json:"frozen_at"`
}

func NewWindow(points []Point, issuedAt time.Time) (Window, error) {
	if len(points) < 2 {
		return Window{}, errors.New("forecast window needs at least two points")
	}
	ordered := append([]Point(nil), points...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].At.Before(ordered[j].At) })
	peak := ordered[0]
	for index, point := range ordered {
		if point.At.IsZero() {
			return Window{}, errors.New("forecast point timestamp is required")
		}
		if index > 0 && !point.At.After(ordered[index-1].At) {
			return Window{}, errors.New("forecast point timestamps must advance")
		}
		if point.Meters > peak.Meters {
			peak = point
		}
	}
	window := Window{
		Revision: model.Forecast{
			RevisionID: uuid.New(),
			PeakMeters: peak.Meters,
			PeakAt:     peak.At.UTC(),
			IssuedAt:   issuedAt.UTC(),
		},
		Points: ordered,
	}
	return window.Freeze(issuedAt), nil
}

func (window Window) Freeze(now time.Time) Window {
	window.Points = append([]Point(nil), window.Points...)
	window.FrozenAt = now.UTC()
	return window
}

func (window Window) RevisionID() uuid.UUID {
	return window.Revision.RevisionID
}
