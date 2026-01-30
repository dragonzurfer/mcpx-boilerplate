package stores

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestInsertEventsRecordsUniquePostImpressionsPerDay(t *testing.T) {
	store := newPostImpressionTestStore(t)
	userID := uint(42)
	now := time.Date(2026, 1, 28, 10, 0, 0, 0, time.UTC)

	events := []EventInput{
		{EventType: "post_open", EntityType: "POST", EntityID: uintPtr(11)},
		{EventType: "post_open", EntityType: "POST", EntityID: uintPtr(11)},
	}

	err := store.InsertEvents(EventBatchInput{
		UserID: &userID,
		AnonID: "anon-1",
		Events: events,
		Now:    now,
	})
	if err != nil {
		t.Fatalf("insert events failed: %v", err)
	}

	var count int64
	if err := store.DB().Model(&PostImpressionModel{}).Count(&count).Error; err != nil {
		t.Fatalf("count impressions failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 impression, got %d", count)
	}

	var row PostImpressionModel
	if err := store.DB().First(&row).Error; err != nil {
		t.Fatalf("load impression failed: %v", err)
	}
	if row.PostID != 11 {
		t.Fatalf("expected post_id 11, got %d", row.PostID)
	}
	if row.UserID == nil || *row.UserID != userID {
		t.Fatalf("expected user_id %d, got %v", userID, row.UserID)
	}
	if row.AnonID != nil {
		t.Fatalf("expected anon_id to be nil when user_id present")
	}
}

func TestMergeAnonPostImpressions(t *testing.T) {
	store := newPostImpressionTestStore(t)
	anonID := "anon-merge"
	userID := uint(7)
	day := time.Date(2026, 1, 27, 0, 0, 0, 0, time.UTC)

	row := PostImpressionModel{
		PostID:  12,
		AnonID:  &anonID,
		DayDate: day,
	}
	if err := store.DB().Create(&row).Error; err != nil {
		t.Fatalf("seed impression failed: %v", err)
	}

	if err := store.MergeAnonPostImpressions(anonID, userID); err != nil {
		t.Fatalf("merge impressions failed: %v", err)
	}

	var updated PostImpressionModel
	if err := store.DB().First(&updated, row.ID).Error; err != nil {
		t.Fatalf("load impression failed: %v", err)
	}
	if updated.UserID == nil || *updated.UserID != userID {
		t.Fatalf("expected user_id %d, got %v", userID, updated.UserID)
	}
}

func newPostImpressionTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&EventModel{}, &PostImpressionModel{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return NewStoreWithDB(db)
}

func uintPtr(value uint) *uint {
	return &value
}
