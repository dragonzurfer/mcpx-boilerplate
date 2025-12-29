package services

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/mcpx/boilerplate/payments"
	"github.com/mcpx/boilerplate/stores"
	"gorm.io/gorm"
)

type BillingStore interface {
	GetActiveSubscription(userID uint, now time.Time) (*stores.SubscriptionModel, error)
	GetLatestSubscription(userID uint) (*stores.SubscriptionModel, error)
	GetUsageCount(projectKey, subjectType, subjectID, metric string, now time.Time) (int64, error)
	IncrementUsage(projectKey, subjectType, subjectID, metric string, delta int64, now time.Time) (*stores.UsageCounterModel, error)
	ListUsageForPeriod(projectKey, subjectType, subjectID string, now time.Time) ([]stores.UsageCounterModel, error)
}

type BillingService struct {
	Store BillingStore
	Plans *payments.Manager
}

type LimitCheck struct {
	Allowed bool  `json:"allowed"`
	Used    int64 `json:"used"`
	Limit   int64 `json:"limit"`
}

type SubscriptionSummary struct {
	PlanCode           string
	Status             string
	Provider           string
	SubscriptionRef    string
	CurrentPeriodStart time.Time
	CurrentPeriodEnd   time.Time
	Active             bool
}

func (s *BillingService) ActivePlan(userID uint, now time.Time) (payments.Plan, error) {
	if userID == 0 {
		return s.Plans.FreePlan(), nil
	}

	sub, err := s.Store.GetActiveSubscription(userID, now)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s.Plans.FreePlan(), nil
		}
		return payments.Plan{}, err
	}

	plan, ok := s.Plans.PlanByCode(sub.PlanCode)
	if !ok {
		return s.Plans.FreePlan(), nil
	}
	return plan, nil
}

func (s *BillingService) UsageSnapshot(projectKey string, userID uint, now time.Time) (map[string]int64, error) {
	if userID == 0 {
		return map[string]int64{}, nil
	}

	rows, err := s.Store.ListUsageForPeriod(projectKey, "user", strconv.FormatUint(uint64(userID), 10), now)
	if err != nil {
		return nil, err
	}

	usage := map[string]int64{}
	for _, row := range rows {
		usage[row.Metric] = row.Count
	}
	return usage, nil
}

func (s *BillingService) CheckUsage(projectKey, subjectType, subjectID, metric string, limit int64, now time.Time) (LimitCheck, error) {
	if limit <= 0 {
		return LimitCheck{Allowed: true, Used: 0, Limit: limit}, nil
	}

	used, err := s.Store.GetUsageCount(projectKey, subjectType, subjectID, metric, now)
	if err != nil {
		return LimitCheck{}, err
	}

	allowed := used < limit
	return LimitCheck{Allowed: allowed, Used: used, Limit: limit}, nil
}

func (s *BillingService) IncrementUsage(projectKey, subjectType, subjectID, metric string, delta int64, now time.Time) error {
	metric = strings.TrimSpace(metric)
	if metric == "" {
		return nil
	}

	if delta <= 0 {
		delta = 1
	}

	_, err := s.Store.IncrementUsage(projectKey, subjectType, subjectID, metric, delta, now)
	return err
}

func (s *BillingService) SubscriptionSummary(userID uint, now time.Time) (*SubscriptionSummary, error) {
	if userID == 0 {
		return nil, nil
	}

	sub, err := s.Store.GetActiveSubscription(userID, now)
	if err == nil {
		return &SubscriptionSummary{
			PlanCode:           sub.PlanCode,
			Status:             sub.Status,
			Provider:           sub.Provider,
			SubscriptionRef:    sub.SubscriptionRef,
			CurrentPeriodStart: sub.CurrentPeriodStart,
			CurrentPeriodEnd:   sub.CurrentPeriodEnd,
			Active:             true,
		}, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	latest, err := s.Store.GetLatestSubscription(userID)
	if err != nil {
		return nil, err
	}

	return &SubscriptionSummary{
		PlanCode:           latest.PlanCode,
		Status:             latest.Status,
		Provider:           latest.Provider,
		SubscriptionRef:    latest.SubscriptionRef,
		CurrentPeriodStart: latest.CurrentPeriodStart,
		CurrentPeriodEnd:   latest.CurrentPeriodEnd,
		Active:             false,
	}, nil
}

func (s *BillingService) RecommendedPlanCode() string {
	recommended, ok := s.Plans.RecommendedPlan()
	if !ok {
		return ""
	}
	return recommended.Code
}
