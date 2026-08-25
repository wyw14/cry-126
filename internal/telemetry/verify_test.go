package telemetry

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/wyw14/cry-126/internal/model"
)

func TestCalmWindowBreaksAcrossSensorClockStep(t *testing.T) {
	base := time.Unix(1700000000, 0).UTC()
	coordinator, err := NewCoordinator(NewState(32), 0.1, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	for _, offset := range []time.Duration{0, 4 * time.Minute, 30 * time.Second, 5 * time.Minute} {
		if _, err := coordinator.Observe(model.NewEvidence("outer", model.EvidenceLevel, uuid.New(), 2.1, base.Add(offset), base.Add(offset), false)); err != nil {
			t.Fatal(err)
		}
	}
	if coordinator.Current("outer").Calm {
		t.Fatal("calm window crossed a sensor clock rollback")
	}
}
