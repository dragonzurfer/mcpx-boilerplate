package stores

import "strings"

func DefaultFunnelEventWeights() []FunnelEventWeightModel {
	return []FunnelEventWeightModel{
		{EventType: "post_open", Weight: 1, Enabled: true},
		{EventType: "post_scroll_depth", Weight: 2, Enabled: true},
		{EventType: "post_time_on_page", Weight: 2, Enabled: true},
		{EventType: "post_complete", Weight: 5, Enabled: true},
		{EventType: "post_promo_click", Weight: 10, Enabled: true},
		{EventType: "post_paywall_hit", Weight: 6, Enabled: true},
		{EventType: "course_open_click", Weight: 2, Enabled: true},
		{EventType: "course_open", Weight: 2, Enabled: true},
		{EventType: "course_lesson_click", Weight: 3, Enabled: true},
		{EventType: "course_time_on_page", Weight: 3, Enabled: true},
		{EventType: "course_lesson_time_on_page", Weight: 3, Enabled: true},
		{EventType: "practice_problem_open", Weight: 2, Enabled: true},
		{EventType: "practice_time_on_page", Weight: 3, Enabled: true},
		{EventType: "practice_run_click", Weight: 3, Enabled: true},
		{EventType: "practice_submit_click", Weight: 5, Enabled: true},
		{EventType: "practice_ai_analyze_click", Weight: 4, Enabled: true},
	}
}

var funnelLegacyEventAliasMap = map[string]string{
	"scroll_depth": "post_scroll_depth",
	"time_on_page": "post_time_on_page",
	"promo_click":  "post_promo_click",
	"promo_clicks": "post_promo_click",
	"paywall_hit":  "post_paywall_hit",
}

func LegacyFunnelEventAliases() map[string]string {
	aliases := make(map[string]string, len(funnelLegacyEventAliasMap))
	for key, value := range funnelLegacyEventAliasMap {
		aliases[key] = value
	}
	return aliases
}

func CanonicalFunnelEventType(eventType string) string {
	normalizedType := strings.ToLower(strings.TrimSpace(eventType))
	if normalizedType == "" {
		return ""
	}
	if alias, ok := funnelLegacyEventAliasMap[normalizedType]; ok {
		return alias
	}
	return normalizedType
}

func DefaultFunnelStageThresholds() []FunnelStageThresholdModel {
	return []FunnelStageThresholdModel{
		{Stage: FunnelStageNew, MinScore: 0, MaxScore: intPtr(5), Enabled: true},
		{Stage: FunnelStageCasual, MinScore: 6, MaxScore: intPtr(15), Enabled: true},
		{Stage: FunnelStageEngaged, MinScore: 16, MaxScore: intPtr(35), Enabled: true},
		{Stage: FunnelStageHot, MinScore: 36, MaxScore: intPtr(60), Enabled: true},
	}
}

func intPtr(value int) *int {
	return &value
}
