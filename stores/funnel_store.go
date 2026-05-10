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
	key := funnelConfigKey()
	if cached, ok := s.getCachedFunnelConfig(key); ok {
		return cached, nil
	}

	cfg := FunnelConfigModel{}
	err := s.db.First(&cfg).Error
	if err == nil {
		s.cacheSet(key, &cfg)
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
	s.cacheSet(key, &cfg)
	return &cfg, nil
}

func (s *Store) UpdateFunnelConfig(input FunnelConfigUpdateInput) (*FunnelConfigModel, error) {
	cfg, err := s.GetFunnelConfig()
	if err != nil {
		return nil, err
	}

	updatedAt := time.Now().UTC()
	updates := map[string]interface{}{
		"scoring_window_days":    input.ScoringWindowDays,
		"decay_enabled":          input.DecayEnabled,
		"daily_decay_factor":     input.DailyDecayFactor,
		"dormant_days_threshold": input.DormantDaysThreshold,
		"updated_at":             updatedAt,
	}

	if err := s.db.Model(cfg).Updates(updates).Error; err != nil {
		return nil, err
	}

	cfg.ScoringWindowDays = input.ScoringWindowDays
	cfg.DecayEnabled = input.DecayEnabled
	cfg.DailyDecayFactor = input.DailyDecayFactor
	cfg.DormantDaysThreshold = input.DormantDaysThreshold
	cfg.UpdatedAt = updatedAt
	s.cacheSet(funnelConfigKey(), cfg)

	return cfg, nil
}

func (s *Store) ListFunnelEventWeights() ([]FunnelEventWeightModel, error) {
	key := funnelWeightsKey()
	if cached, ok := s.getCachedFunnelWeights(key); ok {
		return cached, nil
	}

	if err := s.ensureFunnelEventWeights(); err != nil {
		return nil, err
	}

	rows := []FunnelEventWeightModel{}
	if err := s.db.Order("event_type asc").Find(&rows).Error; err != nil {
		return nil, err
	}

	s.cacheSet(key, rows)
	return rows, nil
}

func (s *Store) UpsertFunnelEventWeight(weight FunnelEventWeightModel) error {
	if weight.EventType == "" {
		return gorm.ErrInvalidData
	}
	if err := s.db.Save(&weight).Error; err != nil {
		return err
	}
	s.cacheSet(funnelWeightsKey(), nil)
	return nil
}

func (s *Store) ensureFunnelEventWeights() error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := migrateLegacyFunnelWeights(tx); err != nil {
			return err
		}

		defaultWeights := DefaultFunnelEventWeights()
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&defaultWeights).Error; err != nil {
			return err
		}
		return nil
	})
}

type funnelEventTypeAlias struct {
	LegacyEventType    string
	CanonicalEventType string
}

func migrateLegacyFunnelWeights(tx *gorm.DB) error {
	for legacyEventType, canonicalEventType := range LegacyFunnelEventAliases() {
		alias := funnelEventTypeAlias{
			LegacyEventType:    legacyEventType,
			CanonicalEventType: canonicalEventType,
		}
		if err := migrateLegacyFunnelWeight(tx, alias); err != nil {
			return err
		}
	}
	return nil
}

func migrateLegacyFunnelWeight(tx *gorm.DB, alias funnelEventTypeAlias) error {
	legacyRow := FunnelEventWeightModel{}
	legacyResult := tx.Where("event_type = ?", alias.LegacyEventType).First(&legacyRow)
	if errors.Is(legacyResult.Error, gorm.ErrRecordNotFound) {
		return nil
	}
	if legacyResult.Error != nil {
		return legacyResult.Error
	}

	canonicalRow := FunnelEventWeightModel{}
	canonicalResult := tx.Where("event_type = ?", alias.CanonicalEventType).First(&canonicalRow)
	if errors.Is(canonicalResult.Error, gorm.ErrRecordNotFound) {
		canonicalRow = FunnelEventWeightModel{
			EventType: alias.CanonicalEventType,
			Weight:    legacyRow.Weight,
			Enabled:   legacyRow.Enabled,
		}
		if err := tx.Create(&canonicalRow).Error; err != nil {
			return err
		}
	} else if canonicalResult.Error != nil {
		return canonicalResult.Error
	}

	return tx.Where("event_type = ?", alias.LegacyEventType).Delete(&FunnelEventWeightModel{}).Error
}

func (s *Store) ListFunnelStageThresholds() ([]FunnelStageThresholdModel, error) {
	key := funnelStagesKey()
	if cached, ok := s.getCachedFunnelStages(key); ok {
		return cached, nil
	}

	rows := []FunnelStageThresholdModel{}
	if err := s.db.Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) > 0 {
		s.cacheSet(key, rows)
		return rows, nil
	}

	defaults := DefaultFunnelStageThresholds()
	if err := s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&defaults).Error; err != nil {
		return nil, err
	}
	s.cacheSet(key, defaults)
	return defaults, nil
}

func (s *Store) UpsertFunnelStageThreshold(threshold FunnelStageThresholdModel) error {
	if threshold.Stage == "" {
		return gorm.ErrInvalidData
	}
	if err := s.db.Save(&threshold).Error; err != nil {
		return err
	}
	s.cacheSet(funnelStagesKey(), nil)
	return nil
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
	key := userMetricsKey(userID)
	if cached, ok := s.getCachedUserMetrics(key); ok {
		return cached, nil
	}

	metrics := UserMetricsModel{}
	if err := s.db.Where("user_id = ?", userID).First(&metrics).Error; err != nil {
		return nil, err
	}
	s.cacheSet(key, &metrics)
	return &metrics, nil
}
