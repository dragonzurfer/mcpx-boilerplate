package stores

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type DatasetCreateInput struct {
	ProblemID        uint
	Type             string
	ScoringMode      string
	ExecutionPolicy  ExecutionPolicy
	ValidatorDefault ValidatorConfig
}

func (s *Store) CreateDataset(input DatasetCreateInput) (*DatasetModel, error) {
	if input.ProblemID == 0 {
		return nil, gorm.ErrInvalidData
	}

	normalizedType := normalizeDatasetType(input.Type)
	normalizedScoring := normalizeScoringMode(input.ScoringMode)

	policy := input.ExecutionPolicy
	if isZeroExecutionPolicy(policy) {
		policy = DefaultExecutionPolicy(normalizedType)
	}

	validatorDefault := input.ValidatorDefault
	if strings.TrimSpace(validatorDefault.Type) == "" {
		validatorDefault = DefaultValidatorConfig()
	}

	datasetModel := DatasetModel{
		ProblemID:            input.ProblemID,
		Type:                 normalizedType,
		ScoringMode:          normalizedScoring,
		ExecutionPolicyJSON:  SerializeExecutionPolicy(policy),
		ValidatorDefaultJSON: SerializeValidatorConfig(validatorDefault),
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
	}

	if err := s.db.Create(&datasetModel).Error; err != nil {
		return nil, err
	}

	return &datasetModel, nil
}

type DatasetUpdateInput struct {
	DatasetID        uint
	Type             string
	ScoringMode      string
	ExecutionPolicy  *ExecutionPolicy
	ValidatorDefault *ValidatorConfig
}

func (s *Store) UpdateDataset(input DatasetUpdateInput) (*DatasetModel, error) {
	if input.DatasetID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	datasetModel := DatasetModel{}
	if err := s.db.Where("id = ?", input.DatasetID).
		First(&datasetModel).Error; err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if strings.TrimSpace(input.Type) != "" {
		updates["type"] = normalizeDatasetType(input.Type)
	}
	if strings.TrimSpace(input.ScoringMode) != "" {
		updates["scoring_mode"] = normalizeScoringMode(input.ScoringMode)
	}
	if input.ExecutionPolicy != nil {
		updates["execution_policy_json"] = SerializeExecutionPolicy(*input.ExecutionPolicy)
	}
	if input.ValidatorDefault != nil {
		updates["validator_default_json"] = SerializeValidatorConfig(*input.ValidatorDefault)
	}

	if len(updates) == 0 {
		return &datasetModel, nil
	}

	updates["updated_at"] = time.Now().UTC()
	if err := s.db.Model(&datasetModel).Updates(updates).Error; err != nil {
		return nil, err
	}

	return &datasetModel, nil
}

type DatasetLookupInput struct {
	DatasetID uint
}

func (s *Store) GetDatasetByID(input DatasetLookupInput) (*DatasetModel, error) {
	if input.DatasetID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	datasetModel := DatasetModel{}
	if err := s.db.Where("id = ?", input.DatasetID).
		First(&datasetModel).Error; err != nil {
		return nil, err
	}

	return &datasetModel, nil
}

type DatasetListInput struct {
	ProblemID uint
	Type      string
}

type DatasetListOutput struct {
	Datasets []DatasetModel
}

func (s *Store) ListDatasets(input DatasetListInput) (DatasetListOutput, error) {
	if input.ProblemID == 0 {
		return DatasetListOutput{Datasets: []DatasetModel{}}, nil
	}

	datasetQuery := s.db.Where("problem_id = ?", input.ProblemID)
	if strings.TrimSpace(input.Type) != "" {
		datasetQuery = datasetQuery.Where("type = ?", strings.TrimSpace(input.Type))
	}

	datasetModels := []DatasetModel{}
	if err := datasetQuery.Order("id asc").
		Find(&datasetModels).Error; err != nil {
		return DatasetListOutput{}, err
	}

	return DatasetListOutput{Datasets: datasetModels}, nil
}

type DatasetDeleteInput struct {
	DatasetID uint
}

func (s *Store) DeleteDataset(input DatasetDeleteInput) error {
	if input.DatasetID == 0 {
		return gorm.ErrRecordNotFound
	}

	return s.db.Delete(&DatasetModel{}, input.DatasetID).Error
}

type TestcaseCreateInput struct {
	DatasetID         uint
	InputText         string
	ExpectedText      string
	Visibility        string
	Weight            int
	Group             string
	Position          int
	ValidatorOverride *ValidatorConfig
}

