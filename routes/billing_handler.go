package routes

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/middleware"
	"github.com/mcpx/boilerplate/payments"
	"github.com/mcpx/boilerplate/services"
	"github.com/mcpx/boilerplate/stores"
)

type BillingHandler struct {
	Store      *stores.Store
	Logger     *log.Logger
	Plans      *payments.Manager
	Service    *services.BillingService
	ProjectKey string
}

func (h *BillingHandler) RegisterPublic(rg *gin.RouterGroup) {
	rg.GET("/billing/plans", h.listPlans)
	rg.POST("/billing/webhook/razorpay", h.razorpayWebhook)
	h.registerGooglePlayRoutesPublic(rg)
}

func (h *BillingHandler) RegisterAuthed(rg *gin.RouterGroup) {
	rg.GET("/billing/me", h.me)
	rg.POST("/billing/checkout", h.createCheckout)
	rg.POST("/billing/verify", h.verifyCheckout)
	rg.POST("/billing/usage", h.trackUsage)
	h.registerGooglePlayRoutesAuthed(rg)
}

func (h *BillingHandler) listPlans(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"plans":         h.Plans.Config.Plans,
		"currency":      h.Plans.Config.Currency,
		"entryPlanCode": h.Plans.Config.EntryPlanCode,
	})
}

func (h *BillingHandler) me(c *gin.Context) {
	user, ok := h.requireUser(c)
	if !ok {
		return
	}

	now := time.Now()
	plan, ok := h.loadPlan(c, user.ID, now)
	if !ok {
		return
	}
	usage, ok := h.loadUsage(c, user.ID, now)
	if !ok {
		return
	}
	subscription, ok := h.loadSubscription(c, user.ID, now)
	if !ok {
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"plan":         plan,
		"usage":        usage,
		"subscription": subscription,
	})
}

type usageRequest struct {
	Metric string `json:"metric" binding:"required"`
	Delta  int64  `json:"delta"`
}

func (h *BillingHandler) trackUsage(c *gin.Context) {
	user, ok := h.requireUser(c)
	if !ok {
		return
	}
	req, ok := h.parseUsageRequest(c)
	if !ok {
		return
	}

	metric := strings.TrimSpace(req.Metric)
	now := time.Now()
	plan, ok := h.loadPlan(c, user.ID, now)
	if !ok {
		return
	}
	limit := h.Plans.QuotaForPlan(plan, metric)

	check, err := h.Service.CheckUsage(h.ProjectKey, "user", strconv.FormatUint(uint64(user.ID), 10), metric, limit, now)
	if err != nil {
		h.Logger.Printf("error [billing/usage]: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load usage"})
		return
	}
	if !check.Allowed {
		paymentRequired(c, "Usage limit reached", metric, check.Used, check.Limit, h.Service.RecommendedPlanCode())
		return
	}

	err = h.Service.IncrementUsage(h.ProjectKey, "user", strconv.FormatUint(uint64(user.ID), 10), metric, req.Delta, now)
	if err != nil {
		h.Logger.Printf("error [billing/usage]: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to track usage"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *BillingHandler) requireUser(c *gin.Context) (*stores.UserModel, bool) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return nil, false
	}
	return user, true
}

func (h *BillingHandler) loadPlan(c *gin.Context, userID uint, now time.Time) (payments.Plan, bool) {
	plan, err := h.Service.ActivePlan(userID, now)
	if err != nil {
		h.Logger.Printf("error [billing/plan]: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load billing"})
		return payments.Plan{}, false
	}
	return plan, true
}

func (h *BillingHandler) loadUsage(c *gin.Context, userID uint, now time.Time) (map[string]int64, bool) {
	usage, err := h.Service.UsageSnapshot(h.ProjectKey, userID, now)
	if err != nil {
		h.Logger.Printf("error [billing/usage]: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load usage"})
		return nil, false
	}
	return usage, true
}

func (h *BillingHandler) loadSubscription(c *gin.Context, userID uint, now time.Time) (gin.H, bool) {
	summary, err := h.Service.SubscriptionSummary(userID, now)
	if err != nil {
		h.Logger.Printf("error [billing/subscription]: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load subscription"})
		return nil, false
	}
	if summary == nil {
		return nil, true
	}

	payload := gin.H{
		"planCode":           summary.PlanCode,
		"status":             summary.Status,
		"provider":           summary.Provider,
		"subscriptionRef":    summary.SubscriptionRef,
		"currentPeriodStart": summary.CurrentPeriodStart,
		"currentPeriodEnd":   summary.CurrentPeriodEnd,
		"active":             summary.Active,
	}
	return payload, true
}

func (h *BillingHandler) parseUsageRequest(c *gin.Context) (usageRequest, bool) {
	var req usageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return usageRequest{}, false
	}
	if req.Delta <= 0 {
		req.Delta = 1
	}
	if strings.TrimSpace(req.Metric) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "metric required"})
		return usageRequest{}, false
	}
	return req, true
}
