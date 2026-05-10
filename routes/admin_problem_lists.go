package routes

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/stores"
	"gorm.io/gorm"
)

type AdminProblemListsHandler struct {
	Store *stores.Store
}

type adminProblemListRequest struct {
	Name        string `json:"name"`
	Slug        string `json:"slug"`
	Description string `json:"description"`
	ProblemIDs  []uint `json:"problem_ids"`
	IsDefault   bool   `json:"is_default"`
}

func (h *AdminProblemListsHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/problem-lists", h.list)
	rg.GET("/problem-lists/:id", h.get)
	rg.POST("/problem-lists", h.create)
	rg.PUT("/problem-lists/:id", h.update)
	rg.DELETE("/problem-lists/:id", h.delete)
}

func (h *AdminProblemListsHandler) list(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	output, err := h.Store.ListProblemLists(stores.ProblemListListInput{
		Query:               query,
		IncludeProblemCount: true,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load problem lists"})
		return
	}

	items := make([]gin.H, 0, len(output.Lists))
	for _, listModel := range output.Lists {
		items = append(items, adminProblemListSummary(listModel, output.ProblemCountByList[listModel.ID]))
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *AdminProblemListsHandler) get(c *gin.Context) {
	listID := parseUintDefault(c.Param("id"))
	if listID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	detail, err := h.loadProblemListDetail(listID)
	if err != nil {
		handleProblemListError(c, err, "failed to load problem list")
		return
	}

	c.JSON(http.StatusOK, gin.H{"list": detail})
}

func (h *AdminProblemListsHandler) create(c *gin.Context) {
	request, err := bindAdminProblemListRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	listModel, err := h.Store.CreateProblemList(stores.ProblemListCreateInput{
		Name:        request.Name,
		Slug:        request.Slug,
		Description: request.Description,
		ProblemIDs:  request.ProblemIDs,
		CreatedBy:   adminUserID(c),
		IsDefault:   request.IsDefault,
	})
	if err != nil {
		handleProblemListError(c, err, "failed to create list")
		return
	}

	detail, err := h.loadProblemListDetail(listModel.ID)
	if err != nil {
		handleProblemListError(c, err, "failed to load problem list")
		return
	}

	c.JSON(http.StatusOK, gin.H{"list": detail})
}

func (h *AdminProblemListsHandler) update(c *gin.Context) {
	listID := parseUintDefault(c.Param("id"))
	if listID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	request, err := bindAdminProblemListRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err = h.Store.UpdateProblemList(stores.ProblemListUpdateInput{
		ProblemListID: listID,
		Name:          request.Name,
		Slug:          request.Slug,
		Description:   request.Description,
		ProblemIDs:    request.ProblemIDs,
		IsDefault:     request.IsDefault,
	})
	if err != nil {
		handleProblemListError(c, err, "failed to update list")
		return
	}

	detail, err := h.loadProblemListDetail(listID)
	if err != nil {
		handleProblemListError(c, err, "failed to load problem list")
		return
	}

	c.JSON(http.StatusOK, gin.H{"list": detail})
}

func (h *AdminProblemListsHandler) delete(c *gin.Context) {
	listID := parseUintDefault(c.Param("id"))
	if listID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	err := h.Store.DeleteProblemList(stores.ProblemListDeleteInput{ProblemListID: listID})
	if err != nil {
		handleProblemListError(c, err, "failed to delete list")
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func bindAdminProblemListRequest(c *gin.Context) (adminProblemListRequest, error) {
	request := adminProblemListRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		return adminProblemListRequest{}, errors.New("invalid payload")
	}

	if strings.TrimSpace(request.Name) == "" {
		return adminProblemListRequest{}, errors.New("name is required")
	}

	return request, nil
}

func (h *AdminProblemListsHandler) loadProblemListDetail(listID uint) (gin.H, error) {
	listModel, err := h.Store.GetProblemListByID(stores.ProblemListLookupInput{ProblemListID: listID})
	if err != nil {
		return gin.H{}, err
	}

	problemIDs, err := h.Store.ListProblemListProblemIDs(stores.ProblemListProblemIDsInput{ProblemListID: listID})
	if err != nil {
		return gin.H{}, err
	}

	problems, err := h.Store.ListProblemListProblems(stores.ProblemListProblemsInput{
		ProblemListID: listID,
		PublishedOnly: false,
	})
	if err != nil {
		return gin.H{}, err
	}

	return adminProblemListDetail(listModel, problemIDs, problems), nil
}

func adminProblemListSummary(listModel stores.ProblemListModel, problemCount int64) gin.H {
	return gin.H{
		"id":            listModel.ID,
		"name":          listModel.Name,
		"slug":          listModel.Slug,
		"description":   listModel.Description,
		"is_default":    listModel.IsDefault,
		"problem_count": problemCount,
		"updated_at":    listModel.UpdatedAt,
	}
}

func adminProblemListDetail(listModel *stores.ProblemListModel, problemIDs []uint, problems []stores.ProblemModel) gin.H {
	problemItems := make([]gin.H, 0, len(problems))
	for _, problemModel := range problems {
		problemItems = append(problemItems, adminProblemSummary(problemModel))
	}

	return gin.H{
		"id":            listModel.ID,
		"name":          listModel.Name,
		"slug":          listModel.Slug,
		"description":   listModel.Description,
		"is_default":    listModel.IsDefault,
		"problem_ids":   problemIDs,
		"problem_count": len(problemItems),
		"problems":      problemItems,
		"created_at":    listModel.CreatedAt,
		"updated_at":    listModel.UpdatedAt,
	}
}

func handleProblemListError(c *gin.Context, err error, fallbackMessage string) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "problem list not found"})
		return
	}

	if errors.Is(err, gorm.ErrInvalidData) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid problem list payload"})
		return
	}

	c.JSON(http.StatusInternalServerError, gin.H{"error": fallbackMessage})
}
