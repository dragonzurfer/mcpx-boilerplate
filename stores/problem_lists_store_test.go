package stores

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestListSolvedProblemIDsRequiresAcceptedSubmit(t *testing.T) {
	store := newSolvedProblemTestStore(t)
	now := time.Now().UTC()

	submissions := []SubmissionModel{
		{ID: 1, UserID: 1, ProblemID: 11, Mode: SubmissionModeRun, Status: SubmissionStatusCompleted, QueuedAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: 2, UserID: 1, ProblemID: 12, Mode: SubmissionModeSubmit, Status: SubmissionStatusCompleted, QueuedAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: 3, UserID: 1, ProblemID: 13, Mode: SubmissionModeSubmit, Status: SubmissionStatusCompleted, QueuedAt: now, CreatedAt: now, UpdatedAt: now},
		{ID: 4, UserID: 2, ProblemID: 14, Mode: SubmissionModeSubmit, Status: SubmissionStatusCompleted, QueuedAt: now, CreatedAt: now, UpdatedAt: now},
	}
	if err := store.db.Create(&submissions).Error; err != nil {
		t.Fatalf("failed to seed submissions: %v", err)
	}

	results := []SubmissionResultModel{
		{SubmissionID: 1, OverallJSON: `{"verdict":"AC"}`},
		{SubmissionID: 2, OverallJSON: `{"verdict":"AC"}`},
		{SubmissionID: 3, OverallJSON: `{"verdict":"WRONG_ANSWER"}`},
		{SubmissionID: 4, OverallJSON: `{"verdict":"AC"}`},
	}
	if err := store.db.Create(&results).Error; err != nil {
		t.Fatalf("failed to seed results: %v", err)
	}

	solvedIDs, err := store.ListSolvedProblemIDs(UserSolvedProblemIDsInput{
		UserID:     1,
		ProblemIDs: []uint{11, 12, 13, 14},
	})
	if err != nil {
		t.Fatalf("expected solved IDs, got %v", err)
	}

	if len(solvedIDs) != 1 || solvedIDs[0] != 12 {
		t.Fatalf("expected only problem 12 solved from accepted submit, got %v", solvedIDs)
	}
}

func newSolvedProblemTestStore(t *testing.T) *Store {
	t.Helper()

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&SubmissionModel{}, &SubmissionResultModel{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	return &Store{db: db}
}
