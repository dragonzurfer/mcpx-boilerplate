package stores

import (
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PostImpressionInput struct {
	PostID    uint
	UserID    *uint
	AnonID    *string
	DayDate   time.Time
	CreatedAt time.Time
}

func (s *Store) UpsertPostImpressions(input []PostImpressionInput) error {
	if len(input) == 0 {
		return nil
	}

	rows := make([]PostImpressionModel, 0, len(input))
	for _, row := range input {
		rows = append(rows, PostImpressionModel{
			PostID:    row.PostID,
			UserID:    row.UserID,
			AnonID:    row.AnonID,
			DayDate:   row.DayDate,
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.CreatedAt,
		})
	}

	return s.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&rows).Error
}

func (s *Store) MergeAnonPostImpressions(anonID string, userID uint) error {
	anonID = strings.TrimSpace(anonID)
	if anonID == "" || userID == 0 {
		return nil
	}

	updates := map[string]interface{}{
		"user_id":    userID,
		"updated_at": time.Now().UTC(),
	}
	return s.db.Model(&PostImpressionModel{}).Where("anon_id = ? AND user_id IS NULL", anonID).Updates(updates).Error
}

type PostAnalyticsRollupInput struct {
	Day time.Time
}

func (s *Store) RollupPostAnalytics(input PostAnalyticsRollupInput) error {
	day := normalizeDay(input.Day)
	dayStart := day
	dayEnd := dayStart.Add(24 * time.Hour)

	metrics := map[uint]*PostDailyMetricModel{}

	events := []EventModel{}
	if err := s.db.Where("entity_type = ? AND entity_id IS NOT NULL AND created_at >= ? AND created_at < ?", "POST", dayStart, dayEnd).Find(&events).Error; err != nil {
		return err
	}

	for _, evt := range events {
		if evt.EntityID == nil || *evt.EntityID == 0 {
			continue
		}
		postID := *evt.EntityID
		metric := ensurePostMetric(metrics, postID, day)

		switch strings.ToLower(strings.TrimSpace(evt.EventType)) {
		case "post_open":
			metric.TotalViews++
		case "post_complete":
			metric.Completes++
		case "scroll_depth":
			if value, ok := readIntFromMetadata(evt.Metadata, "pct"); ok {
				switch value {
				case 25:
					metric.Scroll25++
				case 50:
					metric.Scroll50++
				case 75:
					metric.Scroll75++
				case 90:
					metric.Scroll90++
				}
			}
		case "time_on_page":
			if value, ok := readIntFromMetadata(evt.Metadata, "sec"); ok {
				switch value {
				case 15:
					metric.Time15++
				case 45:
					metric.Time45++
				case 90:
					metric.Time90++
				}
			}
		}
	}

	impressionRows := []postCountRow{}
	if err := s.db.Model(&PostImpressionModel{}).
		Select("post_id, count(*) as count").
		Where("day_date = ?", day).
		Group("post_id").
		Scan(&impressionRows).Error; err != nil {
		return err
	}
	for _, row := range impressionRows {
		metric := ensurePostMetric(metrics, row.PostID, day)
		metric.UniqueImpressions = row.Count
	}

	promoImpressions := []postCountRow{}
	if err := s.db.Model(&PromoImpressionModel{}).
		Select("post_id, count(*) as count").
		Where("post_id IS NOT NULL AND created_at >= ? AND created_at < ?", dayStart, dayEnd).
		Group("post_id").
		Scan(&promoImpressions).Error; err != nil {
		return err
	}
	for _, row := range promoImpressions {
		metric := ensurePostMetric(metrics, row.PostID, day)
		metric.PromoImpressions = row.Count
	}

	promoClicks := []postCountRow{}
	if err := s.db.Model(&PromoClickModel{}).
		Select("post_id, count(*) as count").
		Where("post_id IS NOT NULL AND created_at >= ? AND created_at < ?", dayStart, dayEnd).
		Group("post_id").
		Scan(&promoClicks).Error; err != nil {
		return err
	}
	for _, row := range promoClicks {
		metric := ensurePostMetric(metrics, row.PostID, day)
		metric.PromoClicks = row.Count
	}

	for _, metric := range metrics {
		metric.UpdatedAt = time.Now().UTC()
		if err := s.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "post_id"}, {Name: "day_date"}},
			DoUpdates: clause.AssignmentColumns(postMetricUpdateColumns()),
		}).Create(metric).Error; err != nil {
			return err
		}
	}

	if err := s.rollupPostPromoMetrics(dayStart, dayEnd, day); err != nil {
		return err
	}

	return nil
}

