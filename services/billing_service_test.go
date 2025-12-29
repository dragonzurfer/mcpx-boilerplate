package services

import (
	"testing"
	"time"

	"github.com/mcpx/boilerplate/payments"
	"github.com/mcpx/boilerplate/stores"
	"gorm.io/gorm"
)

type fakeStore struct {
	activeSub     *stores.SubscriptionModel
	latestSub     *stores.SubscriptionModel
	usageByMetric map[string]int64
	usageRows     []stores.UsageCounterModel
	errActive     error
	errLatest     error
}

func (f *fakeStore) GetActiveSubscription(userID uint, now time.Time) (*stores.SubscriptionModel, error) {
	if f.errActive != nil {
		return nil, f.errActive
	}
	if f.activeSub == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return f.activeSub, nil
}

func (f *fakeStore) GetLatestSubscription(userID uint) (*stores.SubscriptionModel, error) {
	if f.errLatest != nil {
		return nil, f.errLatest
	}
	if f.latestSub == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return f.latestSub, nil
}

func (f *fakeStore) GetUsageCount(projectKey, subjectType, subjectID, metric string, now time.Time) (int64, error) {
	if f.usageByMetric == nil {
		return 0, nil
	}
	return f.usageByMetric[metric], nil
}

func (f *fakeStore) IncrementUsage(projectKey, subjectType, subjectID, metric string, delta int64, now time.Time) (*stores.UsageCounterModel, error) {
	if delta <= 0 {
		delta = 1
	}

	if f.usageByMetric == nil {
		f.usageByMetric = map[string]int64{}
	}

	current := f.usageByMetric[metric]
	current += delta
	f.usageByMetric[metric] = current

	row := &stores.UsageCounterModel{
		ProjectKey:  projectKey,
		SubjectType: subjectType,
		SubjectID:   subjectID,
		Metric:      metric,
		PeriodStart: now,
		Count:       current,
	}
	return row, nil
}

func (f *fakeStore) ListUsageForPeriod(projectKey, subjectType, subjectID string, now time.Time) ([]stores.UsageCounterModel, error) {
	if len(f.usageRows) == 0 {
		return []stores.UsageCounterModel{}, nil
	}
	return f.usageRows, nil
}

func TestBillingServiceActivePlan(t *testing.T) {
	mgr := payments.LoadFromConfig(payments.Config{
		Plans: []payments.Plan{
			{Code: "free", Name: "Free", Type: "free", Interval: "monthly"},
			{Code: "pro", Name: "Pro", Type: "subscription", Interval: "monthly"},
		},
	})

	svc := BillingService{
		Store: &fakeStore{errActive: gorm.ErrRecordNotFound},
		Plans: mgr,
	}

	plan, err := svc.ActivePlan(1, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Code != "free" {
		t.Fatalf("expected free plan, got %q", plan.Code)
	}

	svc.Store = &fakeStore{activeSub: &stores.SubscriptionModel{PlanCode: "pro", Status: "active"}}
	plan, err = svc.ActivePlan(1, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan.Code != "pro" {
		t.Fatalf("expected pro plan, got %q", plan.Code)
	}
}

func TestBillingServiceCheckUsage(t *testing.T) {
	mgr := payments.LoadFromConfig(payments.Config{
		Plans: []payments.Plan{
			{Code: "free", Name: "Free", Type: "free", Interval: "monthly", Quotas: map[string]int64{"api_calls": 10}},
		},
	})

	svc := BillingService{
		Store: &fakeStore{usageByMetric: map[string]int64{"api_calls": 9}},
		Plans: mgr,
	}

	check, err := svc.CheckUsage("proj", "user", "1", "api_calls", 10, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !check.Allowed {
		t.Fatal("expected usage to be allowed")
	}

	svc.Store = &fakeStore{usageByMetric: map[string]int64{"api_calls": 10}}
	check, err = svc.CheckUsage("proj", "user", "1", "api_calls", 10, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if check.Allowed {
		t.Fatal("expected usage to be blocked")
	}
}
