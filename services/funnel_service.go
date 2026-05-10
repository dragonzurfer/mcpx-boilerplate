package services

import (
	"math"
	"strings"
	"time"

	"github.com/mcpx/boilerplate/stores"
)

type EntitlementStatus string

const (
	EntitlementNone    EntitlementStatus = "NONE"
	EntitlementActive  EntitlementStatus = "ACTIVE"
	EntitlementExpired EntitlementStatus = "EXPIRED"
)

type FunnelEventInput struct {
	EventType string
	CreatedAt time.Time
}

type StageThreshold struct {
	Stage    string
	MinScore int
	MaxScore *int
	Enabled  bool
}

type FunnelScoreInput struct {
	Now                  time.Time
	Events               []FunnelEventInput
	Weights              map[string]int
	StageThresholds      []StageThreshold
	EntitlementStatus    EntitlementStatus
	DormantDaysThreshold int
	DecayEnabled         bool
	DailyDecayFactor     float64
	LastActiveAtOverride *time.Time
}

type FunnelScoreOutput struct {
	Score        int
	Stage        string
	LastActiveAt *time.Time
	Reads        int
	Completes    int
}

func ComputeFunnelScore(input FunnelScoreInput) FunnelScoreOutput {
	lastActiveAt := resolveLastActiveAt(input)
	score := computeScore(input)

	stage := resolveStage(input, score, lastActiveAt)
	reads, completes := countReadingEvents(input)

	return FunnelScoreOutput{
		Score:        score,
		Stage:        stage,
		LastActiveAt: lastActiveAt,
		Reads:        reads,
		Completes:    completes,
	}
}

func resolveLastActiveAt(input FunnelScoreInput) *time.Time {
	if input.LastActiveAtOverride != nil {
		return input.LastActiveAtOverride
	}

	var last *time.Time
	for _, event := range input.Events {
		if !isActivityEvent(event.EventType) {
			continue
		}
		candidate := event.CreatedAt
		if last == nil || candidate.After(*last) {
			copy := candidate
			last = &copy
		}
	}
	return last
}

func computeScore(input FunnelScoreInput) int {
	score := 0.0
	for _, event := range input.Events {
		eventType := stores.CanonicalFunnelEventType(event.EventType)
		weight := input.Weights[eventType]
		if weight == 0 {
			continue
		}
		multiplier := 1.0
		if input.DecayEnabled {
			multiplier = decayMultiplier(input, event.CreatedAt)
		}
		score += float64(weight) * multiplier
	}
	return int(math.Round(score))
}

func decayMultiplier(input FunnelScoreInput, eventTime time.Time) float64 {
	factor := input.DailyDecayFactor
	if factor <= 0 {
		return 1
	}
	if factor > 1 {
		factor = 1
	}
	days := int(input.Now.Sub(eventTime).Hours() / 24)
	if days <= 0 {
		return 1
	}
	return math.Pow(factor, float64(days))
}

func resolveStage(input FunnelScoreInput, score int, lastActiveAt *time.Time) string {
	if input.EntitlementStatus == EntitlementActive {
		return "PAID_ACTIVE"
	}
	if input.EntitlementStatus == EntitlementExpired {
		return "PAID_EXPIRED"
	}
	if isDormant(input, lastActiveAt) {
		return "DORMANT"
	}

	threshold := pickStageThreshold(input.StageThresholds, score)
	if threshold != "" {
		return threshold
	}
	return "NEW"
}

func isDormant(input FunnelScoreInput, lastActiveAt *time.Time) bool {
	if lastActiveAt == nil {
		return false
	}
	if input.DormantDaysThreshold <= 0 {
		return false
	}

	cutoff := input.Now.AddDate(0, 0, -input.DormantDaysThreshold)
	return lastActiveAt.Before(cutoff)
}

func pickStageThreshold(thresholds []StageThreshold, score int) string {
	sorted := []StageThreshold{}
	for _, threshold := range thresholds {
		if !threshold.Enabled {
			continue
		}
		sorted = append(sorted, threshold)
	}

	if len(sorted) == 0 {
		return ""
	}

	maxMin := -1
	selected := ""
	for _, threshold := range sorted {
		if !scoreWithin(threshold, score) {
			continue
		}
		if threshold.MinScore >= maxMin {
			maxMin = threshold.MinScore
			selected = threshold.Stage
		}
	}
	return selected
}

func scoreWithin(threshold StageThreshold, score int) bool {
	if score < threshold.MinScore {
		return false
	}
	if threshold.MaxScore == nil {
		return true
	}
	return score <= *threshold.MaxScore
}

func countReadingEvents(input FunnelScoreInput) (int, int) {
	reads := 0
	completes := 0
	for _, event := range input.Events {
		eventType := stores.CanonicalFunnelEventType(event.EventType)
		if strings.EqualFold(eventType, "post_open") {
			reads++
		}
		if strings.EqualFold(eventType, "post_complete") {
			completes++
		}
	}
	return reads, completes
}

func isActivityEvent(eventType string) bool {
	value := stores.CanonicalFunnelEventType(eventType)
	switch value {
	case "post_open",
		"post_scroll_depth",
		"post_time_on_page",
		"post_complete",
		"post_promo_click",
		"post_paywall_hit",
		"course_open_click",
		"course_open",
		"course_lesson_click",
		"course_time_on_page",
		"course_lesson_time_on_page",
		"practice_problem_open",
		"practice_time_on_page",
		"practice_run_click",
		"practice_submit_click",
		"practice_ai_analyze_click":
		return true
	default:
		return false
	}
}
