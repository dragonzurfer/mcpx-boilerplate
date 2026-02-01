package routes

import (
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/middleware"
	"github.com/mcpx/boilerplate/services"
	"github.com/mcpx/boilerplate/stores"
)

type ToolsHandler struct {
	Store   *stores.Store
	Service *services.ToolService
	AI      services.ToolAIService
}

type toolActionRequest struct {
	Action    string                 `json:"action"`
	StageKey  string                 `json:"stage_key"`
	UsedAudio bool                   `json:"used_audio"`
	Metadata  map[string]interface{} `json:"metadata"`
}

func (h *ToolsHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/tools", h.list)
	rg.GET("/tools/:slug", h.get)
	rg.POST("/tools/:slug/resume", h.resume)
	rg.POST("/tools/:slug/mentor", h.mentor)
	rg.POST("/tools/:slug/chat", h.chat)
	rg.POST("/tools/:slug/transcribe", h.transcribe)
	rg.POST("/tools/:slug/action", h.action)
}

func (h *ToolsHandler) list(c *gin.Context) {
	rows, err := h.Store.ListTools(stores.ToolListInput{ActiveOnly: true})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load tools"})
		return
	}

	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		items = append(items, gin.H{
			"id":           row.ID,
			"name":         row.Name,
			"slug":         row.Slug,
			"category":     row.Category,
			"is_active":    row.IsActive,
			"is_paid_tool": row.IsPaidTool,
		})
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *ToolsHandler) get(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}

	tool, err := h.Store.GetToolBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tool not found"})
		return
	}

	payload := gin.H{
		"tool": gin.H{
			"id":           tool.ID,
			"name":         tool.Name,
			"slug":         tool.Slug,
			"category":     tool.Category,
			"is_active":    tool.IsActive,
			"is_paid_tool": tool.IsPaidTool,
		},
		"config": stores.ParseToolConfig(tool.ConfigJSON),
	}

	user, ok := middleware.CurrentUser(c)
	if ok && user != nil {
		entitlementActive := toolEntitlementActive(h.Store, user.ID, time.Now().UTC())
		usage, state, err := h.Store.GetOrCreateToolUsage(stores.ToolUsageLookupInput{
			UserID: user.ID,
			ToolID: tool.ID,
			Now:    time.Now().UTC(),
		})
		if err == nil && usage != nil {
			payload["usage_state"] = state
		}
		payload["entitlement_active"] = entitlementActive
	}

	c.JSON(http.StatusOK, payload)
}

func (h *ToolsHandler) action(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}

	tool, err := h.Store.GetToolBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tool not found"})
		return
	}

	var req toolActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	output, err := h.Service.HandleAction(services.ToolActionInput{
		Tool:      *tool,
		UserID:    user.ID,
		Action:    req.Action,
		StageKey:  req.StageKey,
		UsedAudio: req.UsedAudio,
		Metadata:  req.Metadata,
		Now:       time.Now().UTC(),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process action"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"allowed":            output.Allowed,
		"reason":             output.Reason,
		"usage_state":        output.UsageState,
		"entitlement_active": output.EntitlementActive,
		"free_rules":         output.FreeRules,
	})
}

const maxResumeUploadBytes = 5 << 20
const maxAudioUploadBytes = 5 << 20

