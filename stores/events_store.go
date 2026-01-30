package stores

import (
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"
)

type EventInput struct {
	EventType  string
	EntityType string
	EntityID   *uint
	Metadata   map[string]interface{}
	CreatedAt  *time.Time
}

type EventBatchInput struct {
	UserID *uint
	AnonID string
	Events []EventInput
	Now    time.Time
}

func (s *Store) InsertEvents(input EventBatchInput) error {
	if len(input.Events) == 0 {
		return nil
	}

	anonID := strings.TrimSpace(input.AnonID)
	rows := make([]EventModel, 0, len(input.Events))
	impressions := make([]PostImpressionInput, 0, len(input.Events))

	for _, evt := range input.Events {
		createdAt := input.Now
		if evt.CreatedAt != nil {
			createdAt = *evt.CreatedAt
		}

		payload := ""
		if len(evt.Metadata) > 0 {
			if raw, err := json.Marshal(evt.Metadata); err == nil {
				payload = string(raw)
			}
		}

		row := EventModel{
			UserID:     input.UserID,
			AnonID:     nil,
			EventType:  strings.TrimSpace(evt.EventType),
			EntityType: strings.TrimSpace(evt.EntityType),
			EntityID:   evt.EntityID,
			Metadata:   payload,
			CreatedAt:  createdAt,
		}
		if anonID != "" {
			row.AnonID = &anonID
		}
		rows = append(rows, row)

		if shouldTrackPostImpression(evt, row) {
			dayDate := time.Date(createdAt.Year(), createdAt.Month(), createdAt.Day(), 0, 0, 0, 0, time.UTC)
			viewer := resolveImpressionViewer(input.UserID, anonID)
			impressions = append(impressions, PostImpressionInput{
				PostID:    *evt.EntityID,
				UserID:    viewer.UserID,
				AnonID:    viewer.AnonID,
				DayDate:   dayDate,
				CreatedAt: createdAt,
			})
		}
	}

	if err := s.db.Create(&rows).Error; err != nil {
		return err
	}
	if err := s.UpsertPostImpressions(impressions); err != nil {
		return err
	}
	return nil
}

type impressionViewer struct {
	UserID *uint
	AnonID *string
}

func resolveImpressionViewer(userID *uint, anonID string) impressionViewer {
	if userID != nil && *userID != 0 {
		return impressionViewer{UserID: userID, AnonID: nil}
	}
	if strings.TrimSpace(anonID) == "" {
		return impressionViewer{}
	}
	return impressionViewer{UserID: nil, AnonID: &anonID}
}

func shouldTrackPostImpression(evt EventInput, row EventModel) bool {
	if evt.EntityID == nil || *evt.EntityID == 0 {
		return false
	}
	if strings.ToUpper(strings.TrimSpace(evt.EntityType)) != "POST" {
		return false
	}
	if strings.ToLower(strings.TrimSpace(evt.EventType)) != "post_open" {
		return false
	}
	return !row.CreatedAt.IsZero()
}

type EventsWindowInput struct {
	UserID uint
	AnonID string
	Since  time.Time
}

func (s *Store) ListEventsInWindow(input EventsWindowInput) ([]EventModel, error) {
	if input.UserID == 0 && strings.TrimSpace(input.AnonID) == "" {
		return []EventModel{}, gorm.ErrRecordNotFound
	}

	query := s.db.Model(&EventModel{}).Where("created_at >= ?", input.Since)
	if input.UserID != 0 {
		query = query.Where("user_id = ?", input.UserID)
	} else {
		query = query.Where("anon_id = ?", strings.TrimSpace(input.AnonID))
	}

	rows := []EventModel{}
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Store) MergeAnonEvents(anonID string, userID uint) error {
	anonID = strings.TrimSpace(anonID)
	if anonID == "" || userID == 0 {
		return nil
	}

	updates := map[string]interface{}{
		"user_id":    userID,
		"updated_at": time.Now().UTC(),
	}
	return s.db.Model(&EventModel{}).Where("anon_id = ? AND user_id IS NULL", anonID).Updates(updates).Error
}
