package forecast

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

type Coordinator struct {
	state *State
}

func NewCoordinator(state *State) (*Coordinator, error) {
	if state == nil {
		return nil, errors.New("forecast state is required")
	}
	return &Coordinator{state: state}, nil
}

func (coordinator *Coordinator) Update(points []Point, issuedAt time.Time) (Window, error) {
	window, err := NewWindow(points, issuedAt)
	if err != nil {
		return Window{}, err
	}
	if err := coordinator.state.Replace(window); err != nil {
		return Window{}, err
	}
	return window, nil
}

func (coordinator *Coordinator) Current() (Window, bool) {
	return coordinator.state.Current()
}

func (coordinator *Coordinator) CurrentRevision() uuid.UUID {
	window, exists := coordinator.state.Current()
	if !exists {
		return uuid.Nil
	}
	return window.RevisionID()
}

func (coordinator *Coordinator) History() []Window {
	return coordinator.state.History()
}
