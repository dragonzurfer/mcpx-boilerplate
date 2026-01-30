package services

import (
	"testing"
	"time"
)

func TestComputeFunnelScoreHonorsEntitlement(t *testing.T) {
	now := time.Date(2024, 10, 24, 12, 0, 0, 0, time.UTC)
	input := FunnelScoreInput{
		Now:               now,
		EntitlementStatus: EntitlementActive,
		StageThresholds: []StageThreshold{
			{Stage: "NEW", MinScore: 0, MaxScore: intPtrTest(5), Enabled: true},
		},
	}

	output := ComputeFunnelScore(input)
	if output.Stage != "PAID_ACTIVE" {
		t.Fatalf("expected paid active stage, got %q", output.Stage)
	}

	input.EntitlementStatus = EntitlementExpired
	output = ComputeFunnelScore(input)
	if output.Stage != "PAID_EXPIRED" {
		t.Fatalf("expected paid expired stage, got %q", output.Stage)
	}
}

func TestComputeFunnelScoreUsesThresholds(t *testing.T) {
	now := time.Date(2024, 10, 24, 12, 0, 0, 0, time.UTC)
	input := FunnelScoreInput{
		Now: now,
		Events: []FunnelEventInput{
			{EventType: "post_open", CreatedAt: now},
			{EventType: "post_open", CreatedAt: now},
			{EventType: "post_complete", CreatedAt: now},
		},
		Weights: map[string]int{
			"post_open":     1,
			"post_complete": 5,
		},
		StageThresholds: []StageThreshold{
			{Stage: "NEW", MinScore: 0, MaxScore: intPtrTest(3), Enabled: true},
			{Stage: "ENGAGED", MinScore: 4, MaxScore: intPtrTest(10), Enabled: true},
		},
	}

	output := ComputeFunnelScore(input)
	if output.Score != 7 {
		t.Fatalf("expected score 7, got %d", output.Score)
	}
	if output.Stage != "ENGAGED" {
		t.Fatalf("expected engaged stage, got %q", output.Stage)
	}
}

func TestComputeFunnelScoreDetectsDormant(t *testing.T) {
	now := time.Date(2024, 10, 24, 12, 0, 0, 0, time.UTC)
	lastActive := now.AddDate(0, 0, -30)
	input := FunnelScoreInput{
		Now:                   now,
		DormantDaysThreshold:  21,
		LastActiveAtOverride:  &lastActive,
		StageThresholds: []StageThreshold{
			{Stage: "NEW", MinScore: 0, MaxScore: intPtrTest(5), Enabled: true},
		},
	}

	output := ComputeFunnelScore(input)
	if output.Stage != "DORMANT" {
		t.Fatalf("expected dormant stage, got %q", output.Stage)
	}
}

func intPtrTest(value int) *int {
	return &value
}
