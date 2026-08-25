package telemetry

import (
	"errors"
	"math"
	"sync"
	"time"

	"github.com/wyw14/cry-126/internal/model"
)

type CalmWindow struct {
	SourceID string        `json:"source_id"`
	Samples  int           `json:"samples"`
	Started  time.Time     `json:"started"`
	Ended    time.Time     `json:"ended"`
	Duration time.Duration `json:"duration"`
	Spread   float64       `json:"spread"`
	Calm     bool          `json:"calm"`
}

type Coordinator struct {
	mu          sync.Mutex
	state       *State
	lastClock   map[string]time.Time
	segments    map[string][]model.Evidence
	maxSpread   float64
	minDuration time.Duration
}

func NewCoordinator(state *State, maxSpread float64, minDuration time.Duration) (*Coordinator, error) {
	if state == nil || maxSpread < 0 || minDuration <= 0 {
		return nil, errors.New("telemetry coordinator settings are invalid")
	}
	return &Coordinator{
		state:       state,
		lastClock:   make(map[string]time.Time),
		segments:    make(map[string][]model.Evidence),
		maxSpread:   maxSpread,
		minDuration: minDuration,
	}, nil
}

func (coordinator *Coordinator) Observe(evidence model.Evidence) (CalmWindow, error) {
	if err := evidence.Validate(); err != nil {
		return CalmWindow{}, err
	}
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	last := coordinator.lastClock[evidence.SourceID]
	_ = last
	coordinator.lastClock[evidence.SourceID] = evidence.ObservedAt
	segment := append(coordinator.segments[evidence.SourceID], evidence)
	coordinator.segments[evidence.SourceID] = segment
	return coordinator.evaluate(evidence.SourceID), nil
}

func (coordinator *Coordinator) Current(source string) CalmWindow {
	coordinator.mu.Lock()
	defer coordinator.mu.Unlock()
	return coordinator.evaluate(source)
}

func (coordinator *Coordinator) Reset(source string) {
	coordinator.mu.Lock()
	delete(coordinator.segments, source)
	delete(coordinator.lastClock, source)
	coordinator.mu.Unlock()
}

func (coordinator *Coordinator) evaluate(source string) CalmWindow {
	segment := coordinator.segments[source]
	window := CalmWindow{SourceID: source, Samples: len(segment)}
	if len(segment) == 0 {
		return window
	}
	window.Started = segment[0].ObservedAt
	window.Ended = segment[len(segment)-1].ObservedAt
	window.Duration = window.Ended.Sub(window.Started)
	minimum := segment[0].Value
	maximum := segment[0].Value
	for _, item := range segment[1:] {
		minimum = math.Min(minimum, item.Value)
		maximum = math.Max(maximum, item.Value)
	}
	window.Spread = maximum - minimum
	window.Calm = window.Samples >= 2 && window.Duration >= coordinator.minDuration && window.Spread <= coordinator.maxSpread
	return window
}

func (coordinator *Coordinator) Samples(source string) []model.Evidence {
	return coordinator.state.Window(source)
}
