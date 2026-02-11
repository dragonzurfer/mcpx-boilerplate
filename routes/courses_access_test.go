package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/services"
	"github.com/mcpx/boilerplate/stores"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCourseDetailRequiresSignedInUser(t *testing.T) {
	store := newCourseAccessTestStore(t)
	seedCourseGraphForAccessTests(t, store)

	handler := CoursesHandler{Service: &services.ContentService{Store: store}}
	router := gin.New()
	api := router.Group("/api")
	handler.Register(api)

	req := httptest.NewRequest(http.MethodGet, "/api/courses/go-101", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for anonymous course detail, got %d", rec.Code)
	}
}

func TestCourseDetailShowsLessonLocksWithoutEntitlement(t *testing.T) {
	store := newCourseAccessTestStore(t)
	seedCourseGraphForAccessTests(t, store)
	user := &stores.UserModel{ID: 77, Email: "reader@example.com", Role: stores.UserRoleUser}

	handler := CoursesHandler{Service: &services.ContentService{Store: store}}
	router := gin.New()
	api := router.Group("/api")
	api.Use(withTestUser(user))
	handler.Register(api)

	req := httptest.NewRequest(http.MethodGet, "/api/courses/go-101", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response struct {
		EntitlementActive bool `json:"entitlement_active"`
		Course            struct {
			Modules []struct {
				Lessons []struct {
					Slug     string `json:"slug"`
					IsFree   bool   `json:"is_free"`
					IsLocked bool   `json:"is_locked"`
				} `json:"lessons"`
			} `json:"modules"`
		} `json:"course"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}

	if response.EntitlementActive {
		t.Fatalf("expected no active entitlement")
	}
	if len(response.Course.Modules) != 1 {
		t.Fatalf("expected one module")
	}
	if len(response.Course.Modules[0].Lessons) != 2 {
		t.Fatalf("expected two lessons")
	}

	freeLesson := response.Course.Modules[0].Lessons[0]
	paidLesson := response.Course.Modules[0].Lessons[1]
	if !freeLesson.IsFree || freeLesson.IsLocked {
		t.Fatalf("expected free lesson to be unlocked")
	}
	if paidLesson.IsFree || !paidLesson.IsLocked {
		t.Fatalf("expected paid lesson to be locked")
	}
}

func TestCourseLessonReturnsLockedPayloadWhenUserIsNotEntitled(t *testing.T) {
	store := newCourseAccessTestStore(t)
	seedCourseGraphForAccessTests(t, store)
	user := &stores.UserModel{ID: 12, Email: "user@example.com", Role: stores.UserRoleUser}

	handler := CoursesHandler{Service: &services.ContentService{Store: store}}
	router := gin.New()
	api := router.Group("/api")
	api.Use(withTestUser(user))
	handler.Register(api)

	req := httptest.NewRequest(http.MethodGet, "/api/courses/go-101/lessons/go-projects", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 locked payload, got %d", rec.Code)
	}

	var response struct {
		IsLocked bool `json:"is_locked"`
		Gate     struct {
			Type string `json:"type"`
		} `json:"gate"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if !response.IsLocked {
		t.Fatalf("expected locked lesson payload")
	}
	if response.Gate.Type != services.GateTypePaywall {
		t.Fatalf("expected paywall gate")
	}
}

func TestCourseLessonReturnsRenderedContentForEntitledUser(t *testing.T) {
	store := newCourseAccessTestStore(t)
	seedCourseGraphForAccessTests(t, store)
	user := &stores.UserModel{ID: 45, Email: "paid@example.com", Role: stores.UserRoleUser}

	activeEntitlement := stores.EntitlementModel{
		UserID:    user.ID,
		PlanCode:  "PRO_MONTHLY",
		Status:    stores.EntitlementStatusActive,
		StartAt:   time.Now().UTC().Add(-24 * time.Hour),
		EndAt:     time.Now().UTC().Add(24 * time.Hour),
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := store.DB().Create(&activeEntitlement).Error; err != nil {
		t.Fatalf("seed entitlement failed: %v", err)
	}

	handler := CoursesHandler{Service: &services.ContentService{Store: store}}
	router := gin.New()
	api := router.Group("/api")
	api.Use(withTestUser(user))
	handler.Register(api)

	req := httptest.NewRequest(http.MethodGet, "/api/courses/go-101/lessons/go-projects", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response struct {
		IsLocked bool   `json:"is_locked"`
		HTML     string `json:"html"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if response.IsLocked {
		t.Fatalf("expected lesson to be unlocked")
	}
	if response.HTML == "" {
		t.Fatalf("expected rendered lesson html")
	}
}

func seedCourseGraphForAccessTests(t *testing.T, store *stores.Store) {
	t.Helper()
	now := time.Now().UTC()
	course := stores.CourseModel{
		Slug:         "go-101",
		Title:        "Go 101",
		Description:  "Go from fundamentals to shipping projects.",
		Status:       stores.CourseStatusPublished,
		AccessLevel:  stores.AccessLevelPublic,
		PublishedAt:  &now,
		CreatedAt:    now,
		UpdatedAt:    now,
		MetadataJSON: stores.SerializeCourseMetadata(stores.CourseMetadata{Difficulty: "Beginner"}),
		ThumbnailURL: "https://cdn.example.com/go-101.png",
	}
	if err := store.DB().Create(&course).Error; err != nil {
		t.Fatalf("seed course failed: %v", err)
	}

	module := stores.CourseModuleModel{
		CourseID:  course.ID,
		Title:     "Foundations",
		Position:  1,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := store.DB().Create(&module).Error; err != nil {
		t.Fatalf("seed module failed: %v", err)
	}

	freeLesson := stores.CourseLessonModel{
		CourseID:     course.ID,
		ModuleID:     module.ID,
		Title:        "Getting Started",
		Slug:         "getting-started",
		BodyMarkdown: "# Free lesson",
		Position:     1,
		Status:       stores.CourseStatusPublished,
		IsFree:       true,
		PublishedAt:  &now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := store.DB().Create(&freeLesson).Error; err != nil {
		t.Fatalf("seed free lesson failed: %v", err)
	}

	paidLesson := stores.CourseLessonModel{
		CourseID:     course.ID,
		ModuleID:     module.ID,
		Title:        "Go Projects",
		Slug:         "go-projects",
		BodyMarkdown: "# Paid lesson",
		Position:     2,
		Status:       stores.CourseStatusPublished,
		IsFree:       false,
		PublishedAt:  &now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := store.DB().Create(&paidLesson).Error; err != nil {
		t.Fatalf("seed paid lesson failed: %v", err)
	}
}

func newCourseAccessTestStore(t *testing.T) *stores.Store {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&stores.CourseModel{},
		&stores.CourseModuleModel{},
		&stores.CourseLessonModel{},
		&stores.EntitlementModel{},
	); err != nil {
		t.Fatalf("failed to migrate sqlite models: %v", err)
	}
	return stores.NewStoreWithDB(db)
}
