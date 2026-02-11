package routes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/stores"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAdminCoursesCreateStoresStructuredMetadata(t *testing.T) {
	store := newAdminCoursesTestStore(t)
	handler := AdminCoursesHandler{Store: store}

	router := gin.New()
	api := router.Group("/api/admin")
	handler.Register(api)

	payload := map[string]interface{}{
		"slug":          "go-fundamentals",
		"title":         "Go Fundamentals",
		"description":   "Build backend skills with Go and Gin.",
		"thumbnail_url": "https://cdn.example.com/go.png",
		"status":        stores.CourseStatusPublished,
		"metadata": map[string]interface{}{
			"target_audience":    []string{"Backend developers", "Career switchers"},
			"tags":               []string{"Beginner", "Go"},
			"difficulty":         "Beginner",
			"estimated_duration": "6 hours",
			"skills_covered":     []string{"Go syntax", "Gin routing"},
			"highlights":         []string{"Projects", "Checklists"},
		},
	}

	rec := performAdminCourseRequest(t, router, http.MethodPost, "/api/admin/courses", payload)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var response struct {
		Course struct {
			ID           uint                  `json:"id"`
			Slug         string                `json:"slug"`
			ThumbnailURL string                `json:"thumbnail_url"`
			Metadata     stores.CourseMetadata `json:"metadata"`
		} `json:"course"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}

	if response.Course.Slug != "go-fundamentals" {
		t.Fatalf("expected slug go-fundamentals, got %q", response.Course.Slug)
	}
	if response.Course.ThumbnailURL != "https://cdn.example.com/go.png" {
		t.Fatalf("expected thumbnail in response")
	}
	if response.Course.Metadata.Difficulty != "Beginner" {
		t.Fatalf("expected structured metadata in response")
	}

	storedCourse := stores.CourseModel{}
	if err := store.DB().Where("id = ?", response.Course.ID).First(&storedCourse).Error; err != nil {
		t.Fatalf("expected created course in db: %v", err)
	}

	storedMetadata := stores.ParseCourseMetadata(storedCourse.MetadataJSON)
	if storedMetadata.Difficulty != "Beginner" {
		t.Fatalf("expected metadata json to persist difficulty")
	}
	if len(storedMetadata.TargetAudience) != 2 {
		t.Fatalf("expected metadata json to persist target audience")
	}
}

func TestAdminCoursesModuleAndLessonReorder(t *testing.T) {
	store := newAdminCoursesTestStore(t)
	handler := AdminCoursesHandler{Store: store}

	router := gin.New()
	api := router.Group("/api/admin")
	handler.Register(api)

	createPayload := map[string]interface{}{
		"slug":        "system-design",
		"title":       "System Design",
		"description": "Learn architecture foundations.",
		"status":      stores.CourseStatusPublished,
		"metadata": map[string]interface{}{
			"difficulty": "Intermediate",
		},
	}

	createRec := performAdminCourseRequest(t, router, http.MethodPost, "/api/admin/courses", createPayload)
	if createRec.Code != http.StatusOK {
		t.Fatalf("expected course create 200, got %d", createRec.Code)
	}

	var createResp struct {
		Course struct {
			ID uint `json:"id"`
		} `json:"course"`
	}
	if err := json.Unmarshal(createRec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("decode create response failed: %v", err)
	}

	courseID := createResp.Course.ID
	moduleOneID := createModuleViaAPI(t, router, courseID, "Module 1")
	moduleTwoID := createModuleViaAPI(t, router, courseID, "Module 2")

	reorderModulesPayload := map[string]interface{}{"module_ids": []uint{moduleTwoID, moduleOneID}}
	reorderModulesPath := "/api/admin/courses/" + strconv.FormatUint(uint64(courseID), 10) + "/modules/reorder"
	reorderModuleRec := performAdminCourseRequest(t, router, http.MethodPut, reorderModulesPath, reorderModulesPayload)
	if reorderModuleRec.Code != http.StatusOK {
		t.Fatalf("expected module reorder 200, got %d", reorderModuleRec.Code)
	}

	lessonOneID := createLessonViaAPI(t, router, courseID, moduleTwoID, "Lesson A", "lesson-a", false)
	lessonTwoID := createLessonViaAPI(t, router, courseID, moduleTwoID, "Lesson B", "lesson-b", true)

	reorderLessonsPayload := map[string]interface{}{"lesson_ids": []uint{lessonTwoID, lessonOneID}}
	reorderLessonsPath := "/api/admin/courses/" + strconv.FormatUint(uint64(courseID), 10) + "/modules/" + strconv.FormatUint(uint64(moduleTwoID), 10) + "/lessons/reorder"
	reorderLessonRec := performAdminCourseRequest(t, router, http.MethodPut, reorderLessonsPath, reorderLessonsPayload)
	if reorderLessonRec.Code != http.StatusOK {
		t.Fatalf("expected lesson reorder 200, got %d", reorderLessonRec.Code)
	}

	detailPath := "/api/admin/courses/" + strconv.FormatUint(uint64(courseID), 10)
	detailRec := performAdminCourseRequest(t, router, http.MethodGet, detailPath, nil)
	if detailRec.Code != http.StatusOK {
		t.Fatalf("expected detail 200, got %d", detailRec.Code)
	}

	var detail struct {
		Course struct {
			Modules []struct {
				ID      uint `json:"id"`
				Lessons []struct {
					ID     uint   `json:"id"`
					Title  string `json:"title"`
					IsFree bool   `json:"is_free"`
				} `json:"lessons"`
			} `json:"modules"`
		} `json:"course"`
	}
	if err := json.Unmarshal(detailRec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail failed: %v", err)
	}

	if len(detail.Course.Modules) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(detail.Course.Modules))
	}
	if detail.Course.Modules[0].ID != moduleTwoID {
		t.Fatalf("expected reordered module in first position")
	}
	if len(detail.Course.Modules[0].Lessons) != 2 {
		t.Fatalf("expected 2 lessons in reordered module")
	}
	if detail.Course.Modules[0].Lessons[0].ID != lessonTwoID {
		t.Fatalf("expected reordered lesson in first position")
	}
	if !detail.Course.Modules[0].Lessons[0].IsFree {
		t.Fatalf("expected lesson free flag to round-trip")
	}
}

func createModuleViaAPI(t *testing.T, router *gin.Engine, courseID uint, title string) uint {
	t.Helper()
	path := "/api/admin/courses/" + strconv.FormatUint(uint64(courseID), 10) + "/modules"
	rec := performAdminCourseRequest(t, router, http.MethodPost, path, map[string]interface{}{"title": title})
	if rec.Code != http.StatusOK {
		t.Fatalf("create module failed: %d %s", rec.Code, rec.Body.String())
	}

	var response struct {
		Module struct {
			ID uint `json:"id"`
		} `json:"module"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode module response failed: %v", err)
	}
	return response.Module.ID
}

