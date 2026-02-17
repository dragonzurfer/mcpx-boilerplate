package routes

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/stores"
)

type AdminProblemSolutionsHandler struct {
	Store *stores.Store
}

type adminSolutionRequest struct {
	Language        string `json:"language"`
	CodeText        string `json:"code"`
	ComplexityJSON  string `json:"complexity"`
	ApproachSummary string `json:"approach_summary"`
	IsReference     *bool  `json:"is_reference"`
}

func (h *AdminProblemSolutionsHandler) Register(rg *gin.RouterGroup) {
	rg.POST("/problems/:id/solutions", h.createSolution)
	rg.GET("/problems/:id/solutions", h.listSolutions)
	rg.PUT("/solutions/:id", h.updateSolution)
	rg.DELETE("/solutions/:id", h.deleteSolution)
}

func (h *AdminProblemSolutionsHandler) createSolution(c *gin.Context) {
	problemID := requireParamID(c, "id")
	if problemID == 0 {
		return
	}

	request, err := bindAdminSolutionRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	createInput := buildSolutionCreateInput(solutionCreateInput{
		ProblemID: problemID,
		Request:   request,
	})
	solutionModel, err := h.Store.CreateSolution(createInput)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create solution"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"solution": adminSolutionSummary(solutionModel)})
}

func (h *AdminProblemSolutionsHandler) listSolutions(c *gin.Context) {
	problemID := requireParamID(c, "id")
	if problemID == 0 {
		return
	}

	output, err := h.Store.ListSolutions(stores.SolutionListInput{ProblemID: problemID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load solutions"})
		return
	}

	items := make([]gin.H, 0, len(output.Solutions))
	for _, solution := range output.Solutions {
		items = append(items, adminSolutionSummary(&solution))
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *AdminProblemSolutionsHandler) updateSolution(c *gin.Context) {
	solutionID := requireParamID(c, "id")
	if solutionID == 0 {
		return
	}

	request, err := bindAdminSolutionRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updateInput := buildSolutionUpdateInput(solutionUpdateInput{
		SolutionID: solutionID,
		Request:    request,
	})
	solutionModel, err := h.Store.UpdateSolution(updateInput)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update solution"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"solution": adminSolutionSummary(solutionModel)})
}

func (h *AdminProblemSolutionsHandler) deleteSolution(c *gin.Context) {
	solutionID := requireParamID(c, "id")
	if solutionID == 0 {
		return
	}

	if err := h.Store.DeleteSolution(stores.SolutionDeleteInput{SolutionID: solutionID}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete solution"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func bindAdminSolutionRequest(c *gin.Context) (adminSolutionRequest, error) {
	request := adminSolutionRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		return adminSolutionRequest{}, errors.New("invalid payload")
	}

	return request, nil
}

type solutionCreateInput struct {
	ProblemID uint
	Request   adminSolutionRequest
}

func buildSolutionCreateInput(input solutionCreateInput) stores.SolutionCreateInput {
	isReference := false
	if input.Request.IsReference != nil {
		isReference = *input.Request.IsReference
	}

	return stores.SolutionCreateInput{
		ProblemID:       input.ProblemID,
		Language:        input.Request.Language,
		CodeText:        input.Request.CodeText,
		ComplexityJSON:  input.Request.ComplexityJSON,
		ApproachSummary: input.Request.ApproachSummary,
		IsReference:     isReference,
	}
}

type solutionUpdateInput struct {
	SolutionID uint
	Request    adminSolutionRequest
}

func buildSolutionUpdateInput(input solutionUpdateInput) stores.SolutionUpdateInput {
	return stores.SolutionUpdateInput{
		SolutionID:      input.SolutionID,
		Language:        input.Request.Language,
		CodeText:        input.Request.CodeText,
		ComplexityJSON:  input.Request.ComplexityJSON,
		ApproachSummary: input.Request.ApproachSummary,
		IsReference:     input.Request.IsReference,
	}
}

func adminSolutionSummary(solution *stores.SolutionModel) gin.H {
	if solution == nil {
		return gin.H{}
	}

	return gin.H{
		"id":               solution.ID,
		"problem_id":       solution.ProblemID,
		"language":         solution.Language,
		"code":             solution.CodeText,
		"complexity":       solution.ComplexityJSON,
		"approach_summary": solution.ApproachSummary,
		"is_reference":     solution.IsReference,
		"created_at":       solution.CreatedAt,
		"updated_at":       solution.UpdatedAt,
	}
}
