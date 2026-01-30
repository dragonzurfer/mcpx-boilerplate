package services

import (
	"testing"
	"time"

	"github.com/mcpx/boilerplate/stores"
)

func TestComputeEntitlementWindowExtendsActive(t *testing.T) {
	now := time.Date(2024, 10, 24, 12, 0, 0, 0, time.UTC)
	currentStart := now.AddDate(0, 0, -10)
	currentEnd := now.AddDate(0, 0, 20)
	current := stores.EntitlementModel{
		Status:  stores.EntitlementStatusActive,
		StartAt: currentStart,
		EndAt:   currentEnd,
	}

	output := ComputeEntitlementWindow(EntitlementWindowInput{
		Now:              now,
		Current:          &current,
		DurationDays:     30,
	})

	expectedEnd := currentEnd.AddDate(0, 0, 30)
	if !output.EndAt.Equal(expectedEnd) {
		t.Fatalf("expected end %v, got %v", expectedEnd, output.EndAt)
	}
	if !output.StartAt.Equal(currentStart) {
		t.Fatalf("expected start to remain, got %v", output.StartAt)
	}
}

func TestComputeEntitlementWindowResetsExpired(t *testing.T) {
	now := time.Date(2024, 10, 24, 12, 0, 0, 0, time.UTC)
	current := stores.EntitlementModel{
		Status:  stores.EntitlementStatusExpired,
		StartAt: now.AddDate(0, 0, -40),
		EndAt:   now.AddDate(0, 0, -10),
	}

	output := ComputeEntitlementWindow(EntitlementWindowInput{
		Now:          now,
		Current:      &current,
		DurationDays: 30,
	})

	expectedEnd := now.AddDate(0, 0, 30)
	if !output.StartAt.Equal(now) {
		t.Fatalf("expected start at now, got %v", output.StartAt)
	}
	if !output.EndAt.Equal(expectedEnd) {
		t.Fatalf("expected end %v, got %v", expectedEnd, output.EndAt)
	}
}
