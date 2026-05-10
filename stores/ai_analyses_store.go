package stores

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type AIAnalysisCreateInput struct {
	AnalysisID   string
	SubmissionID uint
	UserID       uint
	CodeHash     string
	ResultHash   string
	PolicyJSON   string
	Status       string
	ResponseJSON string
}

func (s *Store) CreateAIAnalysis(input AIAnalysisCreateInput) (*AIAnalysisModel, error) {
	if strings.TrimSpace(input.AnalysisID) == "" {
		return nil, gorm.ErrInvalidData
	}

	analysis := AIAnalysisModel{
		AnalysisID:   strings.TrimSpace(input.AnalysisID),
		SubmissionID: input.SubmissionID,
		UserID:       input.UserID,
		CodeHash:     strings.TrimSpace(input.CodeHash),
		ResultHash:   strings.TrimSpace(input.ResultHash),
		PolicyJSON:   strings.TrimSpace(input.PolicyJSON),
		Status:       strings.TrimSpace(input.Status),
		ResponseJSON: strings.TrimSpace(input.ResponseJSON),
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}

	if err := s.db.Create(&analysis).Error; err != nil {
		return nil, err
	}

	return &analysis, nil
}

type AIAnalysisUpdateInput struct {
	AnalysisID   string
	Status       string
	ResponseJSON string
}

func (s *Store) UpdateAIAnalysis(input AIAnalysisUpdateInput) (*AIAnalysisModel, error) {
	analysisID := strings.TrimSpace(input.AnalysisID)
	if analysisID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	analysis := AIAnalysisModel{}
	if err := s.db.Where("analysis_id = ?", analysisID).
		First(&analysis).Error; err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if strings.TrimSpace(input.Status) != "" {
		updates["status"] = strings.TrimSpace(input.Status)
	}
	if strings.TrimSpace(input.ResponseJSON) != "" {
		updates["response_json"] = strings.TrimSpace(input.ResponseJSON)
	}

	if len(updates) == 0 {
		return &analysis, nil
	}

	updates["updated_at"] = time.Now().UTC()
	if err := s.db.Model(&analysis).Updates(updates).Error; err != nil {
		return nil, err
	}

	return &analysis, nil
}

type AIAnalysisLookupInput struct {
	AnalysisID string
}

func (s *Store) GetAIAnalysisByID(input AIAnalysisLookupInput) (*AIAnalysisModel, error) {
	analysisID := strings.TrimSpace(input.AnalysisID)
	if analysisID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	analysis := AIAnalysisModel{}
	if err := s.db.Where("analysis_id = ?", analysisID).
		First(&analysis).Error; err != nil {
		return nil, err
	}

	return &analysis, nil
}

type AIAnalysisFingerprintInput struct {
	SubmissionID uint
	CodeHash     string
	ResultHash   string
	PolicyJSON   string
}

func (s *Store) FindAIAnalysisByFingerprint(input AIAnalysisFingerprintInput) (*AIAnalysisModel, error) {
	if input.SubmissionID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	analysis := AIAnalysisModel{}
	query := s.db.Where(
		"submission_id = ? AND code_hash = ? AND result_hash = ?",
		input.SubmissionID,
		strings.TrimSpace(input.CodeHash),
		strings.TrimSpace(input.ResultHash),
	)
	if strings.TrimSpace(input.PolicyJSON) != "" {
		query = query.Where("policy_json = ?", strings.TrimSpace(input.PolicyJSON))
	}

	if err := query.Order("created_at desc").
		First(&analysis).Error; err != nil {
		return nil, err
	}

	return &analysis, nil
}

type UserAIAnalysisCountInput struct {
	UserID uint
}

type UserAIAnalysisCountOutput struct {
	Count int64
}

func (s *Store) CountUserAIAnalyses(input UserAIAnalysisCountInput) (UserAIAnalysisCountOutput, error) {
	if input.UserID == 0 {
		return UserAIAnalysisCountOutput{}, nil
	}

	var analysisCount int64
	query := s.db.Model(&AIAnalysisModel{}).Where("user_id = ?", input.UserID)
	if err := query.Count(&analysisCount).Error; err != nil {
		return UserAIAnalysisCountOutput{}, err
	}

	return UserAIAnalysisCountOutput{Count: analysisCount}, nil
}
