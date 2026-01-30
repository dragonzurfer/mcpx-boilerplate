package stores

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRollupPostAnalyticsAggregatesDailyMetrics(t *testing.T) {
	store := newPostAnalyticsTestStore(t)
	day := time.Date(2026, 1, 27, 0, 0, 0, 0, time.UTC)
	dayStart := day.Add(9 * time.Hour)

	postID := uint(101)
	userID := uint(9)

	events := []EventInput{
		{EventType: "post_open", EntityType: "POST", EntityID: &postID, Metadata: map[string]interface{}{}, CreatedAt: timePtr(dayStart)},
		{EventType: "scroll_depth", EntityType: "POST", EntityID: &postID, Metadata: map[string]interface{}{"pct": 75}, CreatedAt: timePtr(dayStart.Add(1 * time.Hour))},
		{EventType: "time_on_page", EntityType: "POST", EntityID: &postID, Metadata: map[string]interface{}{"sec": 45}, CreatedAt: timePtr(dayStart.Add(2 * time.Hour))},
		{EventType: "post_complete", EntityType: "POST", EntityID: &postID, Metadata: map[string]interface{}{}, CreatedAt: timePtr(dayStart.Add(3 * time.Hour))},
	}

	if err := store.InsertEvents(EventBatchInput{UserID: &userID, Events: events, Now: dayStart}); err != nil {
		t.Fatalf("insert events failed: %v", err)
	}

	promoImpression := PromoImpressionModel{
		PromoID:   5,
		VariantID: 7,
		UserID:    &userID,
		PostID:    &postID,
		CreatedAt: dayStart.Add(4 * time.Hour),
	}
	if err := store.DB().Create(&promoImpression).Error; err != nil {
		t.Fatalf("seed promo impression failed: %v", err)
	}

	promoClick := PromoClickModel{
		PromoID:   5,
		VariantID: 7,
		UserID:    &userID,
		PostID:    &postID,
		CreatedAt: dayStart.Add(5 * time.Hour),
	}
	if err := store.DB().Create(&promoClick).Error; err != nil {
		t.Fatalf("seed promo click failed: %v", err)
	}

	if err := store.RollupPostAnalytics(PostAnalyticsRollupInput{Day: day}); err != nil {
		t.Fatalf("rollup failed: %v", err)
	}

	var metrics PostDailyMetricModel
	if err := store.DB().Where("post_id = ? AND day_date = ?", postID, day).First(&metrics).Error; err != nil {
		t.Fatalf("load metrics failed: %v", err)
	}
	if metrics.TotalViews != 1 {
		t.Fatalf("expected total views 1, got %d", metrics.TotalViews)
	}
	if metrics.UniqueImpressions != 1 {
		t.Fatalf("expected unique impressions 1, got %d", metrics.UniqueImpressions)
	}
	if metrics.Scroll75 != 1 {
		t.Fatalf("expected scroll75 1, got %d", metrics.Scroll75)
	}
	if metrics.Time45 != 1 {
		t.Fatalf("expected time45 1, got %d", metrics.Time45)
	}
	if metrics.Completes != 1 {
		t.Fatalf("expected completes 1, got %d", metrics.Completes)
	}
	if metrics.PromoImpressions != 1 {
		t.Fatalf("expected promo impressions 1, got %d", metrics.PromoImpressions)
	}
	if metrics.PromoClicks != 1 {
		t.Fatalf("expected promo clicks 1, got %d", metrics.PromoClicks)
	}

	var promoMetrics PostPromoDailyMetricModel
	if err := store.DB().Where("post_id = ? AND promo_id = ? AND variant_id = ? AND day_date = ?", postID, 5, 7, day).First(&promoMetrics).Error; err != nil {
		t.Fatalf("load promo metrics failed: %v", err)
	}
	if promoMetrics.Impressions != 1 {
		t.Fatalf("expected promo impressions 1, got %d", promoMetrics.Impressions)
	}
	if promoMetrics.Clicks != 1 {
		t.Fatalf("expected promo clicks 1, got %d", promoMetrics.Clicks)
	}
}

func newPostAnalyticsTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&EventModel{},
		&PostImpressionModel{},
		&PostDailyMetricModel{},
		&PostPromoDailyMetricModel{},
		&PromoImpressionModel{},
		&PromoClickModel{},
	); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return NewStoreWithDB(db)
}

func timePtr(value time.Time) *time.Time {
	return &value
}
