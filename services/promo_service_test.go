package services

import (
	"testing"
	"time"
)

func TestDecidePromoHonorsPromosAllowed(t *testing.T) {
	now := time.Now()
	input := PromoDecisionInput{
		PromosAllowed: true,
		UserStage:     "ENGAGED",
		Slot:          "INLINE",
		Now:           now,
		Promos: []PromoCandidate{
			{
				ID:             1,
				Status:         "ACTIVE",
				Slot:           "INLINE",
				Priority:       10,
				EligibleStages: []string{"ENGAGED"},
				Variants:       []PromoVariant{{ID: 1, Weight: 1}},
			},
		},
	}

	output := DecidePromo(input)
	if output.Promo == nil {
		t.Fatal("expected promo when allowed")
	}

	input.PromosAllowed = false
	output = DecidePromo(input)
	if output.Promo != nil {
		t.Fatal("expected no promo when promos are disabled")
	}
}

func TestDecidePromoChoosesHighestPriority(t *testing.T) {
	now := time.Now()
	input := PromoDecisionInput{
		PromosAllowed: true,
		UserStage:     "ENGAGED",
		Slot:          "INLINE",
		Now:           now,
		RandomSeed:    42,
		Promos: []PromoCandidate{
			{
				ID:             1,
				Status:         "ACTIVE",
				Slot:           "INLINE",
				Priority:       1,
				EligibleStages: []string{"ENGAGED"},
				Variants:       []PromoVariant{{ID: 11, Weight: 1}},
			},
			{
				ID:             2,
				Status:         "ACTIVE",
				Slot:           "INLINE",
				Priority:       5,
				EligibleStages: []string{"ENGAGED"},
				Variants:       []PromoVariant{{ID: 22, Weight: 1}},
			},
		},
	}

	output := DecidePromo(input)
	if output.Promo == nil || output.Promo.ID != 2 {
		t.Fatalf("expected highest priority promo, got %#v", output.Promo)
	}
}

func TestDecidePromoHonorsCooldown(t *testing.T) {
	now := time.Now()
	lastClick := now.Add(-2 * time.Hour)
	input := PromoDecisionInput{
		PromosAllowed: true,
		UserStage:     "HOT",
		Slot:          "BOTTOM_CARD",
		Now:           now,
		RandomSeed:    1,
		Promos: []PromoCandidate{
			{
				ID:             3,
				Status:         "ACTIVE",
				Slot:           "BOTTOM_CARD",
				Priority:       10,
				EligibleStages: []string{"HOT"},
				CooldownHours:  24,
				Variants:       []PromoVariant{{ID: 33, Weight: 1}},
			},
		},
		Metrics: map[uint]PromoMetrics{
			3: {LastClickAt: &lastClick},
		},
	}

	output := DecidePromo(input)
	if output.Promo != nil {
		t.Fatal("expected promo to be blocked by cooldown")
	}
}