func (s *Store) RunPostAnalyticsRollup(now time.Time, retentionDays int) error {
	targetDay := normalizeDay(now.AddDate(0, 0, -1))
	lastDay, err := s.LatestPostAnalyticsDay()
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
		if err := s.RollupPostAnalytics(PostAnalyticsRollupInput{Day: day}); err != nil {
			return err
		}
	}

	if retentionDays <= 0 {
		return nil
	}
	cutoff := normalizeDay(now.AddDate(0, 0, -retentionDays))
	return s.CleanupOldAnalytics(cutoff)
}

func (s *Store) rollupPostPromoMetrics(dayStart, dayEnd, dayDate time.Time) error {
	impressions := []postPromoCountRow{}
	if err := s.db.Model(&PromoImpressionModel{}).
		Select("post_id, promo_id, variant_id, count(*) as impressions").
		Where("post_id IS NOT NULL AND created_at >= ? AND created_at < ?", dayStart, dayEnd).
		Group("post_id, promo_id, variant_id").
		Scan(&impressions).Error; err != nil {
		return err
	}

	clicks := []postPromoCountRow{}
	if err := s.db.Model(&PromoClickModel{}).
		Select("post_id, promo_id, variant_id, count(*) as clicks").
		Where("post_id IS NOT NULL AND created_at >= ? AND created_at < ?", dayStart, dayEnd).
		Group("post_id, promo_id, variant_id").
		Scan(&clicks).Error; err != nil {
		return err
	}

	clickMap := map[postPromoKey]int{}
	for _, row := range clicks {
		key := postPromoKey{PostID: row.PostID, PromoID: row.PromoID, VariantID: row.VariantID}
		clickMap[key] = row.Clicks
	}

	for _, row := range impressions {
		key := postPromoKey{PostID: row.PostID, PromoID: row.PromoID, VariantID: row.VariantID}
		model := PostPromoDailyMetricModel{
			PostID:      row.PostID,
			PromoID:     row.PromoID,
			VariantID:   row.VariantID,
			DayDate:     dayDate,
			Impressions: row.Impressions,
			Clicks:      clickMap[key],
			UpdatedAt:   time.Now().UTC(),
		}

		if err := s.db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "post_id"}, {Name: "promo_id"}, {Name: "variant_id"}, {Name: "day_date"}},
			DoUpdates: clause.AssignmentColumns([]string{"impressions", "clicks", "updated_at"}),
		}).Create(&model).Error; err != nil {
			return err
		}
	}
	return nil
}

type PostAnalyticsSummaryInput struct {
	PostID uint
	From   time.Time
	To     time.Time
}

type PostAnalyticsSummary struct {
	TotalViews        int
	UniqueImpressions int
	Scroll25          int
	Scroll50          int
	Scroll75          int
	Scroll90          int
	Time15            int
	Time45            int
	Time90            int
	Completes         int
	PromoImpressions  int
	PromoClicks       int
}

func (s *Store) GetPostAnalyticsSummary(input PostAnalyticsSummaryInput) (PostAnalyticsSummary, error) {
	if input.PostID == 0 {
		return PostAnalyticsSummary{}, gorm.ErrRecordNotFound
	}

	row := PostAnalyticsSummary{}
	err := s.db.Model(&PostDailyMetricModel{}).
		Select(`COALESCE(SUM(total_views),0) as total_views,
			COALESCE(SUM(unique_impressions),0) as unique_impressions,
			COALESCE(SUM(scroll25),0) as scroll25,
			COALESCE(SUM(scroll50),0) as scroll50,
			COALESCE(SUM(scroll75),0) as scroll75,
			COALESCE(SUM(scroll90),0) as scroll90,
			COALESCE(SUM(time15),0) as time15,
			COALESCE(SUM(time45),0) as time45,
			COALESCE(SUM(time90),0) as time90,
			COALESCE(SUM(completes),0) as completes,
			COALESCE(SUM(promo_impressions),0) as promo_impressions,
			COALESCE(SUM(promo_clicks),0) as promo_clicks`).
		Where("post_id = ? AND day_date >= ? AND day_date <= ?", input.PostID, input.From, input.To).
		Scan(&row).Error
	if err != nil {
		return PostAnalyticsSummary{}, err
	}
	return row, nil
}

