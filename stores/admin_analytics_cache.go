package stores

type adminAnalyticsCacheKey struct {
	Static string `json:"static"`
}

func (s *Store) getCachedStageCounts(key string) ([]StageCountRow, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return nil, false
	}
	typed, ok := value.([]StageCountRow)
	if !ok {
		return nil, false
	}
	return typed, true
}

func (s *Store) getCachedPromoAnalyticsCounts(key string) (PromoAnalyticsCounts, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return PromoAnalyticsCounts{}, false
	}
	typed, ok := value.(PromoAnalyticsCounts)
	if !ok {
		return PromoAnalyticsCounts{}, false
	}
	return typed, true
}

func (s *Store) getCachedContentAnalyticsCounts(key string) ([]PromoCountRow, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return nil, false
	}
	typed, ok := value.([]PromoCountRow)
	if !ok {
		return nil, false
	}
	return typed, true
}

func funnelAnalyticsKey() string {
	return cacheKey("analytics:admin:funnel", adminAnalyticsCacheKey{Static: "v1"})
}

func promoAnalyticsKey() string {
	return cacheKey("analytics:admin:promos", adminAnalyticsCacheKey{Static: "v1"})
}

func contentAnalyticsKey() string {
	return cacheKey("analytics:admin:content", adminAnalyticsCacheKey{Static: "v1"})
}
