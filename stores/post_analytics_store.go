package stores

import (
	"encoding/json"
	"sort"
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

		eventType := normalizePostAnalyticsEventType(evt.EventType)
		switch eventType {
		case "post_open":
			metric.TotalViews++
		case "post_complete":
			metric.Completes++
		case "post_scroll_depth":
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
		case "post_time_on_page":
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

type postAnalyticsRangeInput struct {
	PostID uint
	From   time.Time
	To     time.Time
}

type postAnalyticsRangeDecision struct {
	IncludesToday bool
	OverlayToday  bool
	DayStart      time.Time
	DayEnd        time.Time
}

type postAnalyticsDayRange struct {
	PostID   uint
	DayStart time.Time
	DayEnd   time.Time
}

type postAnalyticsDayLookupInput struct {
	PostID uint
	Day    time.Time
}

type postAnalyticsSummaryMergeInput struct {
	Base    PostAnalyticsSummary
	Overlay PostAnalyticsSummary
}

type postDailyMetricBuildInput struct {
	PostID  uint
	Day     time.Time
	Summary PostAnalyticsSummary
}

type postDailyMetricsMergeInput struct {
	Rows    []PostDailyMetricModel
	Overlay PostDailyMetricModel
}

type postPromoAnalyticsMergeInput struct {
	Rows    []PostPromoAnalyticsRow
	Overlay []PostPromoAnalyticsRow
}

type postAnalyticsEventApplyInput struct {
	Summary PostAnalyticsSummary
	Events  []EventModel
}

var postAnalyticsEventTypes = []string{
	"post_open",
	"post_complete",
	"post_scroll_depth",
	"post_time_on_page",
	"scroll_depth",
	"time_on_page",
}

func normalizePostAnalyticsEventType(eventType string) string {
	normalizedType := strings.ToLower(strings.TrimSpace(eventType))
	switch normalizedType {
	case "scroll_depth":
		return "post_scroll_depth"
	case "time_on_page":
		return "post_time_on_page"
	default:
		return normalizedType
	}
}

func normalizePostAnalyticsRange(input postAnalyticsRangeInput) postAnalyticsRangeInput {
	input.From = normalizeDay(input.From)
	input.To = normalizeDay(input.To)
	return input
}

func (s *Store) decidePostAnalyticsRange(input postAnalyticsRangeInput) (postAnalyticsRangeDecision, error) {
	today := normalizeDay(time.Now().UTC())
	includesToday := !today.Before(input.From) && !today.After(input.To)
	decision := postAnalyticsRangeDecision{IncludesToday: includesToday}

	if !includesToday {
		return decision, nil
	}

	hasRollup, err := s.hasPostMetricsForDay(postAnalyticsDayLookupInput{PostID: input.PostID, Day: today})
	if err != nil {
		return postAnalyticsRangeDecision{}, err
	}
	if hasRollup {
		return decision, nil
	}

	decision.OverlayToday = true
	decision.DayStart = today
	decision.DayEnd = today.Add(24 * time.Hour)
	return decision, nil
}

func (s *Store) hasPostMetricsForDay(input postAnalyticsDayLookupInput) (bool, error) {
	if input.PostID == 0 {
		return false, nil
	}
	var count int64

	err := s.db.Model(&PostDailyMetricModel{}).
		Where("post_id = ? AND day_date = ?", input.PostID, input.Day).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *Store) loadRealtimePostSummary(input postAnalyticsDayRange) (PostAnalyticsSummary, error) {
	summary := PostAnalyticsSummary{}

	events, err := s.listPostAnalyticsEvents(input)
	if err != nil {
		return PostAnalyticsSummary{}, err
	}

	summary = applyPostAnalyticsEvents(postAnalyticsEventApplyInput{Summary: summary, Events: events})

	impressions, err := s.countPostImpressionsForDay(input)
	if err != nil {
		return PostAnalyticsSummary{}, err
	}
	summary.UniqueImpressions = impressions

	promoImpressions, err := s.countPostPromoImpressionsForDay(input)
	if err != nil {
		return PostAnalyticsSummary{}, err
	}
	summary.PromoImpressions = promoImpressions

	promoClicks, err := s.countPostPromoClicksForDay(input)
	if err != nil {
		return PostAnalyticsSummary{}, err
	}
	summary.PromoClicks = promoClicks

	return summary, nil
}

func (s *Store) listPostAnalyticsEvents(input postAnalyticsDayRange) ([]EventModel, error) {
	if input.PostID == 0 {
		return []EventModel{}, gorm.ErrRecordNotFound
	}
	rows := []EventModel{}
	err := s.db.Model(&EventModel{}).
		Select("event_type, metadata").
		Where("entity_type = ? AND entity_id = ? AND created_at >= ? AND created_at < ? AND event_type IN ?",
			"POST",
			input.PostID,
			input.DayStart,
			input.DayEnd,
			postAnalyticsEventTypes,
		).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func applyPostAnalyticsEvents(input postAnalyticsEventApplyInput) PostAnalyticsSummary {
	summary := input.Summary
	for _, evt := range input.Events {
		eventType := normalizePostAnalyticsEventType(evt.EventType)
		switch eventType {
		case "post_open":
			summary.TotalViews++
		case "post_complete":
			summary.Completes++
		case "post_scroll_depth":
			if value, ok := readIntFromMetadata(evt.Metadata, "pct"); ok {
				switch value {
				case 25:
					summary.Scroll25++
				case 50:
					summary.Scroll50++
				case 75:
					summary.Scroll75++
				case 90:
					summary.Scroll90++
				}
			}
		case "post_time_on_page":
			if value, ok := readIntFromMetadata(evt.Metadata, "sec"); ok {
				switch value {
				case 15:
					summary.Time15++
				case 45:
					summary.Time45++
				case 90:
					summary.Time90++
				}
			}
		}
	}
	return summary
}

func (s *Store) countPostImpressionsForDay(input postAnalyticsDayRange) (int, error) {
	if input.PostID == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	var count int64

	err := s.db.Model(&PostImpressionModel{}).
		Where("post_id = ? AND day_date = ?", input.PostID, input.DayStart).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (s *Store) countPostPromoImpressionsForDay(input postAnalyticsDayRange) (int, error) {
	if input.PostID == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	var count int64

	err := s.db.Model(&PromoImpressionModel{}).
		Where("post_id = ? AND created_at >= ? AND created_at < ?", input.PostID, input.DayStart, input.DayEnd).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (s *Store) countPostPromoClicksForDay(input postAnalyticsDayRange) (int, error) {
	if input.PostID == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	var count int64

	err := s.db.Model(&PromoClickModel{}).
		Where("post_id = ? AND created_at >= ? AND created_at < ?", input.PostID, input.DayStart, input.DayEnd).
		Count(&count).Error
	if err != nil {
		return 0, err
	}
	return int(count), nil
}

func (s *Store) loadRealtimePostPromoRows(input postAnalyticsDayRange) ([]PostPromoAnalyticsRow, error) {
	if input.PostID == 0 {
		return []PostPromoAnalyticsRow{}, gorm.ErrRecordNotFound
	}
	impressions := []postPromoCountRow{}
	err := s.db.Model(&PromoImpressionModel{}).
		Select("post_id, promo_id, variant_id, count(*) as impressions").
		Where("post_id = ? AND created_at >= ? AND created_at < ?", input.PostID, input.DayStart, input.DayEnd).
		Group("post_id, promo_id, variant_id").
		Scan(&impressions).Error
	if err != nil {
		return nil, err
	}

	clicks := []postPromoCountRow{}
	err = s.db.Model(&PromoClickModel{}).
		Select("post_id, promo_id, variant_id, count(*) as clicks").
		Where("post_id = ? AND created_at >= ? AND created_at < ?", input.PostID, input.DayStart, input.DayEnd).
		Group("post_id, promo_id, variant_id").
		Scan(&clicks).Error
	if err != nil {
		return nil, err
	}

	clickMap := map[postPromoKey]int{}
	for _, row := range clicks {
		key := postPromoKey{PostID: row.PostID, PromoID: row.PromoID, VariantID: row.VariantID}
		clickMap[key] = row.Clicks
	}

	rows := make([]PostPromoAnalyticsRow, 0, len(impressions))
	for _, row := range impressions {
		key := postPromoKey{PostID: row.PostID, PromoID: row.PromoID, VariantID: row.VariantID}
		rows = append(rows, PostPromoAnalyticsRow{
			PostID:      row.PostID,
			PromoID:     row.PromoID,
			VariantID:   row.VariantID,
			Impressions: row.Impressions,
			Clicks:      clickMap[key],
		})
	}
	return rows, nil
}

func mergePostAnalyticsSummary(input postAnalyticsSummaryMergeInput) PostAnalyticsSummary {
	return PostAnalyticsSummary{
		TotalViews:        input.Base.TotalViews + input.Overlay.TotalViews,
		UniqueImpressions: input.Base.UniqueImpressions + input.Overlay.UniqueImpressions,
		Scroll25:          input.Base.Scroll25 + input.Overlay.Scroll25,
		Scroll50:          input.Base.Scroll50 + input.Overlay.Scroll50,
		Scroll75:          input.Base.Scroll75 + input.Overlay.Scroll75,
		Scroll90:          input.Base.Scroll90 + input.Overlay.Scroll90,
		Time15:            input.Base.Time15 + input.Overlay.Time15,
		Time45:            input.Base.Time45 + input.Overlay.Time45,
		Time90:            input.Base.Time90 + input.Overlay.Time90,
		Completes:         input.Base.Completes + input.Overlay.Completes,
		PromoImpressions:  input.Base.PromoImpressions + input.Overlay.PromoImpressions,
		PromoClicks:       input.Base.PromoClicks + input.Overlay.PromoClicks,
	}
}

func buildPostDailyMetricFromSummary(input postDailyMetricBuildInput) PostDailyMetricModel {
	now := time.Now().UTC()
	return PostDailyMetricModel{
		PostID:            input.PostID,
		DayDate:           input.Day,
		TotalViews:        input.Summary.TotalViews,
		UniqueImpressions: input.Summary.UniqueImpressions,
		Scroll25:          input.Summary.Scroll25,
		Scroll50:          input.Summary.Scroll50,
		Scroll75:          input.Summary.Scroll75,
		Scroll90:          input.Summary.Scroll90,
		Time15:            input.Summary.Time15,
		Time45:            input.Summary.Time45,
		Time90:            input.Summary.Time90,
		Completes:         input.Summary.Completes,
		PromoImpressions:  input.Summary.PromoImpressions,
		PromoClicks:       input.Summary.PromoClicks,
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

func mergePostDailyMetrics(input postDailyMetricsMergeInput) []PostDailyMetricModel {
	if input.Overlay.PostID == 0 || input.Overlay.DayDate.IsZero() {
		return input.Rows
	}
	rows := append([]PostDailyMetricModel{}, input.Rows...)
	rows = append(rows, input.Overlay)
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].DayDate.Before(rows[j].DayDate)
	})
	return rows
}

func mergePostPromoAnalyticsRows(input postPromoAnalyticsMergeInput) []PostPromoAnalyticsRow {
	if len(input.Overlay) == 0 {
		return input.Rows
	}
	merged := map[postPromoKey]PostPromoAnalyticsRow{}
	for _, row := range input.Rows {
		key := postPromoKey{PostID: row.PostID, PromoID: row.PromoID, VariantID: row.VariantID}
		merged[key] = row
	}
	for _, row := range input.Overlay {
		key := postPromoKey{PostID: row.PostID, PromoID: row.PromoID, VariantID: row.VariantID}
		current := merged[key]
		current.PostID = row.PostID
		current.PromoID = row.PromoID
		current.VariantID = row.VariantID
		current.Impressions += row.Impressions
		current.Clicks += row.Clicks
		merged[key] = current
	}
	output := make([]PostPromoAnalyticsRow, 0, len(merged))
	for _, row := range merged {
		output = append(output, row)
	}
	sort.Slice(output, func(i, j int) bool {
		if output[i].PromoID != output[j].PromoID {
			return output[i].PromoID < output[j].PromoID
		}
		return output[i].VariantID < output[j].VariantID
	})
	return output
}

func (s *Store) GetPostAnalyticsSummary(input PostAnalyticsSummaryInput) (PostAnalyticsSummary, error) {
	if input.PostID == 0 {
		return PostAnalyticsSummary{}, gorm.ErrRecordNotFound
	}
	rangeInput := normalizePostAnalyticsRange(postAnalyticsRangeInput(input))

	decision, err := s.decidePostAnalyticsRange(rangeInput)
	if err != nil {
		return PostAnalyticsSummary{}, err
	}

	if !decision.IncludesToday {
		key := postAnalyticsSummaryKey(input)
		if cached, ok := s.getCachedPostAnalyticsSummary(key); ok {
			return cached, nil
		}
	}

	row := PostAnalyticsSummary{}
	err = s.db.Model(&PostDailyMetricModel{}).
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
		Where("post_id = ? AND day_date >= ? AND day_date <= ?", input.PostID, rangeInput.From, rangeInput.To).
		Scan(&row).Error
	if err != nil {
		return PostAnalyticsSummary{}, err
	}

	if decision.OverlayToday {
		realtime, err := s.loadRealtimePostSummary(postAnalyticsDayRange{
			PostID:   input.PostID,
			DayStart: decision.DayStart,
			DayEnd:   decision.DayEnd,
		})
		if err != nil {
			return PostAnalyticsSummary{}, err
		}
		row = mergePostAnalyticsSummary(postAnalyticsSummaryMergeInput{
			Base:    row,
			Overlay: realtime,
		})
	}

	if !decision.IncludesToday {
		key := postAnalyticsSummaryKey(input)
		s.cacheSet(key, row)
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
	rangeInput := normalizePostAnalyticsRange(postAnalyticsRangeInput(input))

	decision, err := s.decidePostAnalyticsRange(rangeInput)
	if err != nil {
		return nil, err
	}

	if !decision.IncludesToday {
		key := postAnalyticsDaysKey(input)
		if cached, ok := s.getCachedPostAnalyticsDays(key); ok {
			return cached, nil
		}
	}
	rows := []PostDailyMetricModel{}
	err = s.db.Where("post_id = ? AND day_date >= ? AND day_date <= ?", input.PostID, rangeInput.From, rangeInput.To).
		Order("day_date asc").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}

	if decision.OverlayToday {
		realtime, err := s.loadRealtimePostSummary(postAnalyticsDayRange{
			PostID:   input.PostID,
			DayStart: decision.DayStart,
			DayEnd:   decision.DayEnd,
		})
		if err != nil {
			return nil, err
		}
		overlay := buildPostDailyMetricFromSummary(postDailyMetricBuildInput{
			PostID:  input.PostID,
			Day:     decision.DayStart,
			Summary: realtime,
		})
		rows = mergePostDailyMetrics(postDailyMetricsMergeInput{Rows: rows, Overlay: overlay})
	}

	if !decision.IncludesToday {
		key := postAnalyticsDaysKey(input)
		s.cacheSet(key, rows)
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
	rangeInput := normalizePostAnalyticsRange(postAnalyticsRangeInput(input))

	decision, err := s.decidePostAnalyticsRange(rangeInput)
	if err != nil {
		return nil, err
	}

	if !decision.IncludesToday {
		key := postPromoAnalyticsKey(input)
		if cached, ok := s.getCachedPostPromoAnalytics(key); ok {
			return cached, nil
		}
	}
	rows := []PostPromoAnalyticsRow{}
	err = s.db.Model(&PostPromoDailyMetricModel{}).
		Select("post_id, promo_id, variant_id, SUM(impressions) as impressions, SUM(clicks) as clicks").
		Where("post_id = ? AND day_date >= ? AND day_date <= ?", input.PostID, rangeInput.From, rangeInput.To).
		Group("post_id, promo_id, variant_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	if decision.OverlayToday {
		realtime, err := s.loadRealtimePostPromoRows(postAnalyticsDayRange{
			PostID:   input.PostID,
			DayStart: decision.DayStart,
			DayEnd:   decision.DayEnd,
		})
		if err != nil {
			return nil, err
		}
		rows = mergePostPromoAnalyticsRows(postPromoAnalyticsMergeInput{
			Rows:    rows,
			Overlay: realtime,
		})
	}

	if !decision.IncludesToday {
		key := postPromoAnalyticsKey(input)
		s.cacheSet(key, rows)
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
