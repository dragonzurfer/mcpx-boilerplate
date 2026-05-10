package stores

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrNoQueuedSubmission = errors.New("no queued submissions")

type SubmissionCreateInput struct {
	UserID     uint
	ProblemID  uint
	Language   string
	Mode       string
	DatasetID  uint
	Status     string
	CodeText   string
	CodeHash   string
	LimitsJSON string
	QueuedAt   time.Time
}

func (s *Store) CreateSubmission(input SubmissionCreateInput) (*SubmissionModel, error) {
	if input.UserID == 0 || input.ProblemID == 0 {
		return nil, gorm.ErrInvalidData
	}

	language := strings.ToLower(strings.TrimSpace(input.Language))
	mode := strings.ToUpper(strings.TrimSpace(input.Mode))
	status := strings.ToUpper(strings.TrimSpace(input.Status))

	submission := SubmissionModel{
		UserID:     input.UserID,
		ProblemID:  input.ProblemID,
		Language:   language,
		Mode:       mode,
		DatasetID:  input.DatasetID,
		Status:     status,
		CodeText:   input.CodeText,
		CodeHash:   strings.TrimSpace(input.CodeHash),
		LimitsJSON: strings.TrimSpace(input.LimitsJSON),
		QueuedAt:   input.QueuedAt,
		CreatedAt:  time.Now().UTC(),
		UpdatedAt:  time.Now().UTC(),
	}

	if err := s.db.Create(&submission).Error; err != nil {
		return nil, err
	}

	return &submission, nil
}

type SubmissionLookupInput struct {
	SubmissionID uint
}

func (s *Store) GetSubmissionByID(input SubmissionLookupInput) (*SubmissionModel, error) {
	if input.SubmissionID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	submission := SubmissionModel{}
	if err := s.db.Where("id = ?", input.SubmissionID).First(&submission).Error; err != nil {
		return nil, err
	}

	return &submission, nil
}

type SubmissionListInput struct {
	UserID    uint
	ProblemID uint
	Limit     int
}

type SubmissionListOutput struct {
	Submissions []SubmissionModel
}

func (s *Store) ListSubmissions(input SubmissionListInput) (SubmissionListOutput, error) {
	if input.UserID == 0 {
		return SubmissionListOutput{Submissions: []SubmissionModel{}}, nil
	}

	limit := clampPageSize(input.Limit)

	query := s.db.Where("user_id = ?", input.UserID)
	if input.ProblemID != 0 {
		query = query.Where("problem_id = ?", input.ProblemID)
	}

	submissions := []SubmissionModel{}
	if err := query.Order("queued_at desc, id desc").Limit(limit).Find(&submissions).Error; err != nil {
		return SubmissionListOutput{}, err
	}

	return SubmissionListOutput{Submissions: submissions}, nil
}

type UserSubmissionCountInput struct {
	UserID uint
}

type UserSubmissionCountOutput struct {
	Count int64
}

func (s *Store) CountUserSubmissions(input UserSubmissionCountInput) (UserSubmissionCountOutput, error) {
	if input.UserID == 0 {
		return UserSubmissionCountOutput{}, nil
	}

	var submissionCount int64
	query := s.db.Model(&SubmissionModel{}).Where("user_id = ?", input.UserID)
	if err := query.Count(&submissionCount).Error; err != nil {
		return UserSubmissionCountOutput{}, err
	}

	return UserSubmissionCountOutput{Count: submissionCount}, nil
}

type UserActiveSubmissionCountInput struct {
	UserID uint
}

type UserActiveSubmissionCountOutput struct {
	Count int64
}

func (s *Store) CountUserActiveSubmissions(input UserActiveSubmissionCountInput) (UserActiveSubmissionCountOutput, error) {
	if input.UserID == 0 {
		return UserActiveSubmissionCountOutput{}, nil
	}

	activeStatuses := []string{
		SubmissionStatusQueued,
		SubmissionStatusRunning,
	}

	var activeSubmissionCount int64
	query := s.db.Model(&SubmissionModel{}).
		Where("user_id = ?", input.UserID).
		Where("status IN ?", activeStatuses)
	if err := query.Count(&activeSubmissionCount).Error; err != nil {
		return UserActiveSubmissionCountOutput{}, err
	}

	return UserActiveSubmissionCountOutput{Count: activeSubmissionCount}, nil
}

type UserRecentSubmissionCountInput struct {
	UserID uint
	Since  time.Time
}

type UserRecentSubmissionCountOutput struct {
	Count int64
}

func (s *Store) CountUserSubmissionsSince(input UserRecentSubmissionCountInput) (UserRecentSubmissionCountOutput, error) {
	if input.UserID == 0 || input.Since.IsZero() {
		return UserRecentSubmissionCountOutput{}, nil
	}

	var submissionCount int64
	query := s.db.Model(&SubmissionModel{}).
		Where("user_id = ?", input.UserID).
		Where("queued_at >= ?", input.Since)
	if err := query.Count(&submissionCount).Error; err != nil {
		return UserRecentSubmissionCountOutput{}, err
	}

	return UserRecentSubmissionCountOutput{Count: submissionCount}, nil
}

type SubmissionStatusUpdateInput struct {
	SubmissionID uint
	Status       string
	StartedAt    *time.Time
	FinishedAt   *time.Time
}

func (s *Store) UpdateSubmissionStatus(input SubmissionStatusUpdateInput) error {
	if input.SubmissionID == 0 {
		return gorm.ErrRecordNotFound
	}

	updates := map[string]interface{}{}
	if strings.TrimSpace(input.Status) != "" {
		updates["status"] = strings.ToUpper(strings.TrimSpace(input.Status))
	}
	if input.StartedAt != nil {
		updates["started_at"] = input.StartedAt
	}
	if input.FinishedAt != nil {
		updates["finished_at"] = input.FinishedAt
	}

	if len(updates) == 0 {
		return nil
	}

	updates["updated_at"] = time.Now().UTC()
	return s.db.Model(&SubmissionModel{}).Where("id = ?", input.SubmissionID).Updates(updates).Error
}

type SubmissionClaimInput struct {
	Now time.Time
}

func (s *Store) ClaimNextQueuedSubmission(input SubmissionClaimInput) (*SubmissionModel, error) {
	submission := SubmissionModel{}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		query := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		if err := query.Where("status = ?", SubmissionStatusQueued).
			Order("queued_at asc, id asc").
			Limit(1).
			Find(&submission).Error; err != nil {
			return err
		}
		if submission.ID == 0 {
			return ErrNoQueuedSubmission
		}

		startedAt := input.Now
		updates := map[string]interface{}{
			"status":     SubmissionStatusRunning,
			"started_at": &startedAt,
			"updated_at": time.Now().UTC(),
		}
		return tx.Model(&SubmissionModel{}).Where("id = ?", submission.ID).Updates(updates).Error
	})
	if err != nil {
		return nil, err
	}

	return &submission, nil
}
