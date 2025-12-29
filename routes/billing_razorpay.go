package routes

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/payments"
	"github.com/mcpx/boilerplate/stores"
)

type createCheckoutRequest struct {
	Plan string `json:"plan" binding:"required"`
}

type verifyCheckoutRequest struct {
	RazorpayOrderID        string `json:"razorpayOrderId"`
	RazorpaySubscriptionID string `json:"razorpaySubscriptionId"`
	RazorpayPaymentID      string `json:"razorpayPaymentId"`
	RazorpaySignature      string `json:"razorpaySignature"`
}

type razorpayKeys struct {
	keyID     string
	keySecret string
}

func (h *BillingHandler) createCheckout(c *gin.Context) {
	user, ok := h.requireUser(c)
	if !ok {
		return
	}
	plan, ok := h.checkoutPlan(c)
	if !ok {
		return
	}
	keys, ok := h.loadRazorpayKeys(c)
	if !ok {
		return
	}

	notes := checkoutNotes(user, plan)
	if strings.ToLower(plan.Type) == "subscription" {
		h.createSubscriptionCheckout(c, user, plan, keys, notes)
		return
	}
	h.createOrderCheckout(c, user, plan, keys, notes)
}

func (h *BillingHandler) checkoutPlan(c *gin.Context) (payments.Plan, bool) {
	var req createCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "plan required"})
		return payments.Plan{}, false
	}
	plan, ok := h.Plans.PlanByCode(req.Plan)
	if !ok || strings.ToLower(plan.Type) == "free" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan"})
		return payments.Plan{}, false
	}
	return plan, true
}

func (h *BillingHandler) loadRazorpayKeys(c *gin.Context) (razorpayKeys, bool) {
	keyID := strings.TrimSpace(os.Getenv("RAZORPAY_KEY_ID"))
	keySecret := strings.TrimSpace(os.Getenv("RAZORPAY_KEY_SECRET"))
	if keyID == "" || keySecret == "" {
		h.Logger.Printf("error [billing/checkout]: missing razorpay keys")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server misconfigured"})
		return razorpayKeys{}, false
	}
	return razorpayKeys{keyID: keyID, keySecret: keySecret}, true
}

func (h *BillingHandler) createSubscriptionCheckout(c *gin.Context, user *stores.UserModel, plan payments.Plan, keys razorpayKeys, notes map[string]interface{}) {
	subID, amount, currency, err := createRazorpaySubscription(c, plan, keys, notes)
	if err != nil {
		h.Logger.Printf("error [billing/checkout]: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to create subscription"})
		return
	}
	_, _ = h.Store.UpsertSubscription(user.ID, plan.Code, "razorpay", subID, "created", time.Time{}, time.Time{})
	c.JSON(http.StatusOK, gin.H{
		"type":           "subscription",
		"keyId":          keys.keyID,
		"subscriptionId": subID,
		"amount":         amount,
		"currency":       currency,
		"plan":           plan,
	})
}

func (h *BillingHandler) createOrderCheckout(c *gin.Context, user *stores.UserModel, plan payments.Plan, keys razorpayKeys, notes map[string]interface{}) {
	orderID, amount, currency, err := createRazorpayOrder(c, plan, keys, notes)
	if err != nil {
		h.Logger.Printf("error [billing/checkout]: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to create order"})
		return
	}
	_, _ = h.Store.UpsertSubscription(user.ID, plan.Code, "razorpay", orderID, "created", time.Time{}, time.Time{})
	c.JSON(http.StatusOK, gin.H{
		"type":     "order",
		"keyId":    keys.keyID,
		"orderId":  orderID,
		"amount":   amount,
		"currency": currency,
		"plan":     plan,
	})
}

func checkoutNotes(user *stores.UserModel, plan payments.Plan) map[string]interface{} {
	return map[string]interface{}{
		"userId": strconv.FormatUint(uint64(user.ID), 10),
		"email":  user.Email,
		"plan":   plan.Code,
	}
}

