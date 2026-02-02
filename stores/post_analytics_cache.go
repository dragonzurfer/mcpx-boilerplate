package stores

type postAnalyticsSummaryCacheKey struct {
	PostID uint   `json:"post_id"`
	From   string `json:"from"`
	To     string `json:"to"`
}

type postAnalyticsDaysCacheKey struct {
	PostID uint   `json:"post_id"`
	From   string `json:"from"`
	To     string `json:"to"`
}

type postPromoAnalyticsCacheKey struct {
	PostID uint   `json:"post_id"`
	From   string `json:"from"`
	To     string `json:"to"`
}

func (s *Store) getCachedPostAnalyticsSummary(key string) (PostAnalyticsSummary, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return PostAnalyticsSummary{}, false
	}
	typed, ok := value.(PostAnalyticsSummary)
	if !ok {
		return PostAnalyticsSummary{}, false
	}
	return typed, true
}

func (s *Store) getCachedPostAnalyticsDays(key string) ([]PostDailyMetricModel, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return nil, false
	}
	typed, ok := value.([]PostDailyMetricModel)
	if !ok {
		return nil, false
	}
	return typed, true
}

func (s *Store) getCachedPostPromoAnalytics(key string) ([]PostPromoAnalyticsRow, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return nil, false
	}
	typed, ok := value.([]PostPromoAnalyticsRow)
	if !ok {
		return nil, false
	}
	return typed, true
}

func postAnalyticsSummaryKey(input PostAnalyticsSummaryInput) string {
	key := postAnalyticsSummaryCacheKey{
		PostID: input.PostID,
		From:   normalizeDay(input.From).Format("2006-01-02"),
		To:     normalizeDay(input.To).Format("2006-01-02"),
	}
	return cacheKey("analytics:post:summary", key)
}

func postAnalyticsDaysKey(input PostAnalyticsDaysInput) string {
	key := postAnalyticsDaysCacheKey{
		PostID: input.PostID,
		From:   normalizeDay(input.From).Format("2006-01-02"),
		To:     normalizeDay(input.To).Format("2006-01-02"),
	}
	return cacheKey("analytics:post:days", key)
}

func postPromoAnalyticsKey(input PostPromoAnalyticsInput) string {
	key := postPromoAnalyticsCacheKey{
		PostID: input.PostID,
		From:   normalizeDay(input.From).Format("2006-01-02"),
		To:     normalizeDay(input.To).Format("2006-01-02"),
	}
	return cacheKey("analytics:post:promos", key)
}
