package routes

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/stores"
)

type AdminProblemDatasetsHandler struct {
	Store *stores.Store
}

type adminDatasetRequest struct {
	Type             string                  `json:"type"`
	ScoringMode      string                  `json:"scoring_mode"`
	ExecutionPolicy  *stores.ExecutionPolicy `json:"execution_policy"`
	ValidatorDefault *stores.ValidatorConfig `json:"validator_default"`
}

type adminTestcaseRequest struct {
	Input             string                  `json:"input"`
	ExpectedOutput    string                  `json:"expected_output"`
	Visibility        string                  `json:"visibility"`
	Weight            *int                    `json:"weight"`
	Group             string                  `json:"group"`
	Position          *int                    `json:"position"`
	ValidatorOverride *stores.ValidatorConfig `json:"validator_override"`
}

func (h *AdminProblemDatasetsHandler) Register(rg *gin.RouterGroup) {
	rg.POST("/problems/:id/publish", h.publishProblem)
	rg.POST("/problems/:id/datasets", h.createDataset)
	rg.GET("/problems/:id/datasets", h.listDatasets)
	rg.PUT("/datasets/:id", h.updateDataset)
	rg.DELETE("/datasets/:id", h.deleteDataset)
	rg.POST("/datasets/:id/testcases", h.createTestcase)
	rg.GET("/datasets/:id/testcases", h.listTestcases)
	rg.PUT("/testcases/:id", h.updateTestcase)
	rg.DELETE("/testcases/:id", h.deleteTestcase)
}

