package routes

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/middleware"
	"github.com/mcpx/boilerplate/stores"
	"gorm.io/gorm"
)

type AdminProblemsHandler struct {
	Store *stores.Store
}

type adminProblemRequest struct {
	Slug              string                  `json:"slug"`
	Title             string                  `json:"title"`
	Difficulty        string                  `json:"difficulty"`
	Status            string                  `json:"status"`
	Statement         stores.ProblemStatement `json:"statement"`
	IOSpec            json.RawMessage         `json:"io_spec"`
	Constraints       json.RawMessage         `json:"constraints"`
	Tags              []string                `json:"tags"`
	Editorial         stores.ProblemEditorial `json:"editorial"`
	OfficialSolutions json.RawMessage         `json:"official_solutions"`
	PublishedAt       *time.Time              `json:"published_at"`
}

func (h *AdminProblemsHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/problems", h.list)
	rg.GET("/problems/:id", h.get)
	rg.POST("/problems", h.create)
	rg.PUT("/problems/:id", h.update)
	rg.DELETE("/problems/:id", h.delete)
}

func (h *AdminProblemsHandler) list(c *gin.Context) {
	status := strings.TrimSpace(c.Query("status"))
	difficulty := strings.TrimSpace(c.Query("difficulty"))
	query := strings.TrimSpace(c.Query("q"))
	page := parseIntDefault(c.Query("page"), 0)
	pageSize := parseIntDefault(c.Query("page_size"), 20)

	problemListOutput, err := h.Store.ListProblems(stores.ProblemListInput{
		Status:     status,
		Difficulty: difficulty,
		Query:      query,
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load problems"})
		return
	}

	items := make([]gin.H, 0, len(problemListOutput.Problems))
	for _, problemModel := range problemListOutput.Problems {
		items = append(items, adminProblemSummary(problemModel))
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "total": problemListOutput.Total})
}

func (h *AdminProblemsHandler) get(c *gin.Context) {
	problemID := parseUintDefault(c.Param("id"))
	if problemID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	problemModel, err := h.Store.GetProblemByID(stores.ProblemByIDLookupInput{ProblemID: problemID})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "problem not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"problem": adminProblemDetail(problemModel)})
}

func (h *AdminProblemsHandler) create(c *gin.Context) {
	request, err := bindProblemRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := buildProblemCreateInput(request, adminUserID(c))
	problemModel, err := h.Store.CreateProblem(input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create problem"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"problem": adminProblemDetail(problemModel)})
}

func (h *AdminProblemsHandler) update(c *gin.Context) {
	problemID := parseUintDefault(c.Param("id"))
	if problemID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	request, err := bindProblemRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := buildProblemUpdateInput(problemID, request)
	problemModel, err := h.Store.UpdateProblem(input)
	if err != nil {
		handleProblemUpdateError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"problem": adminProblemDetail(problemModel)})
}

func (h *AdminProblemsHandler) delete(c *gin.Context) {
	problemID := parseUintDefault(c.Param("id"))
	if problemID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.Store.DeleteProblem(stores.ProblemDeleteInput{ProblemID: problemID}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete problem"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func buildProblemCreateInput(request adminProblemRequest, createdBy uint) stores.ProblemCreateInput {
	return stores.ProblemCreateInput{
		ProblemCoreInput: buildProblemCoreInput(request),
		CreatedBy:        createdBy,
		PublishedAt:      request.PublishedAt,
	}
}

func buildProblemUpdateInput(problemID uint, request adminProblemRequest) stores.ProblemUpdateInput {
	return stores.ProblemUpdateInput{
		ProblemID:        problemID,
		ProblemCoreInput: buildProblemCoreInput(request),
		PublishedAt:      request.PublishedAt,
	}
}

func bindProblemRequest(c *gin.Context) (adminProblemRequest, error) {
	request := adminProblemRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		return adminProblemRequest{}, errors.New("invalid payload")
	}

	if err := validateProblemRequest(request); err != nil {
		return adminProblemRequest{}, err
	}

	return request, nil
}

