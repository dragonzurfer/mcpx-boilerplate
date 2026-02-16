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
