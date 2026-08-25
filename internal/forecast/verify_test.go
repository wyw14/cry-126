package forecast_test

import (
	"context"
	"testing"
	"time"

	"github.com/wyw14/cry-126/internal/forecast"
	"github.com/wyw14/cry-126/internal/service"
)

func TestForecastRevisionRevokesObsoleteCrestPermit(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	runtime, err := service.NewRuntime(context.Background(), t.TempDir(), now)
	if err != nil {
		t.Fatal(err)
	}
	first, err := runtime.UpdateForecast([]forecast.Point{{At: now, Meters: 2.1}, {At: now.Add(time.Hour), Meters: 2.4}}, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := runtime.UpdateForecast([]forecast.Point{{At: now, Meters: 4.4}, {At: now.Add(time.Hour), Meters: 4.8}}, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if first.RevisionID() == second.RevisionID() {
		t.Fatal("forecast revision did not advance")
	}
	permit := runtime.Status(now.Add(2 * time.Minute)).Permit
	if permit.ForecastRevisionID != second.RevisionID() {
		t.Fatalf("permit still references forecast %s, want %s", permit.ForecastRevisionID, second.RevisionID())
	}
}
