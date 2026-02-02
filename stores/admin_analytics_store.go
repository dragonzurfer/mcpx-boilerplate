package stores

type StageCountRow struct {
	Stage string `json:"stage"`
	Count int    `json:"count"`
}

type PromoCountRow struct {
	PromoID uint `json:"promo_id"`
	Count   int  `json:"count"`
}

type PromoAnalyticsCounts struct {
	Impressions []PromoCountRow `json:"impressions"`
	Clicks      []PromoCountRow `json:"clicks"`
}

func (s *Store) ListStageCounts() ([]StageCountRow, error) {
	key := funnelAnalyticsKey()
	if cached, ok := s.getCachedStageCounts(key); ok {
		return cached, nil
	}

	rows := []StageCountRow{}
	if err := s.db.Model(&UserMetricsModel{}).Select("stage, count(*) as count").Group("stage").Scan(&rows).Error; err != nil {
		return nil, err
	}
	s.cacheSet(key, rows)
	return rows, nil
}

func (s *Store) GetPromoAnalyticsCounts() (PromoAnalyticsCounts, error) {
	key := promoAnalyticsKey()
	if cached, ok := s.getCachedPromoAnalyticsCounts(key); ok {
		return cached, nil
	}

	impressions := []PromoCountRow{}
	if err := s.db.Model(&PromoImpressionModel{}).Select("promo_id, count(*) as count").Group("promo_id").Scan(&impressions).Error; err != nil {
		return PromoAnalyticsCounts{}, err
	}

	clicks := []PromoCountRow{}
	if err := s.db.Model(&PromoClickModel{}).Select("promo_id, count(*) as count").Group("promo_id").Scan(&clicks).Error; err != nil {
		return PromoAnalyticsCounts{}, err
	}

	output := PromoAnalyticsCounts{Impressions: impressions, Clicks: clicks}
	s.cacheSet(key, output)
	return output, nil
}

func (s *Store) ListContentAnalyticsCounts() ([]PromoCountRow, error) {
	key := contentAnalyticsKey()
	if cached, ok := s.getCachedContentAnalyticsCounts(key); ok {
		return cached, nil
	}

	rows := []PromoCountRow{}
	if err := s.db.Model(&EventModel{}).Select("entity_id as promo_id, count(*) as count").Where("event_type = ?", "post_open").Group("entity_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	s.cacheSet(key, rows)
	return rows, nil
}