func (h *ToolsHandler) resume(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}

	tool, err := h.Store.GetToolBySlug(slug)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tool not found"})
		return
	}

	if h.AI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ai service unavailable"})
		return
	}

	resumeFile, err := readResumeFile(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	actionOutput, err := h.Service.HandleAction(services.ToolActionInput{
		Tool:   *tool,
		UserID: user.ID,
		Action: "resume_upload",
		Now:    time.Now().UTC(),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process action"})
		return
	}
	if !actionOutput.Allowed {
		c.JSON(http.StatusOK, gin.H{
			"allowed":            false,
			"reason":             actionOutput.Reason,
			"usage_state":        actionOutput.UsageState,
			"entitlement_active": actionOutput.EntitlementActive,
		})
		return
	}

	analysis, err := h.AI.AnalyzeResume(c.Request.Context(), services.GeminiResumeInput{
		FileName: resumeFile.FileName,
		Data:     resumeFile.Data,
		MimeType: resumeFile.MimeType,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "resume analysis failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"allowed":            true,
		"analysis":           analysis,
		"usage_state":        actionOutput.UsageState,
		"entitlement_active": actionOutput.EntitlementActive,
	})
}

type toolMentorRequest struct {
	ResumeText string                      `json:"resume_text"`
	Analysis   services.GeminiResumeOutput `json:"analysis"`
	Focus      string                      `json:"focus"`
	Subfocus   string                      `json:"subfocus"`
	Answers    map[string]interface{}      `json:"answers"`
}

func (h *ToolsHandler) mentor(c *gin.Context) {
	user, ok := h.requireUser(c)
	if !ok {
		return
	}

	tool, ok := h.requireTool(c)
	if !ok {
		return
	}

	if !h.enforceToolAccess(c, toolAccessInput{
		Tool:   *tool,
		UserID: user.ID,
		Now:    time.Now().UTC(),
	}) {
		return
	}

	if !h.requireAI(c) {
		return
	}

	req, ok := bindMentorRequest(c)
	if !ok {
		return
	}

	output, err := h.AI.GenerateMentorResponse(c.Request.Context(), services.GeminiMentorInput{
		ResumeText: req.ResumeText,
		Analysis:   req.Analysis,
		Focus:      req.Focus,
		Subfocus:   req.Subfocus,
		Answers:    normalizeAnswers(req.Answers),
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "mentor response failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"mentor": output})
}

type toolChatRequest struct {
	ResumeText string                       `json:"resume_text"`
	Analysis   services.GeminiResumeOutput  `json:"analysis"`
	Focus      string                       `json:"focus"`
	Subfocus   string                       `json:"subfocus"`
	Answers    map[string]interface{}       `json:"answers"`
	History    []services.GeminiChatMessage `json:"history"`
	Message    string                       `json:"message"`
}

func (h *ToolsHandler) chat(c *gin.Context) {
	user, ok := h.requireUser(c)
	if !ok {
		return
	}

	tool, ok := h.requireTool(c)
	if !ok {
		return
	}

	if !h.enforceToolAccess(c, toolAccessInput{
		Tool:   *tool,
		UserID: user.ID,
		Now:    time.Now().UTC(),
	}) {
		return
	}

	if !h.requireAI(c) {
		return
	}

	req, ok := bindChatRequest(c)
	if !ok {
		return
	}

	output, err := h.AI.GenerateChatResponse(c.Request.Context(), services.GeminiChatInput{
		ResumeText: req.ResumeText,
		Analysis:   req.Analysis,
		Focus:      req.Focus,
		Subfocus:   req.Subfocus,
		Answers:    normalizeAnswers(req.Answers),
		History:    normalizeChatHistory(req.History),
		Message:    strings.TrimSpace(req.Message),
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "chat response failed" + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"reply":          output.Reply,
		"reply_markdown": output.ReplyMarkdown,
	})
}

type audioFileInput struct {
	Data     []byte
	MimeType string
}

func (h *ToolsHandler) transcribe(c *gin.Context) {
	user, ok := h.requireUser(c)
	if !ok {
		return
	}

	tool, ok := h.requireTool(c)
	if !ok {
		return
	}

	if !h.enforceToolAccess(c, toolAccessInput{
		Tool:   *tool,
		UserID: user.ID,
		Now:    time.Now().UTC(),
	}) {
		return
	}

	if !h.requireAI(c) {
		return
	}

	audioFile, ok := readAudioFileSafe(c)
	if !ok {
		return
	}

	output, err := h.AI.TranscribeAudio(c.Request.Context(), services.GeminiTranscribeInput{
		Data:     audioFile.Data,
		MimeType: audioFile.MimeType,
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "transcription failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"transcript": output.Transcript})
}

type resumeFileInput struct {
	FileName string
	Data     []byte
	MimeType string
}

func readResumeFile(c *gin.Context) (resumeFileInput, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxResumeUploadBytes)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		return resumeFileInput{}, err
	}
	defer func() {
		_ = file.Close()
	}()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".pdf" {
		return resumeFileInput{}, errors.New("only PDF resumes are supported")
	}

	data, err := io.ReadAll(file)
	if err != nil {
		return resumeFileInput{}, err
	}
	if len(data) == 0 {
		return resumeFileInput{}, errors.New("resume file is empty")
	}

	mimeType := http.DetectContentType(data)
	if !strings.Contains(mimeType, "pdf") {
		mimeType = "application/pdf"
	}

	return resumeFileInput{
		FileName: header.Filename,
		Data:     data,
		MimeType: mimeType,
	}, nil
}

func readAudioFile(c *gin.Context) (audioFileInput, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxAudioUploadBytes)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		return audioFileInput{}, err
	}
	defer func() {
		_ = file.Close()
	}()

	ext := strings.ToLower(filepath.Ext(header.Filename))

	data, err := io.ReadAll(file)
	if err != nil {
		return audioFileInput{}, err
	}
	if len(data) == 0 {
		return audioFileInput{}, errors.New("audio file is empty")
	}

	mimeType := http.DetectContentType(data)
	if !isSupportedAudio(mimeType, ext) {
		return audioFileInput{}, errors.New("unsupported audio format")
	}
	mimeType = normalizeAudioMime(mimeType, ext)

	return audioFileInput{
		Data:     data,
		MimeType: mimeType,
	}, nil
}

func readAudioFileSafe(c *gin.Context) (audioFileInput, bool) {
	audioFile, err := readAudioFile(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return audioFileInput{}, false
	}
	return audioFile, true
}

type toolAccessInput struct {
	Tool   stores.ToolModel
	UserID uint
	Now    time.Time
}

type toolAccessResult struct {
	Allowed           bool
	Reason            string
	EntitlementActive bool
}

func (h *ToolsHandler) ensureToolAccess(input toolAccessInput) toolAccessResult {
	entitlementActive := toolEntitlementActive(h.Store, input.UserID, input.Now)
	if !input.Tool.IsActive {
		return toolAccessResult{Allowed: false, Reason: "INACTIVE", EntitlementActive: entitlementActive}
	}
	if input.Tool.IsPaidTool && !entitlementActive {
		return toolAccessResult{Allowed: false, Reason: "PAID_REQUIRED", EntitlementActive: entitlementActive}
	}
	return toolAccessResult{Allowed: true, EntitlementActive: entitlementActive}
}

func (h *ToolsHandler) requireUser(c *gin.Context) (*stores.UserModel, bool) {
	user, ok := middleware.CurrentUser(c)
	if !ok || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, false
	}
	return user, true
}