func (s *Store) CreateTestcase(input TestcaseCreateInput) (*TestcaseModel, error) {
	if input.DatasetID == 0 {
		return nil, gorm.ErrInvalidData
	}

	weight := normalizeWeight(input.Weight)
	group := normalizeTestcaseGroup(input.Group)
	visibility := normalizeTestcaseVisibility(input.Visibility)

	validatorJSON := "{}"
	if input.ValidatorOverride != nil {
		validatorJSON = SerializeValidatorConfig(*input.ValidatorOverride)
	}

	testcaseModel := TestcaseModel{
		DatasetID:             input.DatasetID,
		InputText:             strings.TrimSpace(input.InputText),
		ExpectedText:          strings.TrimSpace(input.ExpectedText),
		Visibility:            visibility,
		Weight:                weight,
		Group:                 group,
		Position:              input.Position,
		ValidatorOverrideJSON: validatorJSON,
		CreatedAt:             time.Now().UTC(),
		UpdatedAt:             time.Now().UTC(),
	}

	if err := s.db.Create(&testcaseModel).Error; err != nil {
		return nil, err
	}

	return &testcaseModel, nil
}

type TestcaseUpdateInput struct {
	TestcaseID        uint
	InputText         string
	ExpectedText      string
	Visibility        string
	Weight            *int
	Group             string
	Position          *int
	ValidatorOverride *ValidatorConfig
}

func (s *Store) UpdateTestcase(input TestcaseUpdateInput) (*TestcaseModel, error) {
	if input.TestcaseID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	testcaseModel := TestcaseModel{}
	if err := s.db.Where("id = ?", input.TestcaseID).
		First(&testcaseModel).Error; err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if strings.TrimSpace(input.InputText) != "" {
		updates["input_text"] = strings.TrimSpace(input.InputText)
	}
	if strings.TrimSpace(input.ExpectedText) != "" {
		updates["expected_text"] = strings.TrimSpace(input.ExpectedText)
	}
	if strings.TrimSpace(input.Visibility) != "" {
		updates["visibility"] = normalizeTestcaseVisibility(input.Visibility)
	}
	if input.Weight != nil {
		updates["weight"] = normalizeWeight(*input.Weight)
	}
	if strings.TrimSpace(input.Group) != "" {
		updates["group"] = normalizeTestcaseGroup(input.Group)
	}
	if input.Position != nil {
		updates["position"] = *input.Position
	}
	if input.ValidatorOverride != nil {
		updates["validator_override_json"] = SerializeValidatorConfig(*input.ValidatorOverride)
	}

	if len(updates) == 0 {
		return &testcaseModel, nil
	}

	updates["updated_at"] = time.Now().UTC()
	if err := s.db.Model(&testcaseModel).Updates(updates).Error; err != nil {
		return nil, err
	}

	return &testcaseModel, nil
}

type TestcaseLookupInput struct {
	TestcaseID uint
}

func (s *Store) GetTestcaseByID(input TestcaseLookupInput) (*TestcaseModel, error) {
	if input.TestcaseID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	testcaseModel := TestcaseModel{}
	if err := s.db.Where("id = ?", input.TestcaseID).
		First(&testcaseModel).Error; err != nil {
		return nil, err
	}

	return &testcaseModel, nil
}

type TestcaseListInput struct {
	DatasetID uint
}

type TestcaseListOutput struct {
	Testcases []TestcaseModel
}

func (s *Store) ListTestcases(input TestcaseListInput) (TestcaseListOutput, error) {
	if input.DatasetID == 0 {
		return TestcaseListOutput{Testcases: []TestcaseModel{}}, nil
	}

	testcaseModels := []TestcaseModel{}
	if err := s.db.Where("dataset_id = ?", input.DatasetID).
		Order("`group` asc, position asc, id asc").
		Find(&testcaseModels).Error; err != nil {
		return TestcaseListOutput{}, err
	}

	return TestcaseListOutput{Testcases: testcaseModels}, nil
}

type TestcaseDeleteInput struct {
	TestcaseID uint
}

func (s *Store) DeleteTestcase(input TestcaseDeleteInput) error {
	if input.TestcaseID == 0 {
		return gorm.ErrRecordNotFound
	}

	return s.db.Delete(&TestcaseModel{}, input.TestcaseID).Error
}
