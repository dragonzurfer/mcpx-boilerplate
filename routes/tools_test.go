package routes

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/middleware"
	"github.com/mcpx/boilerplate/services"
	"github.com/mcpx/boilerplate/stores"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type fakeToolAI struct {
	mentorOutput     services.GeminiMentorOutput
	chatOutput       services.GeminiChatOutput
	transcribeOutput services.GeminiTranscribeOutput
}

func (f *fakeToolAI) AnalyzeResume(ctx context.Context, input services.GeminiResumeInput) (services.GeminiResumeOutput, error) {
	return services.GeminiResumeOutput{}, nil
}

func (f *fakeToolAI) GenerateMentorResponse(ctx context.Context, input services.GeminiMentorInput) (services.GeminiMentorOutput, error) {
	return f.mentorOutput, nil
}

func (f *fakeToolAI) GenerateChatResponse(ctx context.Context, input services.GeminiChatInput) (services.GeminiChatOutput, error) {
	return f.chatOutput, nil
}

func (f *fakeToolAI) TranscribeAudio(ctx context.Context, input services.GeminiTranscribeInput) (services.GeminiTranscribeOutput, error) {
	return f.transcribeOutput, nil
}

func TestToolsMentorEndpoint(t *testing.T) {
	store := newToolsTestStore(t)
	tool := seedToolForTests(t, store)
	user := &stores.UserModel{ID: 1, Email: "user@test.com"}

	fakeAI := &fakeToolAI{
		mentorOutput: services.GeminiMentorOutput{
			Summary:      []string{"One"},
			Strengths:    []string{"Two"},
			Gaps:         []string{"Three"},
			Plan7D:       []string{"Day1"},
			Plan30D:      []string{"Day30"},
			Resources:    []string{"Link"},
			ResponseText: "Response",
		},
	}

	router := gin.New()
	api := router.Group("/api")
	api.Use(withTestUser(user))

	handler := &ToolsHandler{
		Store:   store,
		Service: &services.ToolService{Store: store},
		AI:      fakeAI,
	}
	handler.Register(api)

	payload := map[string]interface{}{
		"resume_text": "Resume text",
		"analysis": map[string]interface{}{
			"ats_score":           80,
			"readability_summary": "Clear",
			"quick_wins":          []string{"Add metrics"},
			"resume_text":         "Resume text",
		},
		"focus":    "Career Growth",
		"subfocus": "Promotion roadmap",
		"answers": map[string]interface{}{
			"current_role": "Engineer",
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/tools/"+tool.Slug+"/mentor", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	mentor := resp["mentor"].(map[string]interface{})
	if mentor["response_text"].(string) != "Response" {
		t.Fatalf("unexpected mentor response")
	}
}

func TestToolsChatEndpoint(t *testing.T) {
	store := newToolsTestStore(t)
	tool := seedToolForTests(t, store)
	user := &stores.UserModel{ID: 2, Email: "user2@test.com"}

	fakeAI := &fakeToolAI{
		chatOutput: services.GeminiChatOutput{ReplyMarkdown: "Hello"},
	}

	router := gin.New()
	api := router.Group("/api")
	api.Use(withTestUser(user))

	handler := &ToolsHandler{
		Store:   store,
		Service: &services.ToolService{Store: store},
		AI:      fakeAI,
	}
	handler.Register(api)

	payload := map[string]interface{}{
		"resume_text": "Resume text",
		"analysis": map[string]interface{}{
			"ats_score":           80,
			"readability_summary": "Clear",
			"quick_wins":          []string{"Add metrics"},
		},
		"focus":    "Career Growth",
		"subfocus": "Promotion roadmap",
		"answers": map[string]interface{}{
			"current_role": "Engineer",
		},
		"history": []map[string]interface{}{
			{"role": "user", "content": "Hello"},
		},
		"message": "Next step?",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/tools/"+tool.Slug+"/chat", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if resp["reply_markdown"].(string) != "Hello" {
		t.Fatalf("unexpected reply markdown")
	}
}

func TestToolsTranscribeEndpoint(t *testing.T) {
	store := newToolsTestStore(t)
	tool := seedToolForTests(t, store)
	user := &stores.UserModel{ID: 3, Email: "user3@test.com"}

	fakeAI := &fakeToolAI{
		transcribeOutput: services.GeminiTranscribeOutput{Transcript: "Transcript"},
	}

	router := gin.New()
	api := router.Group("/api")
	api.Use(withTestUser(user))

	handler := &ToolsHandler{
		Store:   store,
		Service: &services.ToolService{Store: store},
		AI:      fakeAI,
	}
	handler.Register(api)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "audio.webm")
	if err != nil {
		t.Fatalf("create form file failed: %v", err)
	}
	if _, err := part.Write([]byte("audio-bytes")); err != nil {
		t.Fatalf("write audio failed: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer failed: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/tools/"+tool.Slug+"/transcribe", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response failed: %v", err)
	}
	if resp["transcript"].(string) != "Transcript" {
		t.Fatalf("unexpected transcript")
	}
}

func newToolsTestStore(t *testing.T) *stores.Store {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&stores.ToolModel{},
		&stores.ToolUsageModel{},
		&stores.ToolEventModel{},
		&stores.EntitlementModel{},
	); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return stores.NewStoreWithDB(db)
}

func seedToolForTests(t *testing.T, store *stores.Store) stores.ToolModel {
	t.Helper()
	tool := stores.ToolModel{
		Name:       "Career Copilot",
		Slug:       "career-copilot",
		Category:   "career",
		IsActive:   true,
		IsPaidTool: false,
		ConfigJSON: stores.SerializeToolConfig(stores.ToolConfig{}),
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}
	if err := store.DB().Create(&tool).Error; err != nil {
		t.Fatalf("seed tool failed: %v", err)
	}
	return tool
}

func withTestUser(user *stores.UserModel) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(middleware.ContextUserKey, user)
		c.Next()
	}
}
