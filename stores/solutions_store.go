package stores

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type SolutionCreateInput struct {
	ProblemID       uint
	Language        string
	CodeText        string
	ComplexityJSON  string
	ApproachSummary string
	IsReference     bool
}

func (s *Store) CreateSolution(input SolutionCreateInput) (*SolutionModel, error) {
	if input.ProblemID == 0 {
		return nil, gorm.ErrInvalidData
	}

	language := strings.ToLower(strings.TrimSpace(input.Language))
	if language == "" {
		return nil, gorm.ErrInvalidData
	}

	solution := SolutionModel{
		ProblemID:       input.ProblemID,
		Language:        language,
		CodeText:        input.CodeText,
		ComplexityJSON:  strings.TrimSpace(input.ComplexityJSON),
		ApproachSummary: strings.TrimSpace(input.ApproachSummary),
		IsReference:     input.IsReference,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	if err := s.db.Create(&solution).Error; err != nil {
		return nil, err
	}

	return &solution, nil
}

type SolutionUpdateInput struct {
	SolutionID      uint
	Language        string
	CodeText        string
	ComplexityJSON  string
	ApproachSummary string
	IsReference     *bool
}

func (s *Store) UpdateSolution(input SolutionUpdateInput) (*SolutionModel, error) {
	if input.SolutionID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	solution := SolutionModel{}
	if err := s.db.Where("id = ?", input.SolutionID).
		First(&solution).Error; err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if strings.TrimSpace(input.Language) != "" {
		updates["language"] = strings.ToLower(strings.TrimSpace(input.Language))
	}
	if strings.TrimSpace(input.CodeText) != "" {
		updates["code_text"] = input.CodeText
	}
	if strings.TrimSpace(input.ComplexityJSON) != "" {
		updates["complexity_json"] = strings.TrimSpace(input.ComplexityJSON)
	}
	if strings.TrimSpace(input.ApproachSummary) != "" {
		updates["approach_summary"] = strings.TrimSpace(input.ApproachSummary)
	}
	if input.IsReference != nil {
		updates["is_reference"] = *input.IsReference
	}

	if len(updates) == 0 {
		return &solution, nil
	}

	updates["updated_at"] = time.Now().UTC()
	if err := s.db.Model(&solution).Updates(updates).Error; err != nil {
		return nil, err
	}

	return &solution, nil
}

type SolutionLookupInput struct {
	SolutionID uint
}

func (s *Store) GetSolutionByID(input SolutionLookupInput) (*SolutionModel, error) {
	if input.SolutionID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	solution := SolutionModel{}
	if err := s.db.Where("id = ?", input.SolutionID).
		First(&solution).Error; err != nil {
		return nil, err
	}

	return &solution, nil
}

type SolutionListInput struct {
	ProblemID uint
}

type SolutionListOutput struct {
	Solutions []SolutionModel
}

func (s *Store) ListSolutions(input SolutionListInput) (SolutionListOutput, error) {
	if input.ProblemID == 0 {
		return SolutionListOutput{Solutions: []SolutionModel{}}, nil
	}

	solutions := []SolutionModel{}
	if err := s.db.Where("problem_id = ?", input.ProblemID).
		Order("id asc").
		Find(&solutions).Error; err != nil {
		return SolutionListOutput{}, err
	}

	return SolutionListOutput{Solutions: solutions}, nil
}

type SolutionDeleteInput struct {
	SolutionID uint
}

func (s *Store) DeleteSolution(input SolutionDeleteInput) error {
	if input.SolutionID == 0 {
		return gorm.ErrRecordNotFound
	}

	return s.db.Delete(&SolutionModel{}, input.SolutionID).Error
}
