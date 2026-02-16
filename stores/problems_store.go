package stores

import (
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"
)

type ProblemLookupInput struct {
	Slug string
}

func (s *Store) GetProblemBySlug(input ProblemLookupInput) (*ProblemModel, error) {
	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		return nil, gorm.ErrRecordNotFound
	}

	problemModel := ProblemModel{}
	if err := s.db.Where("slug = ?", slug).First(&problemModel).Error; err != nil {
		return nil, err
	}

	return &problemModel, nil
}

type ProblemByIDLookupInput struct {
	ProblemID uint
}

func (s *Store) GetProblemByID(input ProblemByIDLookupInput) (*ProblemModel, error) {
	if input.ProblemID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	problemModel := ProblemModel{}
	if err := s.db.Where("id = ?", input.ProblemID).First(&problemModel).Error; err != nil {
		return nil, err
	}

	return &problemModel, nil
}

type ProblemListInput struct {
	Status     string
	Difficulty string
	Query      string
	Page       int
	PageSize   int
}

type ProblemListOutput struct {
	Problems []ProblemModel
	Total    int64
}

func (s *Store) ListProblems(input ProblemListInput) (ProblemListOutput, error) {
	return s.fetchProblemList(input)
}

func (s *Store) fetchProblemList(input ProblemListInput) (ProblemListOutput, error) {
	problemQuery := buildProblemListQuery(s.db, input)

	totalProblems, err := countProblemTotal(problemQuery)
	if err != nil {
		return ProblemListOutput{}, err
	}

	pageSize := clampPageSize(input.PageSize)
	offset := clampPage(input.Page) * pageSize

	problemModels := []ProblemModel{}
	if err := problemQuery.Order("published_at desc, id desc").Limit(pageSize).Offset(offset).Find(&problemModels).Error; err != nil {
		return ProblemListOutput{}, err
	}

	return ProblemListOutput{Problems: problemModels, Total: totalProblems}, nil
}

func buildProblemListQuery(db *gorm.DB, input ProblemListInput) *gorm.DB {
	problemQuery := db.Model(&ProblemModel{})
	status := strings.TrimSpace(input.Status)
	if status != "" {
		problemQuery = problemQuery.Where("status = ?", status)
	}

	difficulty := strings.TrimSpace(input.Difficulty)
	if difficulty != "" {
		problemQuery = problemQuery.Where("difficulty = ?", difficulty)
	}

	searchQuery := strings.TrimSpace(input.Query)
	if searchQuery == "" {
		return problemQuery
	}

	likePattern := "%" + searchQuery + "%"
	return problemQuery.Where("title LIKE ? OR slug LIKE ?", likePattern, likePattern)
}

func countProblemTotal(problemQuery *gorm.DB) (int64, error) {
	var totalProblems int64
	if err := problemQuery.Count(&totalProblems).Error; err != nil {
		return 0, err
	}

	return totalProblems, nil
}

type ProblemCoreInput struct {
	Slug                  string
	Title                 string
	Difficulty            string
	Status                string
	Statement             ProblemStatement
	IOSpecJSON            json.RawMessage
	ConstraintsJSON       json.RawMessage
	Tags                  []string
	Editorial             ProblemEditorial
	OfficialSolutionsJSON json.RawMessage
}

type ProblemCreateInput struct {
	ProblemCoreInput
	CreatedBy   uint
	PublishedAt *time.Time
}

func (s *Store) CreateProblem(input ProblemCreateInput) (*ProblemModel, error) {
	normalizedInput, err := normalizeProblemCreateInput(input)
	if err != nil {
		return nil, err
	}

	problemModel := buildProblemModel(normalizedInput)
	if err := s.db.Create(&problemModel).Error; err != nil {
		return nil, err
	}

	return &problemModel, nil
}

type ProblemUpdateInput struct {
	ProblemID uint
	ProblemCoreInput
	PublishedAt *time.Time
}

func (s *Store) UpdateProblem(input ProblemUpdateInput) (*ProblemModel, error) {
	if input.ProblemID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	problemModel, err := s.GetProblemByID(ProblemByIDLookupInput{ProblemID: input.ProblemID})
	if err != nil {
		return nil, err
	}

	normalizedInput, err := normalizeProblemUpdateInput(input)
	if err != nil {
		return nil, err
	}

	updates := buildProblemUpdates(normalizedInput)
	if err := s.db.Model(problemModel).Updates(updates).Error; err != nil {
		return nil, err
	}

	updatedProblem, err := s.GetProblemByID(ProblemByIDLookupInput{ProblemID: input.ProblemID})
	if err != nil {
		return nil, err
	}

	return updatedProblem, nil
}

