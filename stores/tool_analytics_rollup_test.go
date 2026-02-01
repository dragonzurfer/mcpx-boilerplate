package stores

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRollupToolAnalyticsAggregatesEvents(t *testing.T) {
	store := newToolAnalyticsTestStore(t)
	day := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	toolID := uint(12)
	userID := uint(5)

	events := []ToolEventModel{
		{ToolID: toolID, UserID: userID, EventName: "session_start", CreatedAt: day.Add(2 * time.Hour)},
		{ToolID: toolID, UserID: userID, EventName: "resume_upload", CreatedAt: day.Add(3 * time.Hour)},
		{ToolID: toolID, UserID: userID, EventName: "response", Metadata: `{"used_audio":true}`, CreatedAt: day.Add(4 * time.Hour)},
		{ToolID: toolID, UserID: userID, EventName: "response", Metadata: `{"used_audio":false}`, CreatedAt: day.Add(5 * time.Hour)},
		{ToolID: toolID, UserID: userID, EventName: "flow_complete", CreatedAt: day.Add(6 * time.Hour)},
	}
	if err := store.DB().Create(&events).Error; err != nil {
		t.Fatalf("seed events failed: %v", err)
	}

	if err := store.RollupToolAnalytics(ToolAnalyticsRollupInput{Day: day}); err != nil {
		t.Fatalf("rollup failed: %v", err)
	}

	var responseMetric ToolDailyMetricModel
	if err := store.DB().Where("tool_id = ? AND day_date = ? AND event_name = ?", toolID, day, "response").First(&responseMetric).Error; err != nil {
		t.Fatalf("load response metric failed: %v", err)
	}
	if responseMetric.TotalCount != 2 {
		t.Fatalf("expected response count 2, got %d", responseMetric.TotalCount)
	}
	if responseMetric.AudioCount != 1 {
		t.Fatalf("expected audio count 1, got %d", responseMetric.AudioCount)
	}

	var sessionMetric ToolDailyMetricModel
	if err := store.DB().Where("tool_id = ? AND day_date = ? AND event_name = ?", toolID, day, "session_start").First(&sessionMetric).Error; err != nil {
		t.Fatalf("load session metric failed: %v", err)
	}
	if sessionMetric.TotalCount != 1 {
		t.Fatalf("expected session count 1, got %d", sessionMetric.TotalCount)
	}
}

func newToolAnalyticsTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&ToolEventModel{}, &ToolDailyMetricModel{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return NewStoreWithDB(db)
}
