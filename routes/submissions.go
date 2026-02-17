package routes

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/middleware"
	"github.com/mcpx/boilerplate/services"
	"github.com/mcpx/boilerplate/stores"
	"gorm.io/gorm"
)

type SubmissionsHandler struct {
	Store *stores.Store
}

type submissionCreateRequest struct {
	ProblemID uint   `json:"problem_id"`
	Mode      string `json:"mode"`
	Language  string `json:"language"`
	Code      string `json:"code"`
}

type normalizedSubmissionRequest struct {
	ProblemID uint
	Mode      string
	Language  string
	Code      string
}

type datasetSelection struct {
	DatasetID   uint
	DatasetType string
}

func (h *SubmissionsHandler) Register(rg *gin.RouterGroup) {
	rg.POST("/submissions", h.create)
	rg.GET("/submissions/:id", h.get)
	rg.GET("/submissions/:id/result", h.result)
	rg.GET("/users/:id/problems/:problem_id/history", h.history)
}

func (h *SubmissionsHandler) create(c *gin.Context) {
	user := requireSubmissionUser(c)
	if user == nil {
		return
	}

	request, err := bindSubmissionRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	normalized, err := normalizeSubmissionRequest(request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	problemModel, err := h.Store.GetProblemByID(stores.ProblemByIDLookupInput{ProblemID: normalized.ProblemID})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "problem not found"})
		return
	}

	selection, err := h.selectDatasetForMode(problemModel.ID, normalized.Mode)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.ensureSubmissionAccess(user.ID, normalized.Mode); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}

	limits, err := services.ResolveExecutionLimits(services.ExecutionLimitInput{Problem: problemModel, Language: normalized.Language})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid constraints"})
		return
	}

	submission, err := h.createSubmissionRecord(submissionRecordInput{
		UserID:    user.ID,
		Problem:   problemModel,
		Request:   normalized,
		Selection: selection,
		Limits:    limits,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create submission"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"submission_id": submission.ID,
		"status":        submission.Status,
		"dataset_type":  selection.DatasetType,
	})
}

func (h *SubmissionsHandler) get(c *gin.Context) {
	submissionID := parseUintDefault(c.Param("id"))
	if submissionID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	submission, err := h.Store.GetSubmissionByID(stores.SubmissionLookupInput{SubmissionID: submissionID})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "submission not found"})
		return
	}

	if !h.isSubmissionOwner(c, submission) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"submission": submissionSummary(submission)})
}

func (h *SubmissionsHandler) result(c *gin.Context) {
	submissionID := parseUintDefault(c.Param("id"))
	if submissionID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	submission, err := h.Store.GetSubmissionByID(stores.SubmissionLookupInput{SubmissionID: submissionID})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "submission not found"})
		return
	}

	if !h.isSubmissionOwner(c, submission) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	resultModel, err := h.Store.GetSubmissionResult(stores.SubmissionResultLookupInput{SubmissionID: submissionID})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusOK, gin.H{"status": submission.Status})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load result"})
		return
	}

	dataset, err := h.Store.GetDatasetByID(stores.DatasetLookupInput{DatasetID: submission.DatasetID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load dataset"})
		return
	}

	resultPayload := buildResultPayload(resultModel)
	resultPayload = sanitizeResultPayload(sanitizeResultInput{
		Payload:    resultPayload,
		Submission: submission,
		Dataset:    dataset,
	})

	c.JSON(http.StatusOK, gin.H{
		"submission": submissionSummary(submission),
		"result":     resultPayload,
		"receipt":    parseJSON(resultModel.ReceiptJSON),
	})
}

func (h *SubmissionsHandler) history(c *gin.Context) {
	userID := parseUintDefault(c.Param("id"))
	problemID := parseUintDefault(c.Param("problem_id"))

	if userID == 0 || problemID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	currentUser, ok := middleware.CurrentUser(c)
	if !ok || currentUser == nil || currentUser.ID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	output, err := h.Store.ListSubmissions(stores.SubmissionListInput{UserID: userID, ProblemID: problemID, Limit: 50})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load history"})
		return
	}

	items := make([]gin.H, 0, len(output.Submissions))
	for _, submission := range output.Submissions {
		items = append(items, submissionSummary(&submission))
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func requireSubmissionUser(c *gin.Context) *stores.UserModel {
	user, ok := middleware.CurrentUser(c)
	if !ok || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil
	}

	return user
}

func bindSubmissionRequest(c *gin.Context) (submissionCreateRequest, error) {
	request := submissionCreateRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		return submissionCreateRequest{}, errors.New("invalid payload")
	}

	return request, nil
}

func normalizeSubmissionRequest(request submissionCreateRequest) (normalizedSubmissionRequest, error) {
	if request.ProblemID == 0 {
		return normalizedSubmissionRequest{}, errors.New("problem_id required")
	}

	language := normalizeSubmissionLanguage(request.Language)
	if language == "" {
		return normalizedSubmissionRequest{}, errors.New("unsupported language")
	}

	mode := strings.ToUpper(strings.TrimSpace(request.Mode))
	if mode == "" {
		mode = stores.SubmissionModeRun
	}
	if mode != stores.SubmissionModeRun && mode != stores.SubmissionModeSubmit {
		return normalizedSubmissionRequest{}, errors.New("invalid mode")
	}

	code := strings.TrimSpace(request.Code)
	if code == "" {
		return normalizedSubmissionRequest{}, errors.New("code is required")
	}

	return normalizedSubmissionRequest{
		ProblemID: request.ProblemID,
		Mode:      mode,
		Language:  language,
		Code:      code,
	}, nil
}