func createRazorpayOrder(c *gin.Context, plan payments.Plan, keys razorpayKeys, notes map[string]interface{}) (string, int64, string, error) {
	amountPaise, err := amountInPaise(plan.PriceINR)
	if err != nil {
		return "", 0, "", err
	}

	body := razorpayOrderPayload(amountPaise, notes)
	resp, err := postRazorpay(c, keys, "https://api.razorpay.com/v1/orders", body)
	if err != nil {
		return "", 0, "", err
	}

	var out struct {
		ID       string `json:"id"`
		Amount   int64  `json:"amount"`
		Currency string `json:"currency"`
	}
	if err := json.Unmarshal(resp, &out); err != nil {
		return "", 0, "", errors.New("invalid razorpay response")
	}
	if strings.TrimSpace(out.ID) == "" {
		return "", 0, "", errors.New("invalid razorpay response")
	}
	return out.ID, out.Amount, out.Currency, nil
}

func createRazorpaySubscription(c *gin.Context, plan payments.Plan, keys razorpayKeys, notes map[string]interface{}) (string, int64, string, error) {
	planID := strings.TrimSpace(plan.RazorpayPlanID)
	if planID == "" {
		return "", 0, "", errors.New("razorpay plan id missing")
	}

	totalCount := getEnvInt("RAZORPAY_SUBSCRIPTION_TOTAL_COUNT", 12)
	body := razorpaySubscriptionPayload(planID, totalCount, notes)
	resp, err := postRazorpay(c, keys, "https://api.razorpay.com/v1/subscriptions", body)
	if err != nil {
		return "", 0, "", err
	}

	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp, &out); err != nil {
		return "", 0, "", errors.New("invalid razorpay response")
	}
	if strings.TrimSpace(out.ID) == "" {
		return "", 0, "", errors.New("invalid razorpay response")
	}
	amount := int64(plan.PriceINR) * 100
	return out.ID, amount, "INR", nil
}

func amountInPaise(priceINR int) (int64, error) {
	amountPaise := int64(priceINR) * 100
	if amountPaise <= 0 {
		return 0, errors.New("invalid amount")
	}
	return amountPaise, nil
}

func razorpayOrderPayload(amountPaise int64, notes map[string]interface{}) map[string]interface{} {
	receipt := "mcpx_" + strconv.FormatInt(time.Now().Unix(), 10)
	return map[string]interface{}{
		"amount":   amountPaise,
		"currency": "INR",
		"receipt":  receipt,
		"notes":    notes,
	}
}

func razorpaySubscriptionPayload(planID string, totalCount int, notes map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"plan_id":         planID,
		"total_count":     totalCount,
		"quantity":        1,
		"customer_notify": 1,
		"notes":           notes,
	}
}

func postRazorpay(c *gin.Context, keys razorpayKeys, url string, body map[string]interface{}) ([]byte, error) {
	raw, _ := json.Marshal(body)
	request, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.SetBasicAuth(keys.keyID, keys.keySecret)

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, errors.New("razorpay request failed")
	}
	return respBody, nil
}

func (h *BillingHandler) verifyCheckout(c *gin.Context) {
	user, ok := h.requireUser(c)
	if !ok {
		return
	}
	payload, ok := parseVerifyCheckout(c)
	if !ok {
		return
	}

	secret, ok := loadRazorpaySecret(c)
	if !ok {
		return
	}

	sub, ok := h.loadSubscriptionForVerify(c, user.ID, payload.ref)
	if !ok {
		return
	}

	if !verifyRazorpaySignature(secret, payload.base, payload.signature) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	start := h.entitlementStart(user.ID)
	plan := h.planForCode(sub.PlanCode)
	end := entitlementEnd(plan, start)

	_, _ = h.Store.UpsertSubscription(user.ID, sub.PlanCode, "razorpay", payload.ref, "active", start, end)
	c.JSON(http.StatusOK, gin.H{"ok": true, "status": "active", "currentPeriodEnd": end})
}

type verifyPayload struct {
	ref       string
	base      string
	signature string
}