type ProblemDeleteInput struct {
	ProblemID uint
}

func (s *Store) DeleteProblem(input ProblemDeleteInput) error {
	if input.ProblemID == 0 {
		return gorm.ErrRecordNotFound
	}

	return s.db.Delete(&ProblemModel{}, input.ProblemID).Error
}

func normalizeProblemCreateInput(input ProblemCreateInput) (ProblemCreateInput, error) {
	normalizedCore, err := normalizeProblemCoreInput(input.ProblemCoreInput)
	if err != nil {
		return ProblemCreateInput{}, err
	}

	normalizedInput := ProblemCreateInput{
		ProblemCoreInput: normalizedCore,
		CreatedBy:        input.CreatedBy,
		PublishedAt:      input.PublishedAt,
	}

	return normalizedInput, nil
}

func normalizeProblemUpdateInput(input ProblemUpdateInput) (ProblemUpdateInput, error) {
	normalizedCore, err := normalizeProblemCoreInput(input.ProblemCoreInput)
	if err != nil {
		return ProblemUpdateInput{}, err
	}

	normalizedInput := ProblemUpdateInput{
		ProblemID:        input.ProblemID,
		ProblemCoreInput: normalizedCore,
		PublishedAt:      input.PublishedAt,
	}

	return normalizedInput, nil
}

func normalizeProblemCoreInput(input ProblemCoreInput) (ProblemCoreInput, error) {
	slug := strings.TrimSpace(input.Slug)

	title := strings.TrimSpace(input.Title)
	if slug == "" || title == "" {
		return ProblemCoreInput{}, gorm.ErrInvalidData
	}

	normalized := ProblemCoreInput{
		Slug:                  slug,
		Title:                 title,
		Difficulty:            normalizeProblemDifficulty(input.Difficulty),
		Status:                normalizeProblemStatus(input.Status),
		Statement:             normalizeProblemStatement(input.Statement),
		IOSpecJSON:            input.IOSpecJSON,
		ConstraintsJSON:       input.ConstraintsJSON,
		Tags:                  cleanStringList(input.Tags),
		Editorial:             normalizeProblemEditorial(input.Editorial),
		OfficialSolutionsJSON: input.OfficialSolutionsJSON,
	}

	return normalized, nil
}

func buildProblemModel(input ProblemCreateInput) ProblemModel {
	now := time.Now().UTC()

	return ProblemModel{
		Slug:            input.Slug,
		Title:           input.Title,
		Difficulty:      input.Difficulty,
		Status:          input.Status,
		StatementJSON:   SerializeProblemStatement(input.Statement),
		IOSpecJSON:      resolveProblemRawJSON(input.IOSpecJSON, "{}"),
		ConstraintsJSON: resolveProblemRawJSON(input.ConstraintsJSON, "{}"),
		TagsJSON:        SerializeProblemTags(input.Tags),
		EditorialJSON:   SerializeProblemEditorial(input.Editorial),
		SolutionsJSON:   resolveProblemRawJSON(input.OfficialSolutionsJSON, "[]"),
		CreatedBy:       input.CreatedBy,
		PublishedAt:     input.PublishedAt,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func buildProblemUpdates(input ProblemUpdateInput) map[string]interface{} {
	return map[string]interface{}{
		"slug":             input.Slug,
		"title":            input.Title,
		"difficulty":       input.Difficulty,
		"status":           input.Status,
		"statement_json":   SerializeProblemStatement(input.Statement),
		"io_spec_json":     resolveProblemRawJSON(input.IOSpecJSON, "{}"),
		"constraints_json": resolveProblemRawJSON(input.ConstraintsJSON, "{}"),
		"tags_json":        SerializeProblemTags(input.Tags),
		"editorial_json":   SerializeProblemEditorial(input.Editorial),
		"solutions_json":   resolveProblemRawJSON(input.OfficialSolutionsJSON, "[]"),
		"published_at":     input.PublishedAt,
		"updated_at":       time.Now().UTC(),
	}
}

func resolveProblemRawJSON(raw json.RawMessage, fallback string) string {
	if len(raw) == 0 {
		return fallback
	}
	return string(raw)
}
