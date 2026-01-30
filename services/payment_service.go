package services

import (
	"time"

	"github.com/mcpx/boilerplate/stores"
)

type EntitlementWindowInput struct {
	Now          time.Time
	Current      *stores.EntitlementModel
	DurationDays int
}

type EntitlementWindowOutput struct {
	StartAt time.Time
	EndAt   time.Time
}

func ComputeEntitlementWindow(input EntitlementWindowInput) EntitlementWindowOutput {
	startAt := input.Now
	endBase := input.Now

	if input.Current != nil && input.Current.Status == stores.EntitlementStatusActive {
		startAt = input.Current.StartAt
		endBase = maxTime(input.Current.EndAt, input.Now)
	}

	endAt := endBase.AddDate(0, 0, input.DurationDays)
	return EntitlementWindowOutput{StartAt: startAt, EndAt: endAt}
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}
