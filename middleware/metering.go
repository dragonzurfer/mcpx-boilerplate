package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/payments"
	"github.com/mcpx/boilerplate/services"
)

type MeterConfig struct {
	Metric         string
	Cost           int64
	RequireSubject bool
	CountStatusMin int
	CountStatusMax int
}

type MeteringService interface {
	ActivePlan(userID uint, now time.Time) (payments.Plan, error)
	CheckUsage(projectKey, subjectType, subjectID, metric string, limit int64, now time.Time) (services.LimitCheck, error)
	IncrementUsage(projectKey, subjectType, subjectID, metric string, delta int64, now time.Time) error
	RecommendedPlanCode() string
}

func Metered(service MeteringService, projectKey string, cfg MeterConfig) gin.HandlerFunc {
	metric := strings.TrimSpace(cfg.Metric)
	cfg = normalizeMeterConfig(cfg)

	return func(c *gin.Context) {
		subjectType, subjectID, ok := resolveSubject(c)
		if !ok && cfg.RequireSubject {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if subjectType == "" || subjectID == "" || metric == "" {
			c.Next()
			return
		}

		userID := uint(0)
		if user, ok := CurrentUser(c); ok {
			userID = user.ID
		}

		now := time.Now()
		plan, err := service.ActivePlan(userID, now)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to load plan"})
			return
		}

		limit := quotaForPlan(plan, metric)
		if limit > 0 {
			check, err := service.CheckUsage(projectKey, subjectType, subjectID, metric, limit, now)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to load usage"})
				return
			}
			if !check.Allowed {
				recommended := service.RecommendedPlanCode()
				c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{
					"error":           "payment_required",
					"metric":          metric,
					"used":            check.Used,
					"limit":           check.Limit,
					"recommendedPlan": recommended,
				})
				return
			}
			remaining := check.Limit - check.Used
			if remaining < 0 {
				remaining = 0
			}
			c.Header("X-Usage-Remaining", strconv.FormatInt(remaining, 10))
		}

		c.Next()

		status := c.Writer.Status()
		if status < cfg.CountStatusMin || status > cfg.CountStatusMax {
			return
		}
		_ = service.IncrementUsage(projectKey, subjectType, subjectID, metric, cfg.Cost, now)
	}
}

func quotaForPlan(plan payments.Plan, metric string) int64 {
	if plan.Quotas == nil {
		return 0
	}
	return plan.Quotas[metric]
}

func normalizeMeterConfig(cfg MeterConfig) MeterConfig {
	if cfg.Cost <= 0 {
		cfg.Cost = 1
	}
	if cfg.CountStatusMin == 0 {
		cfg.CountStatusMin = 200
	}
	if cfg.CountStatusMax == 0 {
		cfg.CountStatusMax = 399
	}
	return cfg
}

func resolveSubject(c *gin.Context) (string, string, bool) {
	if user, ok := CurrentUser(c); ok {
		return "user", strconv.FormatUint(uint64(user.ID), 10), true
	}
	if hash, _, ok := CurrentAPIKey(c); ok {
		return "api_key", hash, true
	}
	ip := strings.TrimSpace(c.ClientIP())
	if ip != "" {
		return "ip", ip, true
	}
	return "", "", false
}