func adminUserID(c *gin.Context) uint {
	adminUser, _ := middleware.CurrentUser(c)
	if adminUser == nil {
		return 0
	}

	return adminUser.ID
}

func buildProblemCoreInput(request adminProblemRequest) stores.ProblemCoreInput {
	return stores.ProblemCoreInput{
		Slug:                  request.Slug,
		Title:                 request.Title,
		Difficulty:            request.Difficulty,
		Status:                request.Status,
		Statement:             request.Statement,
		IOSpecJSON:            request.IOSpec,
		ConstraintsJSON:       request.Constraints,
		Tags:                  request.Tags,
		Editorial:             request.Editorial,
		OfficialSolutionsJSON: request.OfficialSolutions,
	}
}

func validateProblemRequest(request adminProblemRequest) error {
	slug := strings.TrimSpace(request.Slug)

	title := strings.TrimSpace(request.Title)
	if slug == "" || title == "" {
		return errors.New("slug and title are required")
	}

	if strings.TrimSpace(request.Difficulty) == "" {
		return errors.New("difficulty is required")
	}

	if strings.TrimSpace(request.Status) == "" {
		return errors.New("status is required")
	}

	if strings.TrimSpace(request.Statement.Markdown) == "" {
		return errors.New("statement markdown is required")
	}

	if err := validateProblemRawJSON(request.IOSpec, "io_spec"); err != nil {
		return err
	}

	if err := validateProblemRawJSON(request.Constraints, "constraints"); err != nil {
		return err
	}

	if err := validateProblemRawJSON(request.OfficialSolutions, "official_solutions"); err != nil {
		return err
	}

	return nil
}

func validateProblemRawJSON(raw json.RawMessage, fieldName string) error {
	if len(raw) == 0 {
		return nil
	}

	if json.Valid(raw) {
		return nil
	}

	return errors.New(fieldName + " must be valid JSON")
}

func handleProblemUpdateError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "problem not found"})
		return
	}

	if errors.Is(err, gorm.ErrInvalidData) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid problem payload"})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update problem"})
}

func adminProblemSummary(problemModel stores.ProblemModel) gin.H {
	return gin.H{
		"id":         problemModel.ID,
		"slug":       problemModel.Slug,
		"title":      problemModel.Title,
		"difficulty": problemModel.Difficulty,
		"status":     problemModel.Status,
		"tags":       stores.ParseProblemTags(problemModel.TagsJSON),
		"updated_at": problemModel.UpdatedAt,
	}
}

func adminProblemDetail(problemModel *stores.ProblemModel) gin.H {
	statement := stores.ParseProblemStatement(problemModel.StatementJSON)
	editorial := stores.ParseProblemEditorial(problemModel.EditorialJSON)

	return gin.H{
		"id":                 problemModel.ID,
		"slug":               problemModel.Slug,
		"title":              problemModel.Title,
		"difficulty":         problemModel.Difficulty,
		"status":             problemModel.Status,
		"statement":          statement,
		"io_spec":            rawProblemJSON(problemModel.IOSpecJSON, "{}"),
		"constraints":        rawProblemJSON(problemModel.ConstraintsJSON, "{}"),
		"tags":               stores.ParseProblemTags(problemModel.TagsJSON),
		"editorial":          editorial,
		"official_solutions": rawProblemJSON(problemModel.SolutionsJSON, "[]"),
		"published_at":       problemModel.PublishedAt,
		"created_at":         problemModel.CreatedAt,
		"updated_at":         problemModel.UpdatedAt,
	}
}

func rawProblemJSON(raw string, fallback string) json.RawMessage {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		trimmed = fallback
	}

	if json.Valid([]byte(trimmed)) {
		return json.RawMessage(trimmed)
	}

	return json.RawMessage(fallback)
}
