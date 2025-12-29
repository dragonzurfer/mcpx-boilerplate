package routes

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/billing"
	"github.com/mcpx/boilerplate/core"
	"github.com/mcpx/boilerplate/middleware"
	"gorm.io/gorm"
)

type BillingHandler struct {
	Store      *core.Store
	Logger     *log.Logger
	Plans      *billing.Manager
	ProjectKey string
}

type LimitCheck struct {
	Allowed bool  `json:"allowed"`
	Used    int64 `json:"used"`
	Limit   int64 `json:"limit"`
}

func (h *BillingHandler) RegisterPublic(rg *gin.RouterGroup) {
	rg.GET("/billing/plans", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"plans":         h.Plans.Config.Plans,
			"currency":      h.Plans.Config.Currency,
			"entryPlanCode": h.Plans.Config.EntryPlanCode,
		})
	})
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

func (h *BillingHandler) me(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	now := time.Now()
	plan, err := h.activePlanForUser(user.ID, now)
	if err != nil {
		h.Logger.Printf("error [billing/me]: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load billing"})
		return
	}

	usageRows, err := h.Store.ListUsageForPeriod(h.ProjectKey, "user", strconv.FormatUint(uint64(user.ID), 10), now)
	if err != nil {
		h.Logger.Printf("error [billing/me]: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load usage"})
		return
	}
	usage := map[string]int64{}
	for _, row := range usageRows {
		usage[row.Metric] = row.Count
	}

	subscription := gin.H(nil)
	if sub, err := h.Store.GetActiveSubscription(user.ID, now); err == nil {
		subscription = gin.H{
			"planCode":           sub.PlanCode,
			"status":             sub.Status,
			"provider":           sub.Provider,
			"subscriptionRef":    sub.SubscriptionRef,
			"currentPeriodStart": sub.CurrentPeriodStart,
			"currentPeriodEnd":   sub.CurrentPeriodEnd,
			"active":             true,
		}
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		if sub, err := h.Store.GetLatestSubscription(user.ID); err == nil {
			subscription = gin.H{
				"planCode":           sub.PlanCode,
				"status":             sub.Status,
				"provider":           sub.Provider,
				"subscriptionRef":    sub.SubscriptionRef,
				"currentPeriodStart": sub.CurrentPeriodStart,
				"currentPeriodEnd":   sub.CurrentPeriodEnd,
				"active":             false,
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"plan":         plan,
		"usage":        usage,
		"subscription": subscription,
	})
}

