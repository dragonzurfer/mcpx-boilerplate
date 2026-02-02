package stores

type funnelConfigCacheKey struct {
	Static string `json:"static"`
}

type funnelWeightsCacheKey struct {
	Static string `json:"static"`
}

type funnelStagesCacheKey struct {
	Static string `json:"static"`
}

func (s *Store) getCachedFunnelConfig(key string) (*FunnelConfigModel, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return nil, false
	}
	typed, ok := value.(*FunnelConfigModel)
	if !ok || typed == nil {
		return nil, false
	}
	return typed, true
}

func (s *Store) getCachedFunnelWeights(key string) ([]FunnelEventWeightModel, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return nil, false
	}
	typed, ok := value.([]FunnelEventWeightModel)
	if !ok {
		return nil, false
	}
	return typed, true
}

func (s *Store) getCachedFunnelStages(key string) ([]FunnelStageThresholdModel, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return nil, false
	}
	typed, ok := value.([]FunnelStageThresholdModel)
	if !ok {
		return nil, false
	}
	return typed, true
}

func funnelConfigKey() string {
	return cacheKey("funnel:config", funnelConfigCacheKey{Static: "v1"})
}

func funnelWeightsKey() string {
	return cacheKey("funnel:weights", funnelWeightsCacheKey{Static: "v1"})
}

func funnelStagesKey() string {
	return cacheKey("funnel:stages", funnelStagesCacheKey{Static: "v1"})
}
