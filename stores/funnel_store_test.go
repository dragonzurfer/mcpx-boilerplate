package stores

import (
	"testing"
	"time"

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
		"post_open":                  1,
		"post_scroll_depth":          2,
		"post_time_on_page":          2,
		"post_complete":              5,
		"post_promo_click":           10,
		"post_paywall_hit":           6,
		"course_open_click":          2,
		"course_open":                2,
		"course_lesson_click":        3,
		"course_time_on_page":        3,
		"course_lesson_time_on_page": 3,
		"practice_problem_open":      2,
		"practice_time_on_page":      3,
		"practice_run_click":         3,
		"practice_submit_click":      5,
		"practice_ai_analyze_click":  4,
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

func TestListFunnelEventWeightsMigratesLegacyRows(t *testing.T) {
	store := newFunnelTestStore(t)

	legacyRows := []FunnelEventWeightModel{
		{EventType: "scroll_depth", Weight: 8, Enabled: true},
		{EventType: "time_on_page", Weight: 9, Enabled: false},
		{EventType: "promo_click", Weight: 11, Enabled: true},
		{EventType: "paywall_hit", Weight: 7, Enabled: true},
	}
	if err := store.db.Create(&legacyRows).Error; err != nil {
		t.Fatalf("failed to seed legacy rows: %v", err)
	}

	rows, err := store.ListFunnelEventWeights()
	if err != nil {
		t.Fatalf("expected weights to load, got %v", err)
	}
	rowByType := map[string]FunnelEventWeightModel{}
	for _, row := range rows {
		rowByType[row.EventType] = row
	}

	if _, exists := rowByType["scroll_depth"]; exists {
		t.Fatalf("expected legacy scroll_depth row to be removed")
	}
	if _, exists := rowByType["time_on_page"]; exists {
		t.Fatalf("expected legacy time_on_page row to be removed")
	}
	if _, exists := rowByType["promo_click"]; exists {
		t.Fatalf("expected legacy promo_click row to be removed")
	}
	if _, exists := rowByType["paywall_hit"]; exists {
		t.Fatalf("expected legacy paywall_hit row to be removed")
	}

	if rowByType["post_scroll_depth"].Weight != 8 {
		t.Fatalf("expected post_scroll_depth weight to be migrated")
	}
	if rowByType["post_time_on_page"].Weight != 9 {
		t.Fatalf("expected post_time_on_page weight to be migrated")
	}
	if rowByType["post_promo_click"].Weight != 11 {
		t.Fatalf("expected post_promo_click weight to be migrated")
	}
	if rowByType["post_paywall_hit"].Weight != 7 {
		t.Fatalf("expected post_paywall_hit weight to be migrated")
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

func TestUpdateFunnelConfigRefreshesCachedValue(t *testing.T) {
	store := newFunnelTestStore(t)
	store.EnableCache(CacheConfig{TTL: time.Minute, MaxEntries: 10})

	initialConfig, err := store.GetFunnelConfig()
	if err != nil {
		t.Fatalf("failed to load initial config: %v", err)
	}
	if initialConfig.ScoringWindowDays != 14 {
		t.Fatalf("expected default scoring window 14, got %d", initialConfig.ScoringWindowDays)
	}

	updatedConfig, err := store.UpdateFunnelConfig(FunnelConfigUpdateInput{
		ScoringWindowDays:    30,
		DecayEnabled:         true,
		DailyDecayFactor:     0.75,
		DormantDaysThreshold: 45,
	})
	if err != nil {
		t.Fatalf("failed to update funnel config: %v", err)
	}

	if updatedConfig.ScoringWindowDays != 30 {
		t.Fatalf("expected updated scoring window 30, got %d", updatedConfig.ScoringWindowDays)
	}

	fetchedConfig, err := store.GetFunnelConfig()
	if err != nil {
		t.Fatalf("failed to fetch funnel config after update: %v", err)
	}

	if fetchedConfig.ScoringWindowDays != 30 ||
		!fetchedConfig.DecayEnabled ||
		fetchedConfig.DailyDecayFactor != 0.75 ||
		fetchedConfig.DormantDaysThreshold != 45 {
		t.Fatalf("expected updated values from cache/DB, got %+v", fetchedConfig)
	}
}

func TestUpsertFunnelEventWeightInvalidatesCache(t *testing.T) {
	store := newFunnelTestStore(t)
	store.EnableCache(CacheConfig{TTL: time.Minute, MaxEntries: 10})

	weights, err := store.ListFunnelEventWeights()
	if err != nil {
		t.Fatalf("failed to list funnel weights: %v", err)
	}
	if len(weights) == 0 {
		t.Fatal("expected default funnel weights")
	}

	targetEventType := weights[0].EventType
	if err := store.UpsertFunnelEventWeight(FunnelEventWeightModel{
		EventType: targetEventType,
		Weight:    99,
		Enabled:   false,
	}); err != nil {
		t.Fatalf("failed to update funnel weight: %v", err)
	}

	updatedWeights, err := store.ListFunnelEventWeights()
	if err != nil {
		t.Fatalf("failed to reload funnel weights: %v", err)
	}

	found := false
	for _, row := range updatedWeights {
		if row.EventType != targetEventType {
			continue
		}
		found = true
		if row.Weight != 99 || row.Enabled {
			t.Fatalf("expected updated weight row, got %+v", row)
		}
	}

	if !found {
		t.Fatalf("expected event type %s after update", targetEventType)
	}
}

func TestUpsertFunnelStageThresholdInvalidatesCache(t *testing.T) {
	store := newFunnelTestStore(t)
	store.EnableCache(CacheConfig{TTL: time.Minute, MaxEntries: 10})

	stages, err := store.ListFunnelStageThresholds()
	if err != nil {
		t.Fatalf("failed to list funnel stages: %v", err)
	}
	if len(stages) == 0 {
		t.Fatal("expected default funnel stages")
	}

	targetStage := stages[0].Stage
	max := 123
	if err := store.UpsertFunnelStageThreshold(FunnelStageThresholdModel{
		Stage:    targetStage,
		MinScore: 50,
		MaxScore: &max,
		Enabled:  false,
	}); err != nil {
		t.Fatalf("failed to update funnel stage: %v", err)
	}

	updatedStages, err := store.ListFunnelStageThresholds()
	if err != nil {
		t.Fatalf("failed to reload funnel stages: %v", err)
	}

	found := false
	for _, row := range updatedStages {
		if row.Stage != targetStage {
			continue
		}
		found = true
		if row.MinScore != 50 || row.MaxScore == nil || *row.MaxScore != 123 || row.Enabled {
			t.Fatalf("expected updated stage row, got %+v", row)
		}
	}

	if !found {
		t.Fatalf("expected stage %s after update", targetStage)
	}
}

func newFunnelTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&FunnelConfigModel{},
		&FunnelEventWeightModel{},
		&FunnelStageThresholdModel{},
	); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return &Store{db: db}
}