func (h *AdminProblemDatasetsHandler) publishProblem(c *gin.Context) {
	problemID := requireParamID(c, "id")
	if problemID == 0 {
		return
	}

	now := time.Now().UTC()
	problemModel, err := h.Store.UpdateProblemStatus(stores.ProblemStatusUpdateInput{
		ProblemID:   problemID,
		Status:      stores.ProblemStatusPublished,
		PublishedAt: &now,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to publish problem"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"problem": adminProblemDetail(problemModel)})
}

func (h *AdminProblemDatasetsHandler) createDataset(c *gin.Context) {
	problemID := requireParamID(c, "id")
	if problemID == 0 {
		return
	}

	request, err := bindAdminDatasetRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	createInput := buildDatasetCreateInput(datasetCreateInput{
		ProblemID: problemID,
		Request:   request,
	})
	datasetModel, err := h.Store.CreateDataset(createInput)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create dataset"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"dataset": adminDatasetSummary(datasetModel)})
}

func (h *AdminProblemDatasetsHandler) listDatasets(c *gin.Context) {
	problemID := requireParamID(c, "id")
	if problemID == 0 {
		return
	}

	output, err := h.Store.ListDatasets(stores.DatasetListInput{ProblemID: problemID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load datasets"})
		return
	}

	items := make([]gin.H, 0, len(output.Datasets))
	for _, dataset := range output.Datasets {
		items = append(items, adminDatasetSummary(&dataset))
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *AdminProblemDatasetsHandler) updateDataset(c *gin.Context) {
	datasetID := requireParamID(c, "id")
	if datasetID == 0 {
		return
	}

	request, err := bindAdminDatasetRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updateInput := buildDatasetUpdateInput(datasetUpdateInput{
		DatasetID: datasetID,
		Request:   request,
	})
	datasetModel, err := h.Store.UpdateDataset(updateInput)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update dataset"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"dataset": adminDatasetSummary(datasetModel)})
}

func (h *AdminProblemDatasetsHandler) deleteDataset(c *gin.Context) {
	datasetID := requireParamID(c, "id")
	if datasetID == 0 {
		return
	}

	if err := h.Store.DeleteDataset(stores.DatasetDeleteInput{DatasetID: datasetID}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete dataset"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *AdminProblemDatasetsHandler) createTestcase(c *gin.Context) {
	datasetID := requireParamID(c, "id")
	if datasetID == 0 {
		return
	}

	request, err := bindAdminTestcaseRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	createInput := buildTestcaseCreateInput(testcaseCreateInput{
		DatasetID: datasetID,
		Request:   request,
	})
	testcaseModel, err := h.Store.CreateTestcase(createInput)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create testcase"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"testcase": adminTestcaseSummary(testcaseModel)})
}

func (h *AdminProblemDatasetsHandler) listTestcases(c *gin.Context) {
	datasetID := requireParamID(c, "id")
	if datasetID == 0 {
		return
	}

	output, err := h.Store.ListTestcases(stores.TestcaseListInput{DatasetID: datasetID})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load testcases"})
		return
	}

	items := make([]gin.H, 0, len(output.Testcases))
	for _, testcase := range output.Testcases {
		items = append(items, adminTestcaseSummary(&testcase))
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *AdminProblemDatasetsHandler) updateTestcase(c *gin.Context) {
	testcaseID := requireParamID(c, "id")
	if testcaseID == 0 {
		return
	}

	request, err := bindAdminTestcaseRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updateInput := buildTestcaseUpdateInput(testcaseUpdateInput{
		TestcaseID: testcaseID,
		Request:    request,
	})
	testcaseModel, err := h.Store.UpdateTestcase(updateInput)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update testcase"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"testcase": adminTestcaseSummary(testcaseModel)})
}

func (h *AdminProblemDatasetsHandler) deleteTestcase(c *gin.Context) {
	testcaseID := requireParamID(c, "id")
	if testcaseID == 0 {
		return
	}

	if err := h.Store.DeleteTestcase(stores.TestcaseDeleteInput{TestcaseID: testcaseID}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete testcase"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func requireParamID(c *gin.Context, name string) uint {
	value := parseUintDefault(c.Param(name))
	if value == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return 0
	}

	return value
}

func bindAdminDatasetRequest(c *gin.Context) (adminDatasetRequest, error) {
	request := adminDatasetRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		return adminDatasetRequest{}, errors.New("invalid payload")
	}

	return request, nil
}

func bindAdminTestcaseRequest(c *gin.Context) (adminTestcaseRequest, error) {
	request := adminTestcaseRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		return adminTestcaseRequest{}, errors.New("invalid payload")
	}

	return request, nil
}

type datasetCreateInput struct {
	ProblemID uint
	Request   adminDatasetRequest
}

func buildDatasetCreateInput(input datasetCreateInput) stores.DatasetCreateInput {
	executionPolicy := stores.ExecutionPolicy{}
	if input.Request.ExecutionPolicy != nil {
		executionPolicy = *input.Request.ExecutionPolicy
	}

	validatorDefault := stores.ValidatorConfig{}
	if input.Request.ValidatorDefault != nil {
		validatorDefault = *input.Request.ValidatorDefault
	}

	return stores.DatasetCreateInput{
		ProblemID:        input.ProblemID,
		Type:             input.Request.Type,
		ScoringMode:      input.Request.ScoringMode,
		ExecutionPolicy:  executionPolicy,
		ValidatorDefault: validatorDefault,
	}
}

type datasetUpdateInput struct {
	DatasetID uint
	Request   adminDatasetRequest
}

func buildDatasetUpdateInput(input datasetUpdateInput) stores.DatasetUpdateInput {
	return stores.DatasetUpdateInput{
		DatasetID:        input.DatasetID,
		Type:             input.Request.Type,
		ScoringMode:      input.Request.ScoringMode,
		ExecutionPolicy:  input.Request.ExecutionPolicy,
		ValidatorDefault: input.Request.ValidatorDefault,
	}
}

type testcaseCreateInput struct {
	DatasetID uint
	Request   adminTestcaseRequest
}

func buildTestcaseCreateInput(input testcaseCreateInput) stores.TestcaseCreateInput {
	weight := 0
	if input.Request.Weight != nil {
		weight = *input.Request.Weight
	}

	position := 0
	if input.Request.Position != nil {
		position = *input.Request.Position
	}

	return stores.TestcaseCreateInput{
		DatasetID:         input.DatasetID,
		InputText:         input.Request.Input,
		ExpectedText:      input.Request.ExpectedOutput,
		Visibility:        input.Request.Visibility,
		Weight:            weight,
		Group:             input.Request.Group,
		Position:          position,
		ValidatorOverride: input.Request.ValidatorOverride,
	}
}

type testcaseUpdateInput struct {
	TestcaseID uint
	Request    adminTestcaseRequest
}

func buildTestcaseUpdateInput(input testcaseUpdateInput) stores.TestcaseUpdateInput {
	return stores.TestcaseUpdateInput{
		TestcaseID:        input.TestcaseID,
		InputText:         input.Request.Input,
		ExpectedText:      input.Request.ExpectedOutput,
		Visibility:        input.Request.Visibility,
		Weight:            input.Request.Weight,
		Group:             input.Request.Group,
		Position:          input.Request.Position,
		ValidatorOverride: input.Request.ValidatorOverride,
	}
}

func adminDatasetSummary(dataset *stores.DatasetModel) gin.H {
	if dataset == nil {
		return gin.H{}
	}

	return gin.H{
		"id":                dataset.ID,
		"problem_id":        dataset.ProblemID,
		"type":              dataset.Type,
		"scoring_mode":      dataset.ScoringMode,
		"execution_policy":  stores.ParseExecutionPolicy(dataset.ExecutionPolicyJSON),
		"validator_default": stores.ParseValidatorConfig(dataset.ValidatorDefaultJSON),
		"created_at":        dataset.CreatedAt,
		"updated_at":        dataset.UpdatedAt,
	}
}

func adminTestcaseSummary(testcase *stores.TestcaseModel) gin.H {
	if testcase == nil {
		return gin.H{}
	}

	return gin.H{
		"id":                 testcase.ID,
		"dataset_id":         testcase.DatasetID,
		"input":              testcase.InputText,
		"expected_output":    testcase.ExpectedText,
		"visibility":         testcase.Visibility,
		"weight":             testcase.Weight,
		"group":              testcase.Group,
		"position":           testcase.Position,
		"validator_override": stores.ParseValidatorConfig(testcase.ValidatorOverrideJSON),
		"created_at":         testcase.CreatedAt,
		"updated_at":         testcase.UpdatedAt,
	}
}
