package service

import (
	"context"
	"testing"
	"time"
)

func TestRecoveryLoadsCulvertFencingBeforePumpControllers(t *testing.T) {
	now := time.Unix(1700000000, 0).UTC()
	runtime, err := NewRuntime(context.Background(), t.TempDir(), now)
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Close(); err != nil {
		t.Fatal(err)
	}
	sequence, err := runtime.Recover(context.Background(), now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if !sequence.FencingLoaded || !sequence.PumpsLoaded {
		t.Fatalf("recovery stages incomplete: %+v", sequence)
	}
}
