package services

import (
	"testing"
	"time"

	"github.com/mcpx/boilerplate/stores"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestToolResponseCountsWithinFreeLimit(t *testing.T) {
	store := newToolTestStore(t)
	tool := seedTool(t, store)
	service := &ToolService{Store: store}
	userID := uint(1)

	output, err := service.HandleAction(ToolActionInput{
		Tool:   tool,
		UserID: userID,
		Action: "response",
		Now:    time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !output.Allowed {
		t.Fatalf("expected allowed response")
	}
	if output.UsageState.FreeUserResponsesUsed != 1 {
		t.Fatalf("expected responses used 1, got %d", output.UsageState.FreeUserResponsesUsed)
	}
}

func TestToolResponseBlocksAfterLimit(t *testing.T) {
	store := newToolTestStore(t)
	tool := seedTool(t, store)
	userID := uint(2)

	state := stores.ToolUsageState{
		HasUsedFreeFlow:       true,
		FreeUserResponsesUsed: 2,
		FreeAudioInputsUsed:   0,
	}
	usage := stores.ToolUsageModel{
		UserID:         userID,
		ToolID:         tool.ID,
		UsageStateJSON: stores.SerializeToolUsageState(state),
		FirstUsedAt:    time.Now().UTC(),
		LastUsedAt:     time.Now().UTC(),
	}
	if err := store.DB().Create(&usage).Error; err != nil {
		t.Fatalf("seed usage failed: %v", err)
	}

	service := &ToolService{Store: store}
	output, err := service.HandleAction(ToolActionInput{
		Tool:   tool,
		UserID: userID,
		Action: "response",
		Now:    time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if output.Allowed {
		t.Fatalf("expected response to be blocked")
	}
	if output.Reason != "FREE_RESPONSE_LIMIT" {
		t.Fatalf("expected FREE_RESPONSE_LIMIT, got %s", output.Reason)
	}
}

func TestToolAudioBlocksAfterLimit(t *testing.T) {
	store := newToolTestStore(t)
	tool := seedTool(t, store)
	userID := uint(3)

	state := stores.ToolUsageState{
		HasUsedFreeFlow:       false,
		FreeUserResponsesUsed: 1,
		FreeAudioInputsUsed:   1,
	}
	usage := stores.ToolUsageModel{
		UserID:         userID,
		ToolID:         tool.ID,
		UsageStateJSON: stores.SerializeToolUsageState(state),
		FirstUsedAt:    time.Now().UTC(),
		LastUsedAt:     time.Now().UTC(),
	}
	if err := store.DB().Create(&usage).Error; err != nil {
		t.Fatalf("seed usage failed: %v", err)
	}

	service := &ToolService{Store: store}
	output, err := service.HandleAction(ToolActionInput{
		Tool:      tool,
		UserID:    userID,
		Action:    "response",
		UsedAudio: true,
		Now:       time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if output.Allowed {
		t.Fatalf("expected audio response to be blocked")
	}
	if output.Reason != "FREE_AUDIO_LIMIT" {
		t.Fatalf("expected FREE_AUDIO_LIMIT, got %s", output.Reason)
	}
}

func TestResumeUploadBlocksAfterFreeFlow(t *testing.T) {
	store := newToolTestStore(t)
	tool := seedTool(t, store)
	userID := uint(4)

	state := stores.ToolUsageState{
		HasUsedFreeFlow:       true,
		FreeUserResponsesUsed: 2,
		FreeAudioInputsUsed:   1,
	}
	usage := stores.ToolUsageModel{
		UserID:         userID,
		ToolID:         tool.ID,
		UsageStateJSON: stores.SerializeToolUsageState(state),
		FirstUsedAt:    time.Now().UTC(),
		LastUsedAt:     time.Now().UTC(),
	}
	if err := store.DB().Create(&usage).Error; err != nil {
		t.Fatalf("seed usage failed: %v", err)
	}

	service := &ToolService{Store: store}
	output, err := service.HandleAction(ToolActionInput{
		Tool:   tool,
		UserID: userID,
		Action: "resume_upload",
		Now:    time.Now().UTC(),
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if output.Allowed {
		t.Fatalf("expected resume upload to be blocked")
	}
	if output.Reason != "FREE_FLOW_USED" {
		t.Fatalf("expected FREE_FLOW_USED, got %s", output.Reason)
	}
}

func newToolTestStore(t *testing.T) *stores.Store {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&stores.ToolModel{}, &stores.ToolUsageModel{}, &stores.ToolEventModel{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return stores.NewStoreWithDB(db)
}

func seedTool(t *testing.T, store *stores.Store) stores.ToolModel {
	t.Helper()
	cfg := stores.ToolConfig{
		FreeRules: stores.ToolFreeRules{
			MaxFreeResponses:   2,
			MaxFreeAudioInputs: 1,
			SingleUseFreeFlow:  true,
			RequireSignIn:      true,
		},
	}
	tool := stores.ToolModel{
		Name:       "Career Copilot",
		Slug:       "career-copilot",
		Category:   "career",
		IsActive:   true,
		IsPaidTool: false,
		ConfigJSON: stores.SerializeToolConfig(cfg),
	}
	if err := store.DB().Create(&tool).Error; err != nil {
		t.Fatalf("seed tool failed: %v", err)
	}
	return tool
}
