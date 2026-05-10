package routes

import (
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

const freeAIAnalysisLimit = 5

var errFreeAIAnalysisLimitReached = errors.New("free ai analysis limit reached")

type AIAnalysisHandler struct {
	Service *services.AIAnalysisService
}

type analysisRequest struct {
	SubmissionID      uint   `json:"submission_id"`
	AnalysisMode      string `json:"analysis_mode"`
	HintLevel         int    `json:"hint_level"`
	AllowFullSolution bool   `json:"allow_full_solution"`
}

type analysisPolicyRequest struct {
	AnalysisMode      string `json:"analysis_mode"`
	HintLevel         int    `json:"hint_level"`
	AllowFullSolution bool   `json:"allow_full_solution"`
}

type analysisVerifyRequest struct {
	SubmissionID uint                  `json:"submission_id"`
	Language     string                `json:"language"`
	Code         string                `json:"code"`
	JudgeResult  services.JudgeResult  `json:"judge_result"`
	Receipt      services.Receipt      `json:"receipt"`
	Policy       analysisPolicyRequest `json:"policy"`
}

type analysisPolicyInput struct {
	Mode              string
	HintLevel         int
	AllowFullSolution bool
}

func (h *AIAnalysisHandler) Register(rg *gin.RouterGroup) {
	rg.POST("/ai-analysis", h.create)
	rg.POST("/ai-analysis/verify", h.verify)
}

func (h *AIAnalysisHandler) create(c *gin.Context) {
	user := requireAnalysisUser(c)
	if user == nil {
		return
	}

	request, err := bindAnalysisRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	policy := normalizeAnalysisPolicy(analysisPolicyInput{
		Mode:              request.AnalysisMode,
		HintLevel:         request.HintLevel,
		AllowFullSolution: request.AllowFullSolution,
	})

	if err := h.ensureAnalysisAccess(user.ID); err != nil {
		if errors.Is(err, errFreeAIAnalysisLimitReached) {
			c.JSON(http.StatusPaymentRequired, gin.H{"error": "FREE_AI_ANALYSIS_LIMIT_REACHED"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "analysis limit unavailable"})
		return
	}

	response, err := h.Service.AnalyzeSubmission(services.AnalysisSubmissionInput{
		SubmissionID: request.SubmissionID,
		UserID:       user.ID,
		Policy:       policy,
	})
	if err != nil {
		writeAnalysisError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *AIAnalysisHandler) verify(c *gin.Context) {
	user := requireAnalysisUser(c)
	if user == nil {
		return
	}

	request, err := bindAnalysisVerifyRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	policy := normalizeAnalysisPolicy(analysisPolicyInput{
		Mode:              request.Policy.AnalysisMode,
		HintLevel:         request.Policy.HintLevel,
		AllowFullSolution: request.Policy.AllowFullSolution,
	})

	if err := h.ensureAnalysisAccess(user.ID); err != nil {
		if errors.Is(err, errFreeAIAnalysisLimitReached) {
			c.JSON(http.StatusPaymentRequired, gin.H{"error": "FREE_AI_ANALYSIS_LIMIT_REACHED"})
			return
		}

		c.JSON(http.StatusInternalServerError, gin.H{"error": "analysis limit unavailable"})
		return
	}

	response, err := h.Service.AnalyzeVerifiedPayload(services.AnalysisVerifyInput{
		SubmissionID: request.SubmissionID,
		UserID:       user.ID,
		Language:     request.Language,
		Code:         request.Code,
		JudgeResult:  request.JudgeResult,
		Receipt:      request.Receipt,
		Policy:       policy,
	})
	if err != nil {
		writeAnalysisError(c, err)
		return
	}

	c.JSON(http.StatusOK, response)
}

func requireAnalysisUser(c *gin.Context) *stores.UserModel {
	user, ok := middleware.CurrentUser(c)
	if !ok || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil
	}

	return user
}

func bindAnalysisRequest(c *gin.Context) (analysisRequest, error) {
	request := analysisRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		return analysisRequest{}, errors.New("invalid payload")
	}

	if request.SubmissionID == 0 {
		return analysisRequest{}, errors.New("submission_id required")
	}

	return request, nil
}

func bindAnalysisVerifyRequest(c *gin.Context) (analysisVerifyRequest, error) {
	request := analysisVerifyRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		return analysisVerifyRequest{}, errors.New("invalid payload")
	}

	if request.SubmissionID == 0 {
		return analysisVerifyRequest{}, errors.New("submission_id required")
	}

	if strings.TrimSpace(request.Code) == "" {
		return analysisVerifyRequest{}, errors.New("code is required")
	}

	return request, nil
}

func normalizeAnalysisPolicy(input analysisPolicyInput) services.AnalysisPolicy {
	mode := strings.ToUpper(strings.TrimSpace(input.Mode))
	if mode == "" {
		mode = "COACH"
	}

	hintLevel := input.HintLevel
	if hintLevel <= 0 {
		hintLevel = 1
	}

	return services.AnalysisPolicy{
		Mode:              mode,
		HintLevel:         hintLevel,
		AllowFullSolution: input.AllowFullSolution,
	}
}

func (h *AIAnalysisHandler) ensureAnalysisAccess(userID uint) error {
	if userID == 0 {
		return gorm.ErrRecordNotFound
	}

	_, entitlementErr := h.Service.Store.GetActiveEntitlement(stores.EntitlementLookupInput{
		UserID: userID,
		Now:    time.Now().UTC(),
	})
	if entitlementErr == nil {
		return nil
	}
	if !errors.Is(entitlementErr, gorm.ErrRecordNotFound) {
		return entitlementErr
	}

	countOutput, countErr := h.Service.Store.CountUserAIAnalyses(stores.UserAIAnalysisCountInput{UserID: userID})
	if countErr != nil {
		return countErr
	}
	if countOutput.Count >= freeAIAnalysisLimit {
		return errFreeAIAnalysisLimitReached
	}

	return nil
}

func writeAnalysisError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	if strings.Contains(strings.ToLower(err.Error()), "forbidden") {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "analysis failed"})
}