func createLessonViaAPI(t *testing.T, router *gin.Engine, courseID uint, moduleID uint, title, slug string, isFree bool) uint {
	t.Helper()
	path := "/api/admin/courses/" + strconv.FormatUint(uint64(courseID), 10) + "/modules/" + strconv.FormatUint(uint64(moduleID), 10) + "/lessons"
	payload := map[string]interface{}{
		"title":         title,
		"slug":          slug,
		"body_markdown": "# Lesson",
		"is_free":       isFree,
		"status":        stores.CourseStatusPublished,
	}
	rec := performAdminCourseRequest(t, router, http.MethodPost, path, payload)
	if rec.Code != http.StatusOK {
		t.Fatalf("create lesson failed: %d %s", rec.Code, rec.Body.String())
	}

	var response struct {
		Lesson struct {
			ID uint `json:"id"`
		} `json:"lesson"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode lesson response failed: %v", err)
	}
	return response.Lesson.ID
}

func performAdminCourseRequest(t *testing.T, router *gin.Engine, method, path string, payload interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload failed: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func newAdminCoursesTestStore(t *testing.T) *stores.Store {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&stores.CourseModel{}, &stores.CourseModuleModel{}, &stores.CourseLessonModel{}); err != nil {
		t.Fatalf("failed to migrate sqlite models: %v", err)
	}
	return stores.NewStoreWithDB(db)
}
