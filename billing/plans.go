package billing

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Plan struct {
	Code                 string           `json:"code"`
	Name                 string           `json:"name"`
	Description          string           `json:"description,omitempty"`
	Type                 string           `json:"type"`     // free, one_time, subscription
	Interval             string           `json:"interval"` // monthly, yearly
	PriceINR             int              `json:"priceInr"`
	MostPopular          bool             `json:"mostPopular"`
	Quotas               map[string]int64 `json:"quotas"`
	EntitlementDays      int              `json:"entitlementDays,omitempty"`
	RazorpayPlanID       string           `json:"razorpayPlanId,omitempty"`
	GooglePlayProductIDs []string         `json:"googlePlayProductIds,omitempty"`
}

type Config struct {
	Currency      string `json:"currency"`
	EntryPlanCode string `json:"entryPlanCode"`
	Plans         []Plan `json:"plans"`
}

type Manager struct {
	Config     Config
	planByCode map[string]Plan
}

func LoadFromEnv() (*Manager, error) {
	if raw := strings.TrimSpace(os.Getenv("PLANS_JSON")); raw != "" {
		return LoadFromBytes([]byte(raw))
	}

	path := strings.TrimSpace(os.Getenv("PLANS_FILE"))
	if path == "" {
		path = filepath.Join("config", "plans.json")
	}
	if data, err := os.ReadFile(path); err == nil {
		return LoadFromBytes(data)
	}

	return LoadFromConfig(defaultConfig()), nil
}

func LoadFromBytes(data []byte) (*Manager, error) {
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse plans: %w", err)
	}
	return LoadFromConfig(cfg), nil
}

func LoadFromConfig(cfg Config) *Manager {
	if cfg.Currency == "" {
		cfg.Currency = "INR"
	}
	m := &Manager{Config: cfg, planByCode: map[string]Plan{}}
	for _, plan := range cfg.Plans {
		code := strings.ToLower(strings.TrimSpace(plan.Code))
		if code == "" {
			continue
		}
		plan.Code = code
		if plan.Quotas == nil {
			plan.Quotas = map[string]int64{}
		}
		m.planByCode[code] = plan
	}
	return m
}

func (m *Manager) PlanByCode(code string) (Plan, bool) {
	code = strings.ToLower(strings.TrimSpace(code))
	plan, ok := m.planByCode[code]
	return plan, ok
}

func (m *Manager) FreePlan() Plan {
	if plan, ok := m.PlanByCode("free"); ok {
		return plan
	}
	for _, plan := range m.Config.Plans {
		if strings.ToLower(strings.TrimSpace(plan.Type)) == "free" {
			return plan
		}
	}
	if len(m.Config.Plans) > 0 {
		return m.Config.Plans[0]
	}
	return Plan{Code: "free", Name: "Free", Type: "free", Interval: "monthly", Quotas: map[string]int64{}}
}

func (m *Manager) EntryPlan() (Plan, bool) {
	if m.Config.EntryPlanCode != "" {
		if plan, ok := m.PlanByCode(m.Config.EntryPlanCode); ok {
			return plan, true
		}
	}
	for _, plan := range m.Config.Plans {
		if strings.ToLower(strings.TrimSpace(plan.Type)) == "one_time" {
			return plan, true
		}
	}
	return Plan{}, false
}

func (m *Manager) RecommendedPlan() (Plan, bool) {
	for _, plan := range m.Config.Plans {
		if plan.MostPopular {
			return plan, true
		}
	}
	for _, plan := range m.Config.Plans {
		if strings.ToLower(strings.TrimSpace(plan.Type)) != "free" {
			return plan, true
		}
	}
	return Plan{}, false
}

func (m *Manager) PlanForGooglePlayProduct(productID string) (Plan, bool) {
	productID = strings.TrimSpace(productID)
	if productID == "" {
		return Plan{}, false
	}
	for _, plan := range m.Config.Plans {
		for _, pid := range plan.GooglePlayProductIDs {
			if strings.TrimSpace(pid) == productID {
				return plan, true
			}
		}
	}
	return Plan{}, false
}

func (m *Manager) QuotaForPlan(plan Plan, metric string) int64 {
	metric = strings.TrimSpace(metric)
	if metric == "" {
		return 0
	}
	if plan.Quotas == nil {
		return 0
	}
	return plan.Quotas[metric]
}

func defaultConfig() Config {
	return Config{
		Currency:      "INR",
		EntryPlanCode: "entry",
		Plans: []Plan{
			{
				Code:            "free",
				Name:            "Free",
				Type:            "free",
				Interval:        "monthly",
				PriceINR:        0,
				Quotas:          map[string]int64{"api_calls": 1000},
				EntitlementDays: 30,
			},
			{
				Code:            "entry",
				Name:            "Entry",
				Type:            "one_time",
				Interval:        "yearly",
				PriceINR:        1999,
				MostPopular:     false,
				Quotas:          map[string]int64{"api_calls": 5000},
				EntitlementDays: 365,
			},
			{
				Code:            "pro",
				Name:            "Pro",
				Type:            "subscription",
				Interval:        "monthly",
				PriceINR:        499,
				MostPopular:     true,
				Quotas:          map[string]int64{"api_calls": 20000},
				EntitlementDays: 31,
			},
		},
	}
}
