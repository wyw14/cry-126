package interlock

import (
	"errors"
	"time"

	"github.com/wyw14/cry-126/internal/model"
)

func AcquireStormLock(state *State, operation model.Operation, now time.Time) (Lock, error) {
	if state == nil {
		return Lock{}, errors.New("interlock state is required")
	}
	return state.Acquire("storm-barrier", "emergency-closure", operation, now)
}

func ReleaseStormLock(state *State, lock Lock) bool {
	if state == nil {
		return false
	}
	return state.Release(lock)
}
