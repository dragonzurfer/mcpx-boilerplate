package routes

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/stores"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAdminToolsUpdatePreservesStages(t *testing.T) {
	store := newAdminToolsTestStore(t)
	initialConfig := stores.ToolConfig{
		FreeRules: stores.ToolFreeRules{
			MaxFreeResponses:   2,
			MaxFreeAudioInputs: 1,
			SingleUseFreeFlow:  true,
			RequireSignIn:      true,
		},
		Tracking: stores.ToolTrackingRules{
			TrackUploads: true,
		},
		Stages: []stores.ToolStageDefinition{
			{Key: "upload", Label: "Resume upload"},
		},
	}
	tool := stores.ToolModel{
		Name:       "Career Copilot",
		Slug:       "career-copilot",
		Category:   "career",
		IsActive:   true,
		IsPaidTool: false,
		ConfigJSON: stores.SerializeToolConfig(initialConfig),
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := store.DB().Create(&tool).Error; err != nil {
		t.Fatalf("seed tool failed: %v", err)
	}

	handler := AdminToolsHandler{Store: store}
	router := gin.New()
	api := router.Group("/api/admin")
	handler.Register(api)

	payload := map[string]interface{}{
		"name":         "Career Copilot",
		"category":     "career",
		"is_active":    true,
		"is_paid_tool": false,
		"config": map[string]interface{}{
			"free_rules": map[string]interface{}{
				"max_free_responses":    3,
				"max_free_audio_inputs": 1,
				"single_use_free_flow":  true,
				"require_sign_in":       true,
			},
			"tracking": map[string]interface{}{
				"track_uploads":   true,
				"track_responses": true,
				"track_audio":     true,
				"track_stages":    true,
			},
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/api/admin/tools/1", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var updated stores.ToolModel
	if err := store.DB().Where("id = ?", tool.ID).First(&updated).Error; err != nil {
		t.Fatalf("load updated tool failed: %v", err)
	}
	cfg := stores.ParseToolConfig(updated.ConfigJSON)
	if len(cfg.Stages) != 1 || cfg.Stages[0].Key != "upload" {
		t.Fatalf("expected stages preserved")
	}
}

func newAdminToolsTestStore(t *testing.T) *stores.Store {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&stores.ToolModel{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return stores.NewStoreWithDB(db)
}
