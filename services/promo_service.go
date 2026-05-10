package services

import (
	"math/rand"
	"sort"
	"strings"
	"time"
)

type PromoDecisionInput struct {
	PromosAllowed          bool
	UserStage              string
	Slot                   string
	EntitlementActive      bool
	Now                    time.Time
	RandomSeed             int64
	Promos                 []PromoCandidate
	Metrics                map[uint]PromoMetrics
	GlobalDailyCap         int
	GlobalImpressionsToday int
}

type PromoDecisionOutput struct {
	Promo   *PromoCandidate
	Variant *PromoVariant
}

type PromoCandidate struct {
	ID                   uint
	Code                 string
	Slot                 string
	Status               string
	Priority             int
	EligibleStages       []string
	CooldownHours        int
	MaxImpressionsPerDay int
	MaxClicksPerDay      int
	StartAt              *time.Time
	EndAt                *time.Time
	Variants             []PromoVariant
}

type PromoVariant struct {
	ID         uint
	Headline   string
	Body       string
	CTAText    string
	CTAAction  string
	CTAPayload string
	Weight     int
}

type PromoMetrics struct {
	ImpressionsToday int
	ClicksToday      int
	LastClickAt      *time.Time
}

func DecidePromo(input PromoDecisionInput) PromoDecisionOutput {
	if !input.PromosAllowed {
		return PromoDecisionOutput{}
	}
	if input.EntitlementActive {
		return PromoDecisionOutput{}
	}
	if input.GlobalDailyCap > 0 && input.GlobalImpressionsToday >= input.GlobalDailyCap {
		return PromoDecisionOutput{}
	}

	eligible := filterEligiblePromos(input)
	if len(eligible) == 0 {
		return PromoDecisionOutput{}
	}

	winner := pickHighestPriority(eligible, input)
	if winner == nil {
		return PromoDecisionOutput{}
	}

	variant := pickVariant(*winner, input)
	if variant == nil {
		return PromoDecisionOutput{}
	}

	return PromoDecisionOutput{Promo: winner, Variant: variant}
}

func filterEligiblePromos(input PromoDecisionInput) []PromoCandidate {
	eligible := []PromoCandidate{}
	for _, promo := range input.Promos {
		if !promoActiveForInput(promo, input) {
			continue
		}
		eligible = append(eligible, promo)
	}
	return eligible
}

func promoActiveForInput(promo PromoCandidate, input PromoDecisionInput) bool {
	if strings.ToUpper(promo.Status) != "ACTIVE" {
		return false
	}
	if !strings.EqualFold(promo.Slot, input.Slot) {
		return false
	}
	if !stageEligible(promo.EligibleStages, input.UserStage) {
		return false
	}
	if !withinActiveWindow(promo, input.Now) {
		return false
	}
	if !passesCaps(promo, input) {
		return false
	}
	if !passesCooldown(promo, input) {
		return false
	}
	if len(promo.Variants) == 0 {
		return false
	}
	return true
}

func stageEligible(stages []string, stage string) bool {
	for _, s := range stages {
		if strings.EqualFold(strings.TrimSpace(s), strings.TrimSpace(stage)) {
			return true
		}
	}
	return false
}

func withinActiveWindow(promo PromoCandidate, now time.Time) bool {
	if promo.StartAt != nil && now.Before(*promo.StartAt) {
		return false
	}
	if promo.EndAt != nil && now.After(*promo.EndAt) {
		return false
	}
	return true
}

func passesCaps(promo PromoCandidate, input PromoDecisionInput) bool {
	metrics := input.Metrics[promo.ID]
	if promo.MaxImpressionsPerDay > 0 && metrics.ImpressionsToday >= promo.MaxImpressionsPerDay {
		return false
	}
	if promo.MaxClicksPerDay > 0 && metrics.ClicksToday >= promo.MaxClicksPerDay {
		return false
	}
	return true
}

func passesCooldown(promo PromoCandidate, input PromoDecisionInput) bool {
	if promo.CooldownHours <= 0 {
		return true
	}
	metrics := input.Metrics[promo.ID]
	if metrics.LastClickAt == nil {
		return true
	}
	cooldownUntil := metrics.LastClickAt.Add(time.Duration(promo.CooldownHours) * time.Hour)
	return input.Now.After(cooldownUntil)
}

func pickHighestPriority(promos []PromoCandidate, input PromoDecisionInput) *PromoCandidate {
	sort.SliceStable(promos, func(i, j int) bool {
		return promos[i].Priority > promos[j].Priority
	})

	topPriority := promos[0].Priority
	candidates := []PromoCandidate{promos[0]}

	for _, promo := range promos[1:] {
		if promo.Priority != topPriority {
			break
		}
		candidates = append(candidates, promo)
	}

	if len(candidates) == 1 {
		return &candidates[0]
	}

	randGen := promoRand(input)
	index := randGen.Intn(len(candidates))
	return &candidates[index]
}

func pickVariant(promo PromoCandidate, input PromoDecisionInput) *PromoVariant {
	if len(promo.Variants) == 0 {
		return nil
	}

	weightedTotal := 0
	for _, variant := range promo.Variants {
		weight := normalizedWeight(variant.Weight)
		weightedTotal += weight
	}
	if weightedTotal <= 0 {
		return &promo.Variants[0]
	}

	randGen := promoRand(input)
	draw := randGen.Intn(weightedTotal)

	cursor := 0
	for i := range promo.Variants {
		weight := normalizedWeight(promo.Variants[i].Weight)
		cursor += weight
		if draw < cursor {
			return &promo.Variants[i]
		}
	}

	return &promo.Variants[0]
}

func normalizedWeight(weight int) int {
	if weight <= 0 {
		return 1
	}
	return weight
}

func promoRand(input PromoDecisionInput) *rand.Rand {
	seed := input.RandomSeed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	return rand.New(rand.NewSource(seed))
}
