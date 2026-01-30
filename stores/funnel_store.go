package stores

import (
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type FunnelConfigUpdateInput struct {
	ScoringWindowDays    int
	DecayEnabled         bool
	DailyDecayFactor     float64
	DormantDaysThreshold int
}

func (s *Store) GetFunnelConfig() (*FunnelConfigModel, error) {
	cfg := FunnelConfigModel{}
	err := s.db.First(&cfg).Error
	if err == nil {
		return &cfg, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	cfg = FunnelConfigModel{
		ScoringWindowDays:    14,
		DecayEnabled:         false,
		DailyDecayFactor:     0.9,
		DormantDaysThreshold: 21,
		UpdatedAt:            time.Now().UTC(),
	}
	if err := s.db.Create(&cfg).Error; err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (s *Store) UpdateFunnelConfig(input FunnelConfigUpdateInput) (*FunnelConfigModel, error) {
	cfg, err := s.GetFunnelConfig()
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"scoring_window_days":    input.ScoringWindowDays,
		"decay_enabled":          input.DecayEnabled,
		"daily_decay_factor":     input.DailyDecayFactor,
		"dormant_days_threshold": input.DormantDaysThreshold,
		"updated_at":             time.Now().UTC(),
	}

	if err := s.db.Model(cfg).Updates(updates).Error; err != nil {
		return nil, err
	}
	return cfg, nil
}

func (s *Store) ListFunnelEventWeights() ([]FunnelEventWeightModel, error) {
	rows := []FunnelEventWeightModel{}
	if err := s.db.Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) > 0 {
		return rows, nil
	}

	defaults := DefaultFunnelEventWeights()
	if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&defaults).Error; err != nil {
		return nil, err
	}
	return defaults, nil
}

func (s *Store) UpsertFunnelEventWeight(weight FunnelEventWeightModel) error {
	if weight.EventType == "" {
		return gorm.ErrInvalidData
	}
	return s.db.Save(&weight).Error
}

func (s *Store) ListFunnelStageThresholds() ([]FunnelStageThresholdModel, error) {
	rows := []FunnelStageThresholdModel{}
	if err := s.db.Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) > 0 {
		return rows, nil
	}

	defaults := DefaultFunnelStageThresholds()
	if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&defaults).Error; err != nil {
		return nil, err
	}
	return defaults, nil
}

func (s *Store) UpsertFunnelStageThreshold(threshold FunnelStageThresholdModel) error {
	if threshold.Stage == "" {
		return gorm.ErrInvalidData
	}
	return s.db.Save(&threshold).Error
}

func (s *Store) UpsertUserMetrics(metrics UserMetricsModel) error {
	if metrics.UserID == 0 {
		return gorm.ErrInvalidData
	}
	metrics.UpdatedAt = time.Now().UTC()
	return s.db.Save(&metrics).Error
}

func (s *Store) GetUserMetrics(userID uint) (*UserMetricsModel, error) {
	if userID == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	metrics := UserMetricsModel{}
	if err := s.db.Where("user_id = ?", userID).First(&metrics).Error; err != nil {
		return nil, err
	}
	return &metrics, nil
}