func parseVerifyCheckout(c *gin.Context) (verifyPayload, bool) {
	var req verifyCheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return verifyPayload{}, false
	}

	payload := verifyPayload{}
	payload.ref = strings.TrimSpace(req.RazorpayOrderID)
	if payload.ref != "" {
		payload.base = payload.ref + "|" + strings.TrimSpace(req.RazorpayPaymentID)
	}
	if payload.ref == "" && strings.TrimSpace(req.RazorpaySubscriptionID) != "" {
		payload.ref = strings.TrimSpace(req.RazorpaySubscriptionID)
		payload.base = payload.ref + "|" + strings.TrimSpace(req.RazorpayPaymentID)
	}

	payload.signature = strings.TrimSpace(req.RazorpaySignature)
	if payload.ref == "" || payload.signature == "" || strings.TrimSpace(req.RazorpayPaymentID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing payment details"})
		return verifyPayload{}, false
	}
	return payload, true
}

func loadRazorpaySecret(c *gin.Context) (string, bool) {
	secret := strings.TrimSpace(os.Getenv("RAZORPAY_KEY_SECRET"))
	if secret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server misconfigured"})
		return "", false
	}
	return secret, true
}

func (h *BillingHandler) loadSubscriptionForVerify(c *gin.Context, userID uint, ref string) (*stores.SubscriptionModel, bool) {
	sub, err := h.Store.GetSubscriptionByRef(userID, "razorpay", ref)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "payment not found"})
		return nil, false
	}
	if strings.TrimSpace(sub.PlanCode) == "" || strings.TrimSpace(sub.PlanCode) == "free" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan"})
		return nil, false
	}
	if strings.TrimSpace(sub.Status) == "active" {
		c.JSON(http.StatusOK, gin.H{"ok": true, "status": "active"})
		return nil, false
	}
	return sub, true
}

func verifyRazorpaySignature(secret, base, signature string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(base))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func (h *BillingHandler) entitlementStart(userID uint) time.Time {
	now := time.Now().UTC()
	current, err := h.Store.GetActiveSubscription(userID, now)
	if err != nil {
		return now
	}
	if current.CurrentPeriodEnd.IsZero() {
		return now
	}
	if current.CurrentPeriodEnd.After(now) {
		return current.CurrentPeriodEnd
	}
	return now
}

func (h *BillingHandler) planForCode(code string) payments.Plan {
	plan, ok := h.Plans.PlanByCode(code)
	if ok {
		return plan
	}
	return h.Plans.FreePlan()
}

func entitlementEnd(plan payments.Plan, start time.Time) time.Time {
	entDays := plan.EntitlementDays
	if entDays <= 0 {
		if strings.ToLower(plan.Type) == "subscription" {
			entDays = 31
		} else {
			entDays = 365
		}
	}
	return start.AddDate(0, 0, entDays)
}

type razorpayWebhookPayload struct {
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

func (h *BillingHandler) razorpayWebhook(c *gin.Context) {
	secret := strings.TrimSpace(os.Getenv("RAZORPAY_WEBHOOK_SECRET"))
	if secret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server misconfigured"})
		return
	}
	sig := strings.TrimSpace(c.GetHeader("X-Razorpay-Signature"))
	body, ok := readWebhookBody(c)
	if !ok {
		return
	}
	if !verifyWebhookSignature(secret, sig, body) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}
	payload, ok := parseRazorpayWebhook(body, c)
	if !ok {
		return
	}
	if strings.HasPrefix(payload.Event, "subscription.") {
		h.handleSubscriptionWebhook(payload, c)
		return
	}
	h.handleOrderPaymentWebhook(payload, c)
}

func readWebhookBody(c *gin.Context) ([]byte, bool) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return nil, false
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	return body, true
}

func verifyWebhookSignature(secret, signature string, body []byte) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func parseRazorpayWebhook(body []byte, c *gin.Context) (razorpayWebhookPayload, bool) {
	var payload razorpayWebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return razorpayWebhookPayload{}, false
	}
	return payload, true
}