func (h *ToolsHandler) requireTool(c *gin.Context) (*stores.ToolModel, bool) {
	tool, err := h.loadToolFromSlug(c)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tool not found"})
		return nil, false
	}
	return tool, true
}

func (h *ToolsHandler) requireAI(c *gin.Context) bool {
	if h.AI == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "ai service unavailable"})
		return false
	}
	return true
}

func (h *ToolsHandler) enforceToolAccess(c *gin.Context, input toolAccessInput) bool {
	access := h.ensureToolAccess(input)
	if !access.Allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": access.Reason})
		return false
	}
	return true
}

func bindMentorRequest(c *gin.Context) (toolMentorRequest, bool) {
	var req toolMentorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return toolMentorRequest{}, false
	}
	if err := validateMentorRequest(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return toolMentorRequest{}, false
	}
	return req, true
}

func bindChatRequest(c *gin.Context) (toolChatRequest, bool) {
	var req toolChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return toolChatRequest{}, false
	}
	if err := validateChatRequest(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return toolChatRequest{}, false
	}
	return req, true
}

func (h *ToolsHandler) loadToolFromSlug(c *gin.Context) (*stores.ToolModel, error) {
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		return nil, errors.New("slug required")
	}
	return h.Store.GetToolBySlug(slug)
}

func validateMentorRequest(req toolMentorRequest) error {
	if strings.TrimSpace(req.ResumeText) == "" {
		return errors.New("resume_text required")
	}
	if strings.TrimSpace(req.Focus) == "" {
		return errors.New("focus required")
	}
	if strings.TrimSpace(req.Subfocus) == "" {
		return errors.New("subfocus required")
	}
	return nil
}

func validateChatRequest(req toolChatRequest) error {
	if strings.TrimSpace(req.ResumeText) == "" {
		return errors.New("resume_text required")
	}
	if strings.TrimSpace(req.Message) == "" {
		return errors.New("message required")
	}
	return nil
}

func normalizeAnswers(raw map[string]interface{}) map[string]interface{} {
	if raw == nil {
		return map[string]interface{}{}
	}
	return raw
}

func normalizeChatHistory(history []services.GeminiChatMessage) []services.GeminiChatMessage {
	if history == nil {
		return []services.GeminiChatMessage{}
	}
	return history
}

func isSupportedAudio(mimeType string, ext string) bool {
	if strings.HasPrefix(mimeType, "audio/") {
		return true
	}
	return isAudioExtension(ext)
}

func isAudioExtension(ext string) bool {
	switch ext {
	case ".webm", ".wav", ".mp3", ".m4a", ".aac":
		return true
	default:
		return false
	}
}

func normalizeAudioMime(mimeType string, ext string) string {
	if strings.HasPrefix(mimeType, "audio/") {
		return mimeType
	}
	switch ext {
	case ".wav":
		return "audio/wav"
	case ".mp3":
		return "audio/mpeg"
	case ".m4a":
		return "audio/mp4"
	case ".aac":
		return "audio/aac"
	default:
		return "audio/webm"
	}
}

func toolEntitlementActive(store *stores.Store, userID uint, now time.Time) bool {
	_, err := store.GetActiveEntitlement(stores.EntitlementLookupInput{UserID: userID, Now: now})
	return err == nil
}
