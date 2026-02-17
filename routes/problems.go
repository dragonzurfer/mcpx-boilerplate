package routes

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/stores"
)

type ProblemsHandler struct {
	Store *stores.Store
}

func (h *ProblemsHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/problems", h.list)
	rg.GET("/problems/:slug", h.get)
}

func (h *ProblemsHandler) list(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	difficulty := strings.TrimSpace(c.Query("difficulty"))
	page := parseIntDefault(c.Query("page"), 0)
	pageSize := parseIntDefault(c.Query("page_size"), 20)

	problemListOutput, err := h.Store.ListProblems(stores.ProblemListInput{
		Status:     stores.ProblemStatusPublished,
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
		items = append(items, publicProblemSummary(problemModel))
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "total": problemListOutput.Total})
}

func (h *ProblemsHandler) get(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}

	problemModel, err := h.Store.GetProblemBySlug(stores.ProblemLookupInput{Slug: slug})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "problem not found"})
		return
	}

	if strings.ToUpper(strings.TrimSpace(problemModel.Status)) != stores.ProblemStatusPublished {
		c.JSON(http.StatusNotFound, gin.H{"error": "problem not found"})
		return
	}

	datasets, err := h.Store.ListDatasets(stores.DatasetListInput{ProblemID: problemModel.ID, Type: stores.DatasetTypePublic})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load datasets"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"problem": publicProblemDetail(problemModel, datasets.Datasets)})
}

func publicProblemSummary(problemModel stores.ProblemModel) gin.H {
	return gin.H{
		"id":         problemModel.ID,
		"slug":       problemModel.Slug,
		"title":      problemModel.Title,
		"difficulty": problemModel.Difficulty,
		"status":     problemModel.Status,
		"tags":       stores.ParseProblemTags(problemModel.TagsJSON),
	}
}

func publicProblemDetail(problemModel *stores.ProblemModel, datasets []stores.DatasetModel) gin.H {
	statement := stores.ParseProblemStatement(problemModel.StatementJSON)
	ioSpec := rawProblemJSON(problemModel.IOSpecJSON, "{}")
	constraints := rawProblemJSON(problemModel.ConstraintsJSON, "{}")

	datasetItems := make([]gin.H, 0, len(datasets))
	for _, dataset := range datasets {
		datasetItems = append(datasetItems, gin.H{
			"id":   dataset.ID,
			"type": dataset.Type,
		})
	}

	return gin.H{
		"id":          problemModel.ID,
		"slug":        problemModel.Slug,
		"title":       problemModel.Title,
		"difficulty":  problemModel.Difficulty,
		"status":      problemModel.Status,
		"statement":   statement,
		"io_spec":     ioSpec,
		"constraints": constraints,
		"tags":        stores.ParseProblemTags(problemModel.TagsJSON),
		"datasets":    datasetItems,
	}
}
