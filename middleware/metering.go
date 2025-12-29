package middleware

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/billing"
	"github.com/mcpx/boilerplate/core"
)

type MeterConfig struct {
	Metric         string
	Cost           int64
	RequireSubject bool
	CountStatusMin int
	CountStatusMax int
}

func Metered(store *core.Store, plans *billing.Manager, projectKey string, cfg MeterConfig) gin.HandlerFunc {
	metric := strings.TrimSpace(cfg.Metric)
	if cfg.Cost <= 0 {
		cfg.Cost = 1
	}
	if cfg.CountStatusMin == 0 {
		cfg.CountStatusMin = 200
	}
	if cfg.CountStatusMax == 0 {
		cfg.CountStatusMax = 399
	}
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

		now := time.Now()
		plan := plans.FreePlan()
		if user, ok := CurrentUser(c); ok {
			if sub, err := store.GetActiveSubscription(user.ID, now); err == nil {
				if p, found := plans.PlanByCode(sub.PlanCode); found {
					plan = p
				}
			}
		}

		limit := plans.QuotaForPlan(plan, metric)
		if limit > 0 {
			used, err := store.GetUsageCount(projectKey, subjectType, subjectID, metric, now)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to load usage"})
				return
			}
			if used >= limit {
				recommended, _ := plans.RecommendedPlan()
				c.AbortWithStatusJSON(http.StatusPaymentRequired, gin.H{
					"error":           "payment_required",
					"metric":          metric,
					"used":            used,
					"limit":           limit,
					"recommendedPlan": recommended.Code,
				})
				return
			}
			remaining := limit - used
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
		_, _ = store.IncrementUsage(projectKey, subjectType, subjectID, metric, cfg.Cost, now)
	}
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
