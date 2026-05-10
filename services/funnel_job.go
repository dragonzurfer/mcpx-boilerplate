package services

import (
	"errors"
	"log"
	"time"

	"github.com/mcpx/boilerplate/stores"
	"gorm.io/gorm"
)

type FunnelService struct {
	Store  *stores.Store
	Logger *log.Logger
}

func (s *FunnelService) RecalculateAll(now time.Time) error {
	config, err := s.Store.GetFunnelConfig()
	if err != nil {
		return err
	}
	weights, err := s.loadWeights()
	if err != nil {
		return err
	}
	thresholds, err := s.loadThresholds()
	if err != nil {
		return err
	}

	windowStart := now.AddDate(0, 0, -config.ScoringWindowDays)
	page := 0
	for {
		ids, err := s.Store.ListUserIDs(page, 200)
		if err != nil {
			return err
		}
		if len(ids) == 0 {
			break
		}
		for _, userID := range ids {
			_ = s.recalculateUser(userID, now, windowStart, weights, thresholds, config)
		}
		page++
	}

	return nil
}

func (s *FunnelService) recalculateUser(userID uint, now, windowStart time.Time, weights map[string]int, thresholds []StageThreshold, config *stores.FunnelConfigModel) error {
	rows, err := s.Store.ListEventsInWindow(stores.EventsWindowInput{UserID: userID, Since: windowStart})
	if err != nil && !errorsIsNotFound(err) {
		return err
	}

	events := make([]FunnelEventInput, 0, len(rows))
	for _, row := range rows {
		events = append(events, FunnelEventInput{EventType: row.EventType, CreatedAt: row.CreatedAt})
	}

	entitlementStatus := resolveEntitlementStatus(s.Store, userID, now)
	output := ComputeFunnelScore(FunnelScoreInput{
		Now:                  now,
		Events:               events,
		Weights:              weights,
		StageThresholds:      thresholds,
		EntitlementStatus:    entitlementStatus,
		DormantDaysThreshold: config.DormantDaysThreshold,
		DecayEnabled:         config.DecayEnabled,
		DailyDecayFactor:     config.DailyDecayFactor,
	})

	metrics := stores.UserMetricsModel{
		UserID:       userID,
		Score:        output.Score,
		Stage:        output.Stage,
		LastActiveAt: output.LastActiveAt,
		Reads14d:     output.Reads,
		Completes14d: output.Completes,
	}
	return s.Store.UpsertUserMetrics(metrics)
}

func (s *FunnelService) loadWeights() (map[string]int, error) {
	rows, err := s.Store.ListFunnelEventWeights()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return defaultWeights(), nil
	}

	weights := map[string]int{}
	for _, row := range rows {
		if row.Enabled {
			eventType := stores.CanonicalFunnelEventType(row.EventType)
			weights[eventType] = row.Weight
		}
	}
	return weights, nil
}

func (s *FunnelService) loadThresholds() ([]StageThreshold, error) {
	rows, err := s.Store.ListFunnelStageThresholds()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return defaultThresholds(), nil
	}

	thresholds := make([]StageThreshold, 0, len(rows))
	for _, row := range rows {
		thresholds = append(thresholds, StageThreshold{
			Stage:    row.Stage,
			MinScore: row.MinScore,
			MaxScore: row.MaxScore,
			Enabled:  row.Enabled,
		})
	}
	return thresholds, nil
}

func defaultWeights() map[string]int {
	weights := map[string]int{}
	for _, row := range stores.DefaultFunnelEventWeights() {
		if row.Enabled {
			eventType := stores.CanonicalFunnelEventType(row.EventType)
			weights[eventType] = row.Weight
		}
	}
	return weights
}

func defaultThresholds() []StageThreshold {
	rows := stores.DefaultFunnelStageThresholds()
	thresholds := make([]StageThreshold, 0, len(rows))
	for _, row := range rows {
		thresholds = append(thresholds, StageThreshold{
			Stage:    row.Stage,
			MinScore: row.MinScore,
			MaxScore: row.MaxScore,
			Enabled:  row.Enabled,
		})
	}
	return thresholds
}

func resolveEntitlementStatus(store *stores.Store, userID uint, now time.Time) EntitlementStatus {
	_, err := store.GetActiveEntitlement(stores.EntitlementLookupInput{UserID: userID, Now: now})
	if err == nil {
		return EntitlementActive
	}
	if errorsIsNotFound(err) {
		return EntitlementNone
	}

	latest, latestErr := store.GetLatestEntitlement(userID)
	if latestErr != nil {
		return EntitlementNone
	}
	if latest.Status == stores.EntitlementStatusExpired {
		return EntitlementExpired
	}
	return EntitlementNone
}

func errorsIsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