func (h *BillingHandler) handleOrderPaymentWebhook(payload razorpayWebhookPayload, c *gin.Context) {
	orderID := resolveOrderID(payload)
	if orderID == "" {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	userID, planCode := readNotes(payload)
	plan := h.planForCode(planCode)
	if !verifyPaymentAmount(payload, plan) {
		h.Logger.Printf("warn [billing/webhook]: mismatched amount/currency order=%s", orderID)
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	status := paymentStatus(payload)
	if !isCapturedPayment(status, payload.Event) {
		h.updateSubscriptionStatus(userID, plan.Code, orderID, status)
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	h.activateSubscription(userID, plan, orderID)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func resolveOrderID(payload razorpayWebhookPayload) string {
	orderID := strings.TrimSpace(payload.Payload.Payment.Entity.OrderID)
	if orderID != "" {
		return orderID
	}
	return strings.TrimSpace(payload.Payload.Order.Entity.ID)
}

func readNotes(payload razorpayWebhookPayload) (uint, string) {
	userID, planCode := readNotesFromMap(payload.Payload.Payment.Entity.Notes)
	if userID != 0 || planCode != "" {
		return userID, planCode
	}
	return readNotesFromMap(payload.Payload.Order.Entity.Notes)
}

func readNotesFromMap(notes map[string]interface{}) (uint, string) {
	userID := uint(0)
	planCode := ""
	if v, ok := notes["userId"]; ok {
		switch t := v.(type) {
		case string:
			n, err := strconv.ParseUint(t, 10, 64)
			if err == nil && n > 0 {
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

func verifyPaymentAmount(payload razorpayWebhookPayload, plan payments.Plan) bool {
	payment := payload.Payload.Payment.Entity
	expected := int64(plan.PriceINR) * 100
	currency := strings.ToUpper(strings.TrimSpace(payment.Currency))
	if currency != "INR" {
		return false
	}
	if payment.Amount == 0 {
		return true
	}
	return payment.Amount == expected
}

func paymentStatus(payload razorpayWebhookPayload) string {
	status := strings.ToLower(strings.TrimSpace(payload.Payload.Payment.Entity.Status))
	if status != "" {
		return status
	}
	return strings.ToLower(strings.TrimSpace(payload.Event))
}

func isCapturedPayment(status, event string) bool {
	if status == "captured" {
		return true
	}
	return event == "payment.captured"
}

func (h *BillingHandler) updateSubscriptionStatus(userID uint, planCode, orderID, status string) {
	if userID == 0 {
		return
	}
	_, _ = h.Store.UpsertSubscription(userID, planCode, "razorpay", orderID, status, time.Time{}, time.Time{})
}

func (h *BillingHandler) activateSubscription(userID uint, plan payments.Plan, orderID string) {
	if userID == 0 {
		return
	}
	start := h.entitlementStart(userID)
	end := entitlementEnd(plan, start)
	_, _ = h.Store.UpsertSubscription(userID, plan.Code, "razorpay", orderID, "active", start, end)
}

func (h *BillingHandler) handleSubscriptionWebhook(payload razorpayWebhookPayload, c *gin.Context) {
	sub := payload.Payload.Subscription.Entity
	subID := strings.TrimSpace(sub.ID)
	if subID == "" {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	userID, planCode := readNotesFromMap(sub.Notes)
	if planCode == "" {
		planCode = h.planByRazorpayPlanID(sub.PlanID)
	}
	plan := h.planForCode(planCode)

	status := strings.ToLower(strings.TrimSpace(sub.Status))
	if status == "" {
		status = strings.ToLower(strings.TrimSpace(payload.Event))
	}
	start, end := subscriptionWindow(sub.CurrentStart, sub.CurrentEnd)
	if userID != 0 {
		_, _ = h.Store.UpsertSubscription(userID, plan.Code, "razorpay", subID, status, start, end)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func subscriptionWindow(startUnix, endUnix int64) (time.Time, time.Time) {
	start := time.Time{}
	end := time.Time{}
	if startUnix > 0 {
		start = time.Unix(startUnix, 0).UTC()
	}
	if endUnix > 0 {
		end = time.Unix(endUnix, 0).UTC()
	}
	return start, end
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
