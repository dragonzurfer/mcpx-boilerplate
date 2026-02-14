package stores

import (
	"testing"
	"time"
)

type realtimePostSeed struct {
	PostID    uint
	PromoID   uint
	VariantID uint
	Day       time.Time
}

func TestPostAnalyticsSummaryIncludesRealtimeMetrics(t *testing.T) {
	store := newPostAnalyticsTestStore(t)
	day := normalizeDay(time.Now().UTC())

	seed := seedRealtimePostAnalytics(t, store, day)

	summary, err := store.GetPostAnalyticsSummary(PostAnalyticsSummaryInput{PostID: seed.PostID, From: day, To: day})
	if err != nil {
		t.Fatalf("load summary failed: %v", err)
	}
	if summary.TotalViews != 1 {
		t.Fatalf("expected total views 1, got %d", summary.TotalViews)
	}
	if summary.UniqueImpressions != 1 {
		t.Fatalf("expected unique impressions 1, got %d", summary.UniqueImpressions)
	}
	if summary.Scroll50 != 1 {
		t.Fatalf("expected scroll50 1, got %d", summary.Scroll50)
	}
	if summary.Time45 != 1 {
		t.Fatalf("expected time45 1, got %d", summary.Time45)
	}
	if summary.Completes != 1 {
		t.Fatalf("expected completes 1, got %d", summary.Completes)
	}
	if summary.PromoImpressions != 1 {
		t.Fatalf("expected promo impressions 1, got %d", summary.PromoImpressions)
	}
	if summary.PromoClicks != 1 {
		t.Fatalf("expected promo clicks 1, got %d", summary.PromoClicks)
	}
}

func TestPostAnalyticsDaysIncludesRealtimeMetrics(t *testing.T) {
	store := newPostAnalyticsTestStore(t)
	day := normalizeDay(time.Now().UTC())

	seed := seedRealtimePostAnalytics(t, store, day)

	rows, err := store.ListPostAnalyticsDays(PostAnalyticsDaysInput{PostID: seed.PostID, From: day, To: day})
	if err != nil {
		t.Fatalf("load days failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 day row, got %d", len(rows))
	}
	if !rows[0].DayDate.Equal(day) {
		t.Fatalf("expected day %s, got %s", day.Format("2006-01-02"), rows[0].DayDate.Format("2006-01-02"))
	}
	if rows[0].TotalViews != 1 {
		t.Fatalf("expected total views 1, got %d", rows[0].TotalViews)
	}
	if rows[0].UniqueImpressions != 1 {
		t.Fatalf("expected unique impressions 1, got %d", rows[0].UniqueImpressions)
	}
}

func TestPostPromoAnalyticsIncludesRealtimeMetrics(t *testing.T) {
	store := newPostAnalyticsTestStore(t)
	day := normalizeDay(time.Now().UTC())

	seed := seedRealtimePostAnalytics(t, store, day)

	rows, err := store.ListPostPromoAnalytics(PostPromoAnalyticsInput{PostID: seed.PostID, From: day, To: day})
	if err != nil {
		t.Fatalf("load promos failed: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 promo row, got %d", len(rows))
	}
	if rows[0].PromoID != seed.PromoID {
		t.Fatalf("expected promo %d, got %d", seed.PromoID, rows[0].PromoID)
	}
	if rows[0].VariantID != seed.VariantID {
		t.Fatalf("expected variant %d, got %d", seed.VariantID, rows[0].VariantID)
	}
	if rows[0].Impressions != 1 {
		t.Fatalf("expected impressions 1, got %d", rows[0].Impressions)
	}
	if rows[0].Clicks != 1 {
		t.Fatalf("expected clicks 1, got %d", rows[0].Clicks)
	}
}

func seedRealtimePostAnalytics(t *testing.T, store *Store, day time.Time) realtimePostSeed {
	t.Helper()

	postID := uint(901)
	userID := uint(42)
	promoID := uint(11)
	variantID := uint(3)
	dayStart := day.Add(3 * time.Hour)

	events := []EventInput{
		{EventType: "post_open", EntityType: "POST", EntityID: &postID, Metadata: map[string]interface{}{}, CreatedAt: timePtr(dayStart)},
		{EventType: "scroll_depth", EntityType: "POST", EntityID: &postID, Metadata: map[string]interface{}{"pct": 50}, CreatedAt: timePtr(dayStart.Add(10 * time.Minute))},
		{EventType: "time_on_page", EntityType: "POST", EntityID: &postID, Metadata: map[string]interface{}{"sec": 45}, CreatedAt: timePtr(dayStart.Add(20 * time.Minute))},
		{EventType: "post_complete", EntityType: "POST", EntityID: &postID, Metadata: map[string]interface{}{}, CreatedAt: timePtr(dayStart.Add(30 * time.Minute))},
	}

	err := store.InsertEvents(EventBatchInput{UserID: &userID, Events: events, Now: dayStart})
	if err != nil {
		t.Fatalf("insert events failed: %v", err)
	}

	promoImpression := PromoImpressionModel{
		PromoID:   promoID,
		VariantID: variantID,
		UserID:    &userID,
		PostID:    &postID,
		CreatedAt: dayStart.Add(40 * time.Minute),
	}
	err = store.DB().Create(&promoImpression).Error
	if err != nil {
		t.Fatalf("seed promo impression failed: %v", err)
	}

	promoClick := PromoClickModel{
		PromoID:   promoID,
		VariantID: variantID,
		UserID:    &userID,
		PostID:    &postID,
		CreatedAt: dayStart.Add(50 * time.Minute),
	}
	err = store.DB().Create(&promoClick).Error
	if err != nil {
		t.Fatalf("seed promo click failed: %v", err)
	}

	return realtimePostSeed{PostID: postID, PromoID: promoID, VariantID: variantID, Day: day}
}