func (h *BillingHandler) trackUsage(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req struct {
		Metric string `json:"metric" binding:"required"`
		Delta  int64  `json:"delta"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if req.Delta <= 0 {
		req.Delta = 1
	}
	metric := strings.TrimSpace(req.Metric)
	if metric == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "metric required"})
		return
	}

	now := time.Now()
	plan, err := h.activePlanForUser(user.ID, now)
	if err != nil {
		h.Logger.Printf("error [billing/usage]: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load plan"})
		return
	}
	limit := h.Plans.QuotaForPlan(plan, metric)
	check, err := h.checkUsageLimit("user", strconv.FormatUint(uint64(user.ID), 10), metric, limit, now)
	if err != nil {
		h.Logger.Printf("error [billing/usage]: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load usage"})
		return
	}
	if !check.Allowed {
		recommended, _ := h.Plans.RecommendedPlan()
		paymentRequired(c, "Usage limit reached", metric, check.Used, check.Limit, recommended.Code)
		return
	}

	usage, err := h.Store.IncrementUsage(h.ProjectKey, "user", strconv.FormatUint(uint64(user.ID), 10), metric, req.Delta, now)
	if err != nil {
		h.Logger.Printf("error [billing/usage]: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to track usage"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "usage": usage})
}

func (h *BillingHandler) activePlanForUser(userID uint, now time.Time) (billing.Plan, error) {
	if userID == 0 {
		return h.Plans.FreePlan(), nil
	}
	sub, err := h.Store.GetActiveSubscription(userID, now)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return h.Plans.FreePlan(), nil
		}
		return billing.Plan{}, err
	}
	if plan, ok := h.Plans.PlanByCode(sub.PlanCode); ok {
		return plan, nil
	}
	return h.Plans.FreePlan(), nil
}

func (h *BillingHandler) checkUsageLimit(subjectType, subjectID, metric string, limit int64, now time.Time) (LimitCheck, error) {
	if limit <= 0 {
		return LimitCheck{Allowed: true, Used: 0, Limit: limit}, nil
	}
	used, err := h.Store.GetUsageCount(h.ProjectKey, subjectType, subjectID, metric, now)
	if err != nil {
		return LimitCheck{}, err
	}
	return LimitCheck{Allowed: used < limit, Used: used, Limit: limit}, nil
}

func paymentRequired(c *gin.Context, reason, metric string, used, limit int64, recommended string) {
	c.JSON(http.StatusPaymentRequired, gin.H{
		"error":           "payment_required",
		"reason":          reason,
		"metric":          metric,
		"used":            used,
		"limit":           limit,
		"recommendedPlan": recommended,
	})
}

type createCheckoutRequest struct {
	Plan string `json:"plan" binding:"required"`
}

func (h *BillingHandler) createCheckout(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req createCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "plan required"})
		return
	}
	plan, ok := h.Plans.PlanByCode(req.Plan)
	if !ok || strings.ToLower(plan.Type) == "free" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan"})
		return
	}

	keyID := strings.TrimSpace(os.Getenv("RAZORPAY_KEY_ID"))
	keySecret := strings.TrimSpace(os.Getenv("RAZORPAY_KEY_SECRET"))
	if keyID == "" || keySecret == "" {
		h.Logger.Printf("error [billing/checkout]: missing razorpay keys")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server misconfigured"})
		return
	}

	notes := map[string]interface{}{
		"userId": strconv.FormatUint(uint64(user.ID), 10),
		"email":  user.Email,
		"plan":   plan.Code,
	}

	if strings.ToLower(plan.Type) == "subscription" {
		subID, amount, currency, err := createRazorpaySubscription(c, plan, keyID, keySecret, notes)
		if err != nil {
			h.Logger.Printf("error [billing/checkout]: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to create subscription"})
			return
		}
		_, _ = h.Store.UpsertSubscription(user.ID, plan.Code, "razorpay", subID, "created", time.Time{}, time.Time{})
		c.JSON(http.StatusOK, gin.H{
			"type":           "subscription",
			"keyId":          keyID,
			"subscriptionId": subID,
			"amount":         amount,
			"currency":       currency,
			"plan":           plan,
		})
		return
	}

	orderID, amount, currency, err := createRazorpayOrder(c, plan, keyID, keySecret, notes)
	if err != nil {
		h.Logger.Printf("error [billing/checkout]: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to create order"})
		return
	}

	_, _ = h.Store.UpsertSubscription(user.ID, plan.Code, "razorpay", orderID, "created", time.Time{}, time.Time{})

	c.JSON(http.StatusOK, gin.H{
		"type":     "order",
		"keyId":    keyID,
		"orderId":  orderID,
		"amount":   amount,
		"currency": currency,
		"plan":     plan,
	})
}

func createRazorpayOrder(c *gin.Context, plan billing.Plan, keyID, keySecret string, notes map[string]interface{}) (string, int64, string, error) {
	amountPaise := int64(plan.PriceINR) * 100
	if amountPaise <= 0 {
		return "", 0, "", errors.New("invalid amount")
	}

	receipt := "mcpx_" + strconv.FormatInt(time.Now().Unix(), 10)
	body := map[string]interface{}{
		"amount":   amountPaise,
		"currency": "INR",
		"receipt":  receipt,
		"notes":    notes,
	}
	raw, _ := json.Marshal(body)

	httpReq, _ := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, "https://api.razorpay.com/v1/orders", bytes.NewReader(raw))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.SetBasicAuth(keyID, keySecret)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", 0, "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, "", errors.New("razorpay order creation failed")
	}

	var out struct {
		ID       string `json:"id"`
		Amount   int64  `json:"amount"`
		Currency string `json:"currency"`
	}
	_ = json.Unmarshal(respBody, &out)
	if strings.TrimSpace(out.ID) == "" {
		return "", 0, "", errors.New("invalid razorpay response")
	}
	return out.ID, out.Amount, out.Currency, nil
}

func createRazorpaySubscription(c *gin.Context, plan billing.Plan, keyID, keySecret string, notes map[string]interface{}) (string, int64, string, error) {
	planID := strings.TrimSpace(plan.RazorpayPlanID)
	if planID == "" {
		return "", 0, "", errors.New("razorpay plan id missing")
	}
	totalCount := getEnvInt("RAZORPAY_SUBSCRIPTION_TOTAL_COUNT", 12)
	body := map[string]interface{}{
		"plan_id":         planID,
		"total_count":     totalCount,
		"quantity":        1,
		"customer_notify": 1,
		"notes":           notes,
	}
	raw, _ := json.Marshal(body)

	httpReq, _ := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, "https://api.razorpay.com/v1/subscriptions", bytes.NewReader(raw))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.SetBasicAuth(keyID, keySecret)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", 0, "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", 0, "", errors.New("razorpay subscription creation failed")
	}

	var out struct {
		ID       string `json:"id"`
		Status   string `json:"status"`
		Currency string `json:"currency"`
		Quantity int64  `json:"quantity"`
		ShortURL string `json:"short_url"`
		PlanID   string `json:"plan_id"`
	}
	_ = json.Unmarshal(respBody, &out)
	if strings.TrimSpace(out.ID) == "" {
		return "", 0, "", errors.New("invalid razorpay response")
	}
	amount := int64(plan.PriceINR) * 100
	return out.ID, amount, "INR", nil
}

type verifyCheckoutRequest struct {
	RazorpayOrderID        string `json:"razorpayOrderId"`
	RazorpaySubscriptionID string `json:"razorpaySubscriptionId"`
	RazorpayPaymentID      string `json:"razorpayPaymentId"`
	RazorpaySignature      string `json:"razorpaySignature"`
}

func (h *BillingHandler) verifyCheckout(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req verifyCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	ref := strings.TrimSpace(req.RazorpayOrderID)
	base := ""
	if ref != "" {
		base = ref + "|" + strings.TrimSpace(req.RazorpayPaymentID)
	} else if strings.TrimSpace(req.RazorpaySubscriptionID) != "" {
		ref = strings.TrimSpace(req.RazorpaySubscriptionID)
		base = ref + "|" + strings.TrimSpace(req.RazorpayPaymentID)
	}
	if ref == "" || strings.TrimSpace(req.RazorpayPaymentID) == "" || strings.TrimSpace(req.RazorpaySignature) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing payment details"})
		return
	}

	keySecret := strings.TrimSpace(os.Getenv("RAZORPAY_KEY_SECRET"))
	if keySecret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server misconfigured"})
		return
	}

	sub, err := h.Store.GetSubscriptionByRef(user.ID, "razorpay", ref)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "payment not found"})
		return
	}
	if strings.TrimSpace(sub.PlanCode) == "" || strings.TrimSpace(sub.PlanCode) == "free" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan"})
		return
	}
	if strings.TrimSpace(sub.Status) == "active" {
		c.JSON(http.StatusOK, gin.H{"ok": true, "status": "active"})
		return
	}

	mac := hmac.New(sha256.New, []byte(keySecret))
	mac.Write([]byte(base))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(strings.TrimSpace(req.RazorpaySignature))) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	now := time.Now().UTC()
	start := now
	if current, err := h.Store.GetActiveSubscription(user.ID, now); err == nil {
		if !current.CurrentPeriodEnd.IsZero() && current.CurrentPeriodEnd.After(start) {
			start = current.CurrentPeriodEnd
		}
	}
	plan, ok := h.Plans.PlanByCode(sub.PlanCode)
	if !ok {
		plan = h.Plans.FreePlan()
	}
	entDays := plan.EntitlementDays
	if entDays <= 0 {
		if strings.ToLower(plan.Type) == "subscription" {
			entDays = 31
		} else {
			entDays = 365
		}
	}
	end := start.AddDate(0, 0, entDays)

	_, _ = h.Store.UpsertSubscription(user.ID, sub.PlanCode, "razorpay", ref, "active", start, end)
	c.JSON(http.StatusOK, gin.H{"ok": true, "status": "active", "currentPeriodEnd": end})
}

func (h *BillingHandler) razorpayWebhook(c *gin.Context) {
	secret := strings.TrimSpace(os.Getenv("RAZORPAY_WEBHOOK_SECRET"))
	if secret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server misconfigured"})
		return
	}
	sig := strings.TrimSpace(c.GetHeader("X-Razorpay-Signature"))
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(sig)) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	var payload struct {
		Event   string `json:"event"`
		Payload struct {
			Payment struct {
				Entity struct {
					ID       string                 `json:"id"`
					OrderID  string                 `json:"order_id"`
					Status   string                 `json:"status"`
					Amount   int64                  `json:"amount"`
					Currency string                 `json:"currency"`
					Notes    map[string]interface{} `json:"notes"`
				} `json:"entity"`
			} `json:"payment"`
			Order struct {
				Entity struct {
					ID    string                 `json:"id"`
					Notes map[string]interface{} `json:"notes"`
				} `json:"entity"`
			} `json:"order"`
			Subscription struct {
				Entity struct {
					ID           string                 `json:"id"`
					Status       string                 `json:"status"`
					PlanID       string                 `json:"plan_id"`
					CurrentStart int64                  `json:"current_start"`
					CurrentEnd   int64                  `json:"current_end"`
					Notes        map[string]interface{} `json:"notes"`
				} `json:"entity"`
			} `json:"subscription"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	if strings.HasPrefix(payload.Event, "subscription.") {
		h.handleSubscriptionWebhook(payload, c)
		return
	}

	payment := payload.Payload.Payment.Entity
	order := payload.Payload.Order.Entity
	orderID := strings.TrimSpace(payment.OrderID)
	if orderID == "" {
		orderID = strings.TrimSpace(order.ID)
	}
	if orderID == "" || strings.TrimSpace(payment.ID) == "" {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	userID, planCode := readNotes(payment.Notes)
	if userID == 0 {
		userID, planCode = readNotes(order.Notes)
	}
	if planCode == "" {
		planCode = h.Plans.FreePlan().Code
	}
	plan, ok := h.Plans.PlanByCode(planCode)
	if !ok {
		plan = h.Plans.FreePlan()
	}

	expectedAmount := int64(plan.PriceINR) * 100
	if strings.ToUpper(strings.TrimSpace(payment.Currency)) != "INR" || (payment.Amount > 0 && payment.Amount != expectedAmount) {
		h.Logger.Printf("warn [billing/webhook]: mismatched amount/currency order=%s payment=%s", orderID, payment.ID)
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	status := strings.ToLower(strings.TrimSpace(payment.Status))
	if status == "" {
		status = strings.ToLower(strings.TrimSpace(payload.Event))
	}
	if status != "captured" && payload.Event != "payment.captured" {
		if userID != 0 {
			_, _ = h.Store.UpsertSubscription(userID, plan.Code, "razorpay", orderID, status, time.Time{}, time.Time{})
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	if userID != 0 {
		now := time.Now().UTC()
		start := now
		if current, err := h.Store.GetActiveSubscription(userID, now); err == nil {
			if !current.CurrentPeriodEnd.IsZero() && current.CurrentPeriodEnd.After(start) {
				start = current.CurrentPeriodEnd
			}
		}
		entDays := plan.EntitlementDays
		if entDays <= 0 {
			entDays = 365
		}
		end := start.AddDate(0, 0, entDays)
		_, _ = h.Store.UpsertSubscription(userID, plan.Code, "razorpay", orderID, "active", start, end)
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *BillingHandler) handleSubscriptionWebhook(payload struct {
	Event   string `json:"event"`
	Payload struct {
		Payment struct {
			Entity struct {
				ID       string                 `json:"id"`
				OrderID  string                 `json:"order_id"`
				Status   string                 `json:"status"`
				Amount   int64                  `json:"amount"`
				Currency string                 `json:"currency"`
				Notes    map[string]interface{} `json:"notes"`
			} `json:"entity"`
		} `json:"payment"`
		Order struct {
			Entity struct {
				ID    string                 `json:"id"`
				Notes map[string]interface{} `json:"notes"`
			} `json:"entity"`
		} `json:"order"`
		Subscription struct {
			Entity struct {
				ID           string                 `json:"id"`
				Status       string                 `json:"status"`
				PlanID       string                 `json:"plan_id"`
				CurrentStart int64                  `json:"current_start"`
				CurrentEnd   int64                  `json:"current_end"`
				Notes        map[string]interface{} `json:"notes"`
			} `json:"entity"`
		} `json:"subscription"`
	} `json:"payload"`
}, c *gin.Context) {
	sub := payload.Payload.Subscription.Entity
	subID := strings.TrimSpace(sub.ID)
	if subID == "" {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	userID, planCode := readNotes(sub.Notes)
	if planCode == "" {
		planCode = h.planByRazorpayPlanID(sub.PlanID)
	}
	plan, ok := h.Plans.PlanByCode(planCode)
	if !ok {
		plan = h.Plans.FreePlan()
	}

	status := strings.ToLower(strings.TrimSpace(sub.Status))
	if status == "" {
		status = strings.ToLower(strings.TrimSpace(payload.Event))
	}
	start := time.Time{}
	end := time.Time{}
	if sub.CurrentStart > 0 {
		start = time.Unix(sub.CurrentStart, 0).UTC()
	}
	if sub.CurrentEnd > 0 {
		end = time.Unix(sub.CurrentEnd, 0).UTC()
	}
	if userID != 0 {
		_, _ = h.Store.UpsertSubscription(userID, plan.Code, "razorpay", subID, status, start, end)
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func readNotes(notes map[string]interface{}) (uint, string) {
	userID := uint(0)
	planCode := ""
	if v, ok := notes["userId"]; ok {
		switch t := v.(type) {
		case string:
			if n, _ := strconv.ParseUint(t, 10, 64); n > 0 {
				userID = uint(n)
			}
		case float64:
			if t > 0 {
				userID = uint(t)
			}
		}
	}
	if v, ok := notes["plan"].(string); ok {
		planCode = strings.TrimSpace(v)
	}
	return userID, planCode
}

func (h *BillingHandler) planByRazorpayPlanID(planID string) string {
	planID = strings.TrimSpace(planID)
	if planID == "" {
		return ""
	}
	for _, plan := range h.Plans.Config.Plans {
		if strings.TrimSpace(plan.RazorpayPlanID) == planID {
			return plan.Code
		}
	}
	return ""
}

func getEnvInt(key string, fallback int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed
		}
	}
	return fallback
}