type PostAnalyticsDaysInput struct {
	PostID uint
	From   time.Time
	To     time.Time
}

func (s *Store) ListPostAnalyticsDays(input PostAnalyticsDaysInput) ([]PostDailyMetricModel, error) {
	if input.PostID == 0 {
		return []PostDailyMetricModel{}, gorm.ErrRecordNotFound
	}
	rows := []PostDailyMetricModel{}
	err := s.db.Where("post_id = ? AND day_date >= ? AND day_date <= ?", input.PostID, input.From, input.To).
		Order("day_date asc").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

type PostPromoAnalyticsRow struct {
	PostID      uint `json:"post_id"`
	PromoID     uint `json:"promo_id"`
	VariantID   uint `json:"variant_id"`
	Impressions int  `json:"impressions"`
	Clicks      int  `json:"clicks"`
}

type PostPromoAnalyticsInput struct {
	PostID uint
	From   time.Time
	To     time.Time
}

func (s *Store) ListPostPromoAnalytics(input PostPromoAnalyticsInput) ([]PostPromoAnalyticsRow, error) {
	if input.PostID == 0 {
		return []PostPromoAnalyticsRow{}, gorm.ErrRecordNotFound
	}
	rows := []PostPromoAnalyticsRow{}
	err := s.db.Model(&PostPromoDailyMetricModel{}).
		Select("post_id, promo_id, variant_id, SUM(impressions) as impressions, SUM(clicks) as clicks").
		Where("post_id = ? AND day_date >= ? AND day_date <= ?", input.PostID, input.From, input.To).
		Group("post_id, promo_id, variant_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Store) LatestPostAnalyticsDay() (*time.Time, error) {
	var row PostDailyMetricModel
	err := s.db.Order("day_date desc").Limit(1).Find(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == 0 {
		return nil, nil
	}
	return &row.DayDate, nil
}

func (s *Store) CleanupOldAnalytics(cutoff time.Time) error {
	if cutoff.IsZero() {
		return nil
	}
	if err := s.db.Where("created_at < ?", cutoff).Delete(&EventModel{}).Error; err != nil {
		return err
	}
	if err := s.db.Where("day_date < ?", cutoff).Delete(&PostImpressionModel{}).Error; err != nil {
		return err
	}
	if err := s.db.Where("created_at < ?", cutoff).Delete(&PromoImpressionModel{}).Error; err != nil {
		return err
	}
	if err := s.db.Where("created_at < ?", cutoff).Delete(&PromoClickModel{}).Error; err != nil {
		return err
	}
	return nil
}

type postCountRow struct {
	PostID uint `gorm:"column:post_id"`
	Count  int  `gorm:"column:count"`
}

type postPromoCountRow struct {
	PostID      uint `gorm:"column:post_id"`
	PromoID     uint `gorm:"column:promo_id"`
	VariantID   uint `gorm:"column:variant_id"`
	Impressions int  `gorm:"column:impressions"`
	Clicks      int  `gorm:"column:clicks"`
}

type postPromoKey struct {
	PostID    uint
	PromoID   uint
	VariantID uint
}

func ensurePostMetric(metrics map[uint]*PostDailyMetricModel, postID uint, day time.Time) *PostDailyMetricModel {
	if postID == 0 {
		return &PostDailyMetricModel{}
	}
	if existing, ok := metrics[postID]; ok {
		return existing
	}
	row := &PostDailyMetricModel{
		PostID:  postID,
		DayDate: day,
	}
	metrics[postID] = row
	return row
}

func normalizeDay(value time.Time) time.Time {
	if value.IsZero() {
		return time.Now().UTC().Truncate(24 * time.Hour)
	}
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func postMetricUpdateColumns() []string {
	return []string{
		"total_views",
		"unique_impressions",
		"scroll25",
		"scroll50",
		"scroll75",
		"scroll90",
		"time15",
		"time45",
		"time90",
		"completes",
		"promo_impressions",
		"promo_clicks",
		"updated_at",
	}
}

func readIntFromMetadata(metadata string, key string) (int, bool) {
	if strings.TrimSpace(metadata) == "" {
		return 0, false
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(metadata), &payload); err != nil {
		return 0, false
	}
	raw, ok := payload[key]
	if !ok {
		return 0, false
	}
	switch value := raw.(type) {
	case float64:
		return int(value), true
	case int:
		return value, true
	default:
		return 0, false
	}
}
