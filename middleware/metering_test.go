package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/payments"
	"github.com/mcpx/boilerplate/services"
)

type fakeMeterService struct {
	plan            payments.Plan
	check           services.LimitCheck
	checkErr        error
	incrementCalls  int
}

func (f *fakeMeterService) ActivePlan(userID uint, now time.Time) (payments.Plan, error) {
	return f.plan, nil
}

func (f *fakeMeterService) CheckUsage(projectKey, subjectType, subjectID, metric string, limit int64, now time.Time) (services.LimitCheck, error) {
	return f.check, f.checkErr
}

func (f *fakeMeterService) IncrementUsage(projectKey, subjectType, subjectID, metric string, delta int64, now time.Time) error {
	f.incrementCalls++
	return nil
}

func (f *fakeMeterService) RecommendedPlanCode() string {
	return "pro"
}

func TestMetered_AllowsAndCounts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeMeterService{
		plan:  payments.Plan{Code: "free", Quotas: map[string]int64{"api_calls": 2}},
		check: services.LimitCheck{Allowed: true, Used: 1, Limit: 2},
	}

	r := gin.New()
	r.Use(Metered(service, "proj", MeterConfig{Metric: "api_calls", Cost: 1, RequireSubject: true}))
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if service.incrementCalls != 1 {
		t.Fatalf("expected increment, got %d", service.incrementCalls)
	}
}

func TestMetered_BlocksWhenOverLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	service := &fakeMeterService{
		plan:  payments.Plan{Code: "free", Quotas: map[string]int64{"api_calls": 1}},
		check: services.LimitCheck{Allowed: false, Used: 1, Limit: 1},
	}

	r := gin.New()
	r.Use(Metered(service, "proj", MeterConfig{Metric: "api_calls", Cost: 1, RequireSubject: true}))
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusPaymentRequired {
		t.Fatalf("expected 402, got %d", w.Code)
	}
	if service.incrementCalls != 0 {
		t.Fatalf("expected no increment, got %d", service.incrementCalls)
	}
}
