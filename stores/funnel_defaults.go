package stores

func DefaultFunnelEventWeights() []FunnelEventWeightModel {
	return []FunnelEventWeightModel{
		{EventType: "post_open", Weight: 1, Enabled: true},
		{EventType: "scroll_depth", Weight: 2, Enabled: true},
		{EventType: "time_on_page", Weight: 2, Enabled: true},
		{EventType: "post_complete", Weight: 5, Enabled: true},
		{EventType: "promo_click", Weight: 10, Enabled: true},
		{EventType: "paywall_hit", Weight: 6, Enabled: true},
	}
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
