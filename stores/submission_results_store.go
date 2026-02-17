package stores

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type SubmissionResultUpsertInput struct {
	SubmissionID  uint
	OverallJSON   string
	CompileJSON   string
	TestsJSON     string
	TimingJSON    string
	ArtifactsJSON string
	ReceiptJSON   string
}

func (s *Store) UpsertSubmissionResult(input SubmissionResultUpsertInput) (*SubmissionResultModel, error) {
	if input.SubmissionID == 0 {
		return nil, gorm.ErrInvalidData
	}

	result := SubmissionResultModel{}

	lookup := s.db.Where("submission_id = ?", input.SubmissionID).First(&result)

	if lookup.Error == nil {
		updates := map[string]interface{}{
			"overall_json":   strings.TrimSpace(input.OverallJSON),
			"compile_json":   strings.TrimSpace(input.CompileJSON),
			"tests_json":     strings.TrimSpace(input.TestsJSON),
			"timing_json":    strings.TrimSpace(input.TimingJSON),
			"artifacts_json": strings.TrimSpace(input.ArtifactsJSON),
			"receipt_json":   strings.TrimSpace(input.ReceiptJSON),
			"updated_at":     time.Now().UTC(),
		}
		if err := s.db.Model(&result).Updates(updates).Error; err != nil {
			return nil, err
		}
		return &result, nil
	}

	if lookup.Error != nil && !errors.Is(lookup.Error, gorm.ErrRecordNotFound) {
		return nil, lookup.Error
	}

	result = SubmissionResultModel{
		SubmissionID:  input.SubmissionID,
		OverallJSON:   strings.TrimSpace(input.OverallJSON),
		CompileJSON:   strings.TrimSpace(input.CompileJSON),
		TestsJSON:     strings.TrimSpace(input.TestsJSON),
		TimingJSON:    strings.TrimSpace(input.TimingJSON),
		ArtifactsJSON: strings.TrimSpace(input.ArtifactsJSON),
		ReceiptJSON:   strings.TrimSpace(input.ReceiptJSON),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}

	if err := s.db.Create(&result).Error; err != nil {
		return nil, err
	}

	return &result, nil
}

type SubmissionResultLookupInput struct {
	SubmissionID uint
}

func (s *Store) GetSubmissionResult(input SubmissionResultLookupInput) (*SubmissionResultModel, error) {
	if input.SubmissionID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	result := SubmissionResultModel{}
	if err := s.db.Where("submission_id = ?", input.SubmissionID).
		First(&result).Error; err != nil {
		return nil, err
	}

	return &result, nil
}
