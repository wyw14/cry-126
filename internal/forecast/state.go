package forecast

import (
	"errors"
	"sync"

	"github.com/google/uuid"
)

type State struct {
	mu      sync.RWMutex
	current Window
	history []Window
}

func NewState() *State {
	return &State{}
}

func (state *State) Replace(window Window) error {
	if err := window.Revision.Validate(); err != nil {
		return err
	}
	if len(window.Points) < 2 {
		return errors.New("forecast window points are incomplete")
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.current.Revision.RevisionID != uuid.Nil {
		state.history = append(state.history, cloneWindow(state.current))
	}
	state.current = cloneWindow(window)
	return nil
}

func (state *State) Current() (Window, bool) {
	state.mu.RLock()
	defer state.mu.RUnlock()
	if state.current.Revision.RevisionID == uuid.Nil {
		return Window{}, false
	}
	return cloneWindow(state.current), true
}

func (state *State) History() []Window {
	state.mu.RLock()
	defer state.mu.RUnlock()
	result := make([]Window, len(state.history))
	for index := range state.history {
		result[index] = cloneWindow(state.history[index])
	}
	return result
}

func cloneWindow(window Window) Window {
	window.Points = append([]Point(nil), window.Points...)
	return window
}
