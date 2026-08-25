package tide

import (
	"errors"
	"math"
	"sort"
	"time"

	"github.com/google/uuid"
)

type Assessment struct {
	RevisionID        uuid.UUID `json:"revision_id"`
	IndependentVotes  int       `json:"independent_votes"`
	RequiredVotes     int       `json:"required_votes"`
	MedianMeters      float64   `json:"median_meters"`
	MaximumDifference float64   `json:"maximum_difference"`
	Quorum            bool      `json:"quorum"`
	AssessedAt        time.Time `json:"assessed_at"`
}

func Assess(revision uuid.UUID, votes []Vote, required int, maxAge time.Duration, now time.Time) (Assessment, error) {
	if revision == uuid.Nil || required < 1 || maxAge <= 0 {
		return Assessment{}, errors.New("tide assessment settings are invalid")
	}
	fresh := make([]Vote, 0, len(votes))
	seen := make(map[string]struct{}, len(votes))
	for _, vote := range votes {
		if vote.SourceID == "" || vote.ObservedAt.After(now) || now.Sub(vote.ObservedAt) > maxAge {
			continue
		}
		if _, exists := seen[vote.SourceID]; exists {
			continue
		}
		seen[vote.SourceID] = struct{}{}
		fresh = append(fresh, vote)
	}
	values := make([]float64, 0, len(fresh))
	for _, vote := range fresh {
		values = append(values, vote.Meters)
	}
	sort.Float64s(values)
	assessment := Assessment{
		RevisionID:       revision,
		IndependentVotes: len(values),
		RequiredVotes:    required,
		AssessedAt:       now.UTC(),
		Quorum:           len(values) >= required,
	}
	if len(values) > 0 {
		middle := len(values) / 2
		assessment.MedianMeters = values[middle]
		if len(values)%2 == 0 {
			assessment.MedianMeters = (values[middle-1] + values[middle]) / 2
		}
		assessment.MaximumDifference = math.Abs(values[len(values)-1] - values[0])
	}
	return assessment, nil
}

func AboveThreshold(assessment Assessment, threshold float64) bool {
	return assessment.Quorum && assessment.MedianMeters >= threshold
}

func Stable(assessment Assessment, tolerance float64) bool {
	return assessment.Quorum && assessment.MaximumDifference <= tolerance
}
