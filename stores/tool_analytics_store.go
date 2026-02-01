package stores

import (
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ToolAnalyticsRollupInput struct {
	Day time.Time
}

func (s *Store) RollupToolAnalytics(input ToolAnalyticsRollupInput) error {
	day := normalizeDay(input.Day)
	dayStart := day
	dayEnd := dayStart.Add(24 * time.Hour)

	events := []ToolEventModel{}
	if err := s.db.Where("created_at >= ? AND created_at < ?", dayStart, dayEnd).Find(&events).Error; err != nil {
		return err
	}

	metrics := map[toolMetricKey]*ToolDailyMetricModel{}
	for _, evt := range events {
		key := toolMetricKey{ToolID: evt.ToolID, EventName: normalizeToolEventName(evt.EventName)}
		metric := ensureToolMetric(metrics, key, day)
		metric.TotalCount++
		if key.EventName == "response" && readBoolFromMetadata(evt.Metadata, "used_audio") {
			metric.AudioCount++
		}
	}

	for _, metric := range metrics {
		metric.UpdatedAt = time.Now().UTC()
		if metric.CreatedAt.IsZero() {
			metric.CreatedAt = metric.UpdatedAt
		}
		if err := s.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "tool_id"}, {Name: "day_date"}, {Name: "event_name"}},
			DoUpdates: clause.AssignmentColumns(toolMetricUpdateColumns()),
		}).Create(metric).Error; err != nil {
			return err
		}
	}

	return nil
}

func (s *Store) RunToolAnalyticsRollup(now time.Time, retentionDays int) error {
	targetDay := normalizeDay(now.AddDate(0, 0, -1))
	lastDay, err := s.LatestToolAnalyticsDay()
	if err != nil {
		return err
	}

	startDay := targetDay
	if lastDay != nil {
		nextDay := normalizeDay(lastDay.AddDate(0, 0, 1))
		if nextDay.After(targetDay) {
			return nil
		}
		startDay = nextDay
	}

	for day := startDay; !day.After(targetDay); day = day.AddDate(0, 0, 1) {
		if err := s.RollupToolAnalytics(ToolAnalyticsRollupInput{Day: day}); err != nil {
			return err
		}
	}

	if retentionDays <= 0 {
		return nil
	}
	cutoff := normalizeDay(now.AddDate(0, 0, -retentionDays))
	return s.CleanupToolEvents(cutoff)
}

func (s *Store) LatestToolAnalyticsDay() (*time.Time, error) {
	row := ToolDailyMetricModel{}
	err := s.db.Model(&ToolDailyMetricModel{}).
		Select("day_date").
		Order("day_date desc").
		Limit(1).
		Find(&row).Error
	if err != nil {
		return nil, err
	}
	if row.DayDate.IsZero() {
		return nil, nil
	}
	return &row.DayDate, nil
}

func (s *Store) CleanupToolEvents(cutoff time.Time) error {
	if cutoff.IsZero() {
		return nil
	}
	return s.db.Where("created_at < ?", cutoff).Delete(&ToolEventModel{}).Error
}

type ToolMetricsSummaryInput struct {
	ToolID uint
	From   time.Time
	To     time.Time
}

type ToolMetricsSummary struct {
	TotalEvents    int `json:"total_events"`
	Sessions       int `json:"sessions"`
	ResumeUploads  int `json:"resume_uploads"`
	Responses      int `json:"responses"`
	AudioResponses int `json:"audio_responses"`
	FlowCompletes  int `json:"flow_completes"`
	StageCompletes int `json:"stage_completes"`
}

func (s *Store) SummarizeToolMetrics(input ToolMetricsSummaryInput) (ToolMetricsSummary, error) {
	if input.ToolID == 0 {
		return ToolMetricsSummary{}, gorm.ErrInvalidData
	}

	rows := []toolMetricCountRow{}
	if err := s.db.Model(&ToolDailyMetricModel{}).
		Select("event_name, sum(total_count) as total_count, sum(audio_count) as audio_count").
		Where("tool_id = ? AND day_date >= ? AND day_date <= ?", input.ToolID, normalizeDay(input.From), normalizeDay(input.To)).
		Group("event_name").
		Scan(&rows).Error; err != nil {
		return ToolMetricsSummary{}, err
	}

	return buildToolMetricsSummary(rows), nil
}

type toolMetricKey struct {
	ToolID    uint
	EventName string
}

type toolMetricCountRow struct {
	EventName  string
	TotalCount int
	AudioCount int
}

func ensureToolMetric(metrics map[toolMetricKey]*ToolDailyMetricModel, key toolMetricKey, day time.Time) *ToolDailyMetricModel {
	if metrics == nil {
		return &ToolDailyMetricModel{}
	}
	if existing, ok := metrics[key]; ok {
		return existing
	}
	row := &ToolDailyMetricModel{
		ToolID:    key.ToolID,
		EventName: key.EventName,
		DayDate:   day,
	}
	metrics[key] = row
	return row
}

func normalizeToolEventName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func toolMetricUpdateColumns() []string {
	return []string{"total_count", "audio_count", "updated_at"}
}

func buildToolMetricsSummary(rows []toolMetricCountRow) ToolMetricsSummary {
	summary := ToolMetricsSummary{}
	for _, row := range rows {
		summary.TotalEvents += row.TotalCount
		switch normalizeToolEventName(row.EventName) {
		case "session_start":
			summary.Sessions += row.TotalCount
		case "resume_upload":
			summary.ResumeUploads += row.TotalCount
		case "response":
			summary.Responses += row.TotalCount
			summary.AudioResponses += row.AudioCount
		case "flow_complete":
			summary.FlowCompletes += row.TotalCount
		case "stage_complete":
			summary.StageCompletes += row.TotalCount
		}
	}
	return summary
}

func readBoolFromMetadata(metadata string, key string) bool {
	if strings.TrimSpace(metadata) == "" {
		return false
	}
	raw := map[string]interface{}{}
	if err := json.Unmarshal([]byte(metadata), &raw); err != nil {
		return false
	}
	value, ok := raw[key]
	if !ok {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.ToLower(strings.TrimSpace(typed)) == "true"
	default:
		return false
	}
}
