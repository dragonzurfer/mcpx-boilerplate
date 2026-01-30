package routes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/stores"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestPostAnalyticsSummary(t *testing.T) {
	store := newPostAnalyticsRouteStore(t)
	postID := uint(33)
	day1 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	day2 := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	rows := []stores.PostDailyMetricModel{
		{PostID: postID, DayDate: day1, TotalViews: 5, UniqueImpressions: 3, Completes: 2, PromoImpressions: 4, PromoClicks: 1},
		{PostID: postID, DayDate: day2, TotalViews: 7, UniqueImpressions: 5, Completes: 3, PromoImpressions: 2, PromoClicks: 1},
	}
	if err := store.DB().Create(&rows).Error; err != nil {
		t.Fatalf("seed metrics failed: %v", err)
	}

	handler := AdminPostAnalyticsHandler{Store: store}
	router := gin.New()
	api := router.Group("/api/admin")
	handler.Register(api)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/analytics/posts/%d/summary?from=2026-01-01&to=2026-01-02", postID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload struct {
		Totals struct {
			TotalViews        int `json:"total_views"`
			UniqueImpressions int `json:"unique_impressions"`
			Completes         int `json:"completes"`
			PromoImpressions  int `json:"promo_impressions"`
			PromoClicks       int `json:"promo_clicks"`
		} `json:"totals"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if payload.Totals.TotalViews != 12 {
		t.Fatalf("expected total views 12, got %d", payload.Totals.TotalViews)
	}
	if payload.Totals.UniqueImpressions != 8 {
		t.Fatalf("expected unique impressions 8, got %d", payload.Totals.UniqueImpressions)
	}
	if payload.Totals.Completes != 5 {
		t.Fatalf("expected completes 5, got %d", payload.Totals.Completes)
	}
	if payload.Totals.PromoImpressions != 6 {
		t.Fatalf("expected promo impressions 6, got %d", payload.Totals.PromoImpressions)
	}
	if payload.Totals.PromoClicks != 2 {
		t.Fatalf("expected promo clicks 2, got %d", payload.Totals.PromoClicks)
	}
}

func TestPostAnalyticsTimeseries(t *testing.T) {
	store := newPostAnalyticsRouteStore(t)
	postID := uint(77)
	day := time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)
	row := stores.PostDailyMetricModel{PostID: postID, DayDate: day, TotalViews: 2, UniqueImpressions: 1}
	if err := store.DB().Create(&row).Error; err != nil {
		t.Fatalf("seed metrics failed: %v", err)
	}

	handler := AdminPostAnalyticsHandler{Store: store}
	router := gin.New()
	api := router.Group("/api/admin")
	handler.Register(api)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/admin/analytics/posts/%d/timeseries?from=2026-01-01&to=2026-01-05", postID), nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var payload struct {
		Days []stores.PostDailyMetricModel `json:"days"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if len(payload.Days) != 1 {
		t.Fatalf("expected 1 day, got %d", len(payload.Days))
	}
	if payload.Days[0].TotalViews != 2 {
		t.Fatalf("expected total views 2, got %d", payload.Days[0].TotalViews)
	}
}

func newPostAnalyticsRouteStore(t *testing.T) *stores.Store {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&stores.PostDailyMetricModel{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return stores.NewStoreWithDB(db)
}
