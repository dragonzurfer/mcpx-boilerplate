package stores

import "strings"

type toolLookupCacheKey struct {
	Slug string `json:"slug"`
}

type toolListCacheKey struct {
	ActiveOnly bool `json:"active_only"`
}

type toolMetricsCacheKey struct {
	ToolID uint   `json:"tool_id"`
	From   string `json:"from"`
	To     string `json:"to"`
}

func (s *Store) getCachedToolLookup(key string) (*ToolModel, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return nil, false
	}
	typed, ok := value.(*ToolModel)
	if !ok || typed == nil {
		return nil, false
	}
	return typed, true
}

func (s *Store) getCachedToolList(key string) ([]ToolModel, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return nil, false
	}
	typed, ok := value.([]ToolModel)
	if !ok {
		return nil, false
	}
	return typed, true
}

func (s *Store) getCachedToolMetrics(key string) (ToolMetricsSummary, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return ToolMetricsSummary{}, false
	}
	typed, ok := value.(ToolMetricsSummary)
	if !ok {
		return ToolMetricsSummary{}, false
	}
	return typed, true
}

func toolLookupKey(slug string) string {
	key := toolLookupCacheKey{Slug: normalizeString(strings.ToLower(slug))}
	return cacheKey("tools:slug", key)
}

func toolListKey(input ToolListInput) string {
	key := toolListCacheKey(input)
	return cacheKey("tools:list", key)
}

func toolMetricsKey(input ToolMetricsSummaryInput) string {
	key := toolMetricsCacheKey{
		ToolID: input.ToolID,
		From:   normalizeDay(input.From).Format("2006-01-02"),
		To:     normalizeDay(input.To).Format("2006-01-02"),
	}
	return cacheKey("tools:metrics", key)
}
