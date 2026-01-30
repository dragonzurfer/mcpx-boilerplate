package payments

import "testing"

func TestLoadFromConfig_DefaultsAndLookup(t *testing.T) {
	cfg := Config{
		Plans: []Plan{
			{
				Code:      "FREE",
				Name:      "Free",
				Type:      "one_time",
				Interval:  "monthly",
				PriceINR:  0,
				Quotas:    nil,
				MostPopular: false,
			},
			{
				Code:        "Pro",
				Name:        "Pro",
				Type:        "subscription",
				Interval:    "monthly",
				PriceINR:    499,
				MostPopular: true,
				Quotas:      map[string]int64{"api_calls": 100},
			},
		},
	}

	mgr := LoadFromConfig(cfg)
	if mgr.Config.Currency != "INR" {
		t.Fatalf("expected default currency INR, got %q", mgr.Config.Currency)
	}

	plan, ok := mgr.PlanByCode("free")
	if !ok {
		t.Fatal("expected free plan to be found")
	}
	if plan.Code != "free" {
		t.Fatalf("expected plan code lowercased, got %q", plan.Code)
	}
	if plan.Quotas == nil {
		t.Fatal("expected quotas to default to empty map")
	}
}

func TestRecommendedPlan(t *testing.T) {
	cfg := Config{
		Plans: []Plan{
			{Code: "monthly", Name: "Monthly", Type: "one_time", Interval: "monthly"},
			{Code: "yearly", Name: "Yearly", Type: "one_time", Interval: "yearly", MostPopular: true},
		},
	}
	mgr := LoadFromConfig(cfg)

	plan, ok := mgr.RecommendedPlan()
	if !ok {
		t.Fatal("expected a recommended plan")
	}
	if plan.Code != "yearly" {
		t.Fatalf("expected yearly as recommended, got %q", plan.Code)
	}
}

func TestEntryPlan(t *testing.T) {
	cfg := Config{
		EntryPlanCode: "monthly",
		Plans: []Plan{
			{Code: "monthly", Name: "Monthly", Type: "one_time", Interval: "monthly"},
			{Code: "yearly", Name: "Yearly", Type: "one_time", Interval: "yearly"},
		},
	}
	mgr := LoadFromConfig(cfg)

	plan, ok := mgr.EntryPlan()
	if !ok {
		t.Fatal("expected entry plan")
	}
	if plan.Code != "monthly" {
		t.Fatalf("expected monthly plan, got %q", plan.Code)
	}
}
