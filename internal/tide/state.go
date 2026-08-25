package tide

import (
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/wyw14/cry-126/internal/model"
)

type Vote struct {
	SourceID   string    `json:"source_id"`
	EvidenceID uuid.UUID `json:"evidence_id"`
	RevisionID uuid.UUID `json:"revision_id"`
	Meters     float64   `json:"meters"`
	ObservedAt time.Time `json:"observed_at"`
	Kind       string    `json:"kind"`
	Derived    bool      `json:"derived"`
}

type State struct {
	mu       sync.RWMutex
	votes    map[string]Vote
	revision uuid.UUID
}

func NewState() *State {
	return &State{votes: make(map[string]Vote), revision: uuid.New()}
}

func (state *State) Put(evidence model.Evidence) Vote {
	vote := Vote{
		SourceID:   evidence.SourceID,
		EvidenceID: evidence.ID,
		RevisionID: evidence.RevisionID,
		Meters:     evidence.Value,
		ObservedAt: evidence.ObservedAt,
		Kind:       string(evidence.Kind),
		Derived:    evidence.Derived,
	}
	state.mu.Lock()
	current, exists := state.votes[evidence.SourceID]
	if !exists || !vote.ObservedAt.Before(current.ObservedAt) {
		state.votes[evidence.SourceID] = vote
		state.revision = uuid.New()
	}
	state.mu.Unlock()
	return vote
}

func (state *State) Votes() []Vote {
	state.mu.RLock()
	defer state.mu.RUnlock()
	result := make([]Vote, 0, len(state.votes))
	for _, vote := range state.votes {
		result = append(result, vote)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].SourceID < result[j].SourceID })
	return result
}

func (state *State) Revision() uuid.UUID {
	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.revision
}

func (state *State) ClearBefore(cutoff time.Time) int {
	state.mu.Lock()
	defer state.mu.Unlock()
	removed := 0
	for source, vote := range state.votes {
		if vote.ObservedAt.Before(cutoff) {
			delete(state.votes, source)
			removed++
		}
	}
	if removed > 0 {
		state.revision = uuid.New()
	}
	return removed
}
