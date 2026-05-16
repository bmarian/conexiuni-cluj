package handlers

import (
	"testing"
	"time"
)

func testLearningTime(hour, min, sec int) (time.Time, *time.Location) {
	loc := time.FixedZone("Europe/Bucharest", 2*60*60)
	return time.Date(2026, 5, 16, hour, min, sec, 0, loc), loc
}

func TestComputeVehicleLearningPlanPausesWhenVehicleQuotaAheadOfClock(t *testing.T) {
	now, loc := testLearningTime(6, 0, 0)

	plan := computeVehicleLearningPlan(
		3000,
		4800,
		750,
		3000,
		now,
		loc,
	)

	if plan.Ready {
		t.Fatalf("plan should not be ready while total vehicle quota is ahead of the clock")
	}
	if plan.VehiclesUsed != 1800 {
		t.Fatalf("vehicles used = %d, want 1800", plan.VehiclesUsed)
	}
	if plan.Interval < 2*time.Hour+59*time.Minute || plan.Interval > 3*time.Hour+time.Minute {
		t.Fatalf("interval = %s, want about 3h until the quota schedule catches up", plan.Interval)
	}
}

func TestComputeVehicleLearningPlanUsesFairIntervalWhenOnPace(t *testing.T) {
	now, loc := testLearningTime(6, 0, 0)

	plan := computeVehicleLearningPlan(
		3600,
		4800,
		750,
		3000,
		now,
		loc,
	)

	if plan.Ready {
		t.Fatalf("plan should wait for the next evenly spaced learner slot")
	}
	if plan.Interval != 28*time.Second+800*time.Millisecond {
		t.Fatalf("interval = %s, want 28.8s", plan.Interval)
	}
	if plan.ShelfLife != 27*time.Second+800*time.Millisecond {
		t.Fatalf("shelfLife = %s, want 27.8s", plan.ShelfLife)
	}
}

func TestComputeVehicleLearningPlanReadyAfterNextLearnerSlot(t *testing.T) {
	now, loc := testLearningTime(6, 0, 36)

	plan := computeVehicleLearningPlan(
		3599,
		4800,
		750,
		3000,
		now,
		loc,
	)

	if !plan.Ready {
		t.Fatalf("plan should be ready after the next learner slot has arrived")
	}
	if plan.Interval != 28*time.Second+800*time.Millisecond {
		t.Fatalf("interval = %s, want the fair learner interval", plan.Interval)
	}
}
