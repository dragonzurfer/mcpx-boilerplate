package stores

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestListFunnelEventWeightsSeedsDefaults(t *testing.T) {
	store := newFunnelTestStore(t)

	rows, err := store.ListFunnelEventWeights()
	if err != nil {
		t.Fatalf("expected weights to load, got %v", err)
	}

	expected := map[string]int{
		"post_open":     1,
		"scroll_depth":  2,
		"time_on_page":  2,
		"post_complete": 5,
		"promo_click":   10,
		"paywall_hit":   6,
	}

	if len(rows) != len(expected) {
		t.Fatalf("expected %d weights, got %d", len(expected), len(rows))
	}

	seen := map[string]bool{}
	for _, row := range rows {
		seen[row.EventType] = true
		if expected[row.EventType] != row.Weight {
			t.Fatalf("expected weight %d for %s, got %d", expected[row.EventType], row.EventType, row.Weight)
		}
		if !row.Enabled {
			t.Fatalf("expected %s to be enabled", row.EventType)
		}
	}

	for key := range expected {
		if !seen[key] {
			t.Fatalf("expected weight for %s", key)
		}
	}

	rows, err = store.ListFunnelEventWeights()
	if err != nil {
		t.Fatalf("expected weights to load, got %v", err)
	}
	if len(rows) != len(expected) {
		t.Fatalf("expected %d weights after reload, got %d", len(expected), len(rows))
	}
}

func TestListFunnelStageThresholdsSeedsDefaults(t *testing.T) {
	store := newFunnelTestStore(t)

	rows, err := store.ListFunnelStageThresholds()
	if err != nil {
		t.Fatalf("expected stages to load, got %v", err)
	}

	type stageExpectation struct {
		min int
		max int
	}

	expected := map[string]stageExpectation{
		FunnelStageNew:     {min: 0, max: 5},
		FunnelStageCasual:  {min: 6, max: 15},
		FunnelStageEngaged: {min: 16, max: 35},
		FunnelStageHot:     {min: 36, max: 60},
	}

	if len(rows) != len(expected) {
		t.Fatalf("expected %d stages, got %d", len(expected), len(rows))
	}

	seen := map[string]bool{}
	for _, row := range rows {
		expectation, ok := expected[row.Stage]
		if !ok {
			t.Fatalf("unexpected stage %s", row.Stage)
		}
		seen[row.Stage] = true
		if row.MinScore != expectation.min {
			t.Fatalf("expected min score %d for %s, got %d", expectation.min, row.Stage, row.MinScore)
		}
		if row.MaxScore == nil || *row.MaxScore != expectation.max {
			t.Fatalf("expected max score %d for %s, got %v", expectation.max, row.Stage, row.MaxScore)
		}
		if !row.Enabled {
			t.Fatalf("expected %s to be enabled", row.Stage)
		}
	}

	for key := range expected {
		if !seen[key] {
			t.Fatalf("expected stage %s", key)
		}
	}
}

func newFunnelTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&FunnelEventWeightModel{}, &FunnelStageThresholdModel{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return &Store{db: db}
}