type submissionRecordInput struct {
	UserID    uint
	Problem   *stores.ProblemModel
	Request   normalizedSubmissionRequest
	Selection datasetSelection
	Limits    services.ExecutionLimits
}

func (h *SubmissionsHandler) createSubmissionRecord(input submissionRecordInput) (*stores.SubmissionModel, error) {
	queuedAt := time.Now().UTC()
	limitsJSON := mustMarshal(input.Limits)

	submissionInput := stores.SubmissionCreateInput{
		UserID:     input.UserID,
		ProblemID:  input.Problem.ID,
		Language:   input.Request.Language,
		Mode:       input.Request.Mode,
		DatasetID:  input.Selection.DatasetID,
		Status:     stores.SubmissionStatusQueued,
		CodeText:   input.Request.Code,
		CodeHash:   services.HashString(input.Request.Code),
		LimitsJSON: limitsJSON,
		QueuedAt:   queuedAt,
	}

	return h.Store.CreateSubmission(submissionInput)
}

func (h *SubmissionsHandler) selectDatasetForMode(problemID uint, mode string) (datasetSelection, error) {
	datasetType := stores.DatasetTypePublic
	if strings.ToUpper(strings.TrimSpace(mode)) == stores.SubmissionModeSubmit {
		datasetType = stores.DatasetTypeHidden
	}

	output, err := h.Store.ListDatasets(stores.DatasetListInput{ProblemID: problemID, Type: datasetType})
	if err != nil {
		return datasetSelection{}, err
	}
	if len(output.Datasets) == 0 {
		return datasetSelection{}, errors.New("dataset not configured")
	}

	return datasetSelection{
		DatasetID:   output.Datasets[0].ID,
		DatasetType: datasetType,
	}, nil
}

func (h *SubmissionsHandler) ensureSubmissionAccess(userID uint, mode string) error {
	modeValue := strings.ToUpper(strings.TrimSpace(mode))
	if modeValue != stores.SubmissionModeSubmit {
		return nil
	}

	_, err := h.Store.GetActiveEntitlement(stores.EntitlementLookupInput{UserID: userID, Now: time.Now().UTC()})
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return errors.New("active plan required for submit")
	}
	return errors.New("unable to verify plan")
}

func (h *SubmissionsHandler) isSubmissionOwner(c *gin.Context, submission *stores.SubmissionModel) bool {
	if submission == nil {
		return false
	}

	user, ok := middleware.CurrentUser(c)
	if ok && user != nil && user.ID == submission.UserID {
		return true
	}

	if user == nil {
		return false
	}

	role := strings.ToUpper(strings.TrimSpace(user.Role))
	return role == stores.UserRoleAdmin || role == stores.UserRoleSuperAdmin
}

func submissionSummary(submission *stores.SubmissionModel) gin.H {
	if submission == nil {
		return gin.H{}
	}

	return gin.H{
		"id":          submission.ID,
		"problem_id":  submission.ProblemID,
		"language":    submission.Language,
		"mode":        submission.Mode,
		"dataset_id":  submission.DatasetID,
		"status":      submission.Status,
		"queued_at":   submission.QueuedAt,
		"started_at":  submission.StartedAt,
		"finished_at": submission.FinishedAt,
	}
}

func normalizeSubmissionLanguage(language string) string {
	value := strings.ToLower(strings.TrimSpace(language))
	switch value {
	case "go", "golang":
		return "go"
	case "c":
		return "c"
	case "cpp", "c++":
		return "cpp"
	case "java":
		return "java"
	default:
		return ""
	}
}

func buildResultPayload(result *stores.SubmissionResultModel) gin.H {
	return gin.H{
		"overall":   parseJSON(result.OverallJSON),
		"compile":   parseJSON(result.CompileJSON),
		"tests":     parseJSONArray(result.TestsJSON),
		"timing":    parseJSON(result.TimingJSON),
		"artifacts": parseJSON(result.ArtifactsJSON),
	}
}

type sanitizeResultInput struct {
	Payload    gin.H
	Submission *stores.SubmissionModel
	Dataset    *stores.DatasetModel
}

func sanitizeResultPayload(input sanitizeResultInput) gin.H {
	mode := strings.ToUpper(strings.TrimSpace(input.Submission.Mode))
	dataType := strings.ToUpper(strings.TrimSpace(input.Dataset.Type))

	if mode != stores.SubmissionModeSubmit && dataType != stores.DatasetTypeHidden {
		return input.Payload
	}

	tests, ok := input.Payload["tests"].([]interface{})
	if !ok {
		return input.Payload
	}

	sanitized := make([]interface{}, 0, len(tests))

	for _, item := range tests {
		row, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

		detail, ok := row["detail"].(map[string]interface{})
		if ok {
			delete(detail, "input")
			delete(detail, "expected")
			delete(detail, "actual")
			row["detail"] = detail
		}

		sanitized = append(sanitized, row)
	}

	input.Payload["tests"] = sanitized
	return input.Payload
}

func parseJSON(raw string) gin.H {
	value := map[string]interface{}{}
	if strings.TrimSpace(raw) == "" {
		return value
	}

	if err := json.Unmarshal([]byte(raw), &value); err != nil {
		return map[string]interface{}{}
	}

	return value
}

func parseJSONArray(raw string) []interface{} {
	if strings.TrimSpace(raw) == "" {
		return []interface{}{}
	}

	items := []interface{}{}
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		return []interface{}{}
	}

	return items
}

func mustMarshal(value interface{}) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "null"
	}

	return string(encoded)
}
