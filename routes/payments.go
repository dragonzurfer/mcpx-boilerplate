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
	"github.com/mcpx/boilerplate/middleware"
	"github.com/mcpx/boilerplate/payments"
	"github.com/mcpx/boilerplate/services"
	"github.com/mcpx/boilerplate/stores"
)

type PaymentsHandler struct {
	Store  *stores.Store
	Plans  *payments.Manager
	Logger *log.Logger
}

func (h *PaymentsHandler) RegisterPublic(rg *gin.RouterGroup) {
	rg.GET("/plans", h.listPlans)
	rg.POST("/payments/webhook", h.webhook)
}

func (h *PaymentsHandler) RegisterAuthed(rg *gin.RouterGroup) {
	rg.POST("/payments/create-order", h.createOrder)
	rg.POST("/payments/confirm", h.confirm)
}

type createOrderRequest struct {
	PlanCode string `json:"plan_code" binding:"required"`
}

type confirmPaymentRequest struct {
	RazorpayOrderID   string `json:"razorpay_order_id" binding:"required"`
	RazorpayPaymentID string `json:"razorpay_payment_id" binding:"required"`
	RazorpaySignature string `json:"razorpay_signature" binding:"required"`
}

func (h *PaymentsHandler) listPlans(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"plans": h.Plans.Config.Plans})
}

func (h *PaymentsHandler) createOrder(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req createOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "plan_code required"})
		return
	}
	plan, ok := h.Plans.PlanByCode(req.PlanCode)
	if !ok || strings.ToLower(plan.Type) == "free" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid plan"})
		return
	}

	keys, err := loadRazorpayKeys()
	if err != nil {
		h.Logger.Printf("error [payments/create-order]: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server misconfigured"})
		return
	}

	orderID, amount, currency, err := createRazorpayOrder(c, plan, keys, map[string]interface{}{
		"user_id": strconv.FormatUint(uint64(user.ID), 10),
		"plan":    plan.Code,
		"email":   user.Email,
	})
	if err != nil {
		h.Logger.Printf("error [payments/create-order]: %v", err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to create order"})
		return
	}

	_, err = h.Store.CreatePayment(stores.PaymentCreateInput{UserID: user.ID, PlanCode: plan.Code, RazorpayOrderID: orderID, AmountINR: plan.PriceINR})
	if err != nil {
		h.Logger.Printf("error [payments/create-order]: persist payment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist payment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"order_id":        orderID,
		"amount":          amount,
		"currency":        currency,
		"razorpay_key_id": keys.KeyID,
	})
}

func (h *PaymentsHandler) confirm(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	var req confirmPaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	keys, err := loadRazorpayKeys()
	if err != nil {
		h.Logger.Printf("error [payments/confirm]: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server misconfigured"})
		return
	}

	if !verifyRazorpaySignature(req.RazorpayOrderID, req.RazorpayPaymentID, req.RazorpaySignature, keys.KeySecret) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	payment, err := h.Store.UpdatePaymentStatus(stores.PaymentUpdateInput{
		RazorpayOrderID:   req.RazorpayOrderID,
		RazorpayPaymentID: req.RazorpayPaymentID,
		Status:            stores.PaymentStatusPaid,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update payment"})
		return
	}

	if err := h.applyEntitlement(payment, user.ID); err != nil {
		h.Logger.Printf("error [payments/confirm]: entitlement: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update entitlement"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *PaymentsHandler) webhook(c *gin.Context) {
	secret := strings.TrimSpace(os.Getenv("RAZORPAY_WEBHOOK_SECRET"))
	if secret == "" {
		h.Logger.Printf("warn [payments/webhook]: missing webhook secret")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "webhook secret missing"})
		return
	}

	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	signature := strings.TrimSpace(c.GetHeader("X-Razorpay-Signature"))
	if !verifyWebhookSignature(payload, signature, secret) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	var event razorpayWebhookEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}

	orderID := strings.TrimSpace(event.Payload.Payment.Entity.OrderID)
	paymentID := strings.TrimSpace(event.Payload.Payment.Entity.ID)
	if orderID == "" {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	payment, err := h.Store.UpdatePaymentStatus(stores.PaymentUpdateInput{
		RazorpayOrderID:   orderID,
		RazorpayPaymentID: paymentID,
		Status:            stores.PaymentStatusPaid,
	})
	if err != nil {
		h.Logger.Printf("error [payments/webhook]: update payment: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update payment"})
		return
	}

	if err := h.applyEntitlement(payment, payment.UserID); err != nil {
		h.Logger.Printf("error [payments/webhook]: entitlement: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update entitlement"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type razorpayKeys struct {
	KeyID     string
	KeySecret string
}

func loadRazorpayKeys() (razorpayKeys, error) {
	keyID := strings.TrimSpace(os.Getenv("RAZORPAY_KEY_ID"))
	keySecret := strings.TrimSpace(os.Getenv("RAZORPAY_KEY_SECRET"))
	if keyID == "" || keySecret == "" {
		return razorpayKeys{}, errors.New("razorpay keys missing")
	}
	return razorpayKeys{KeyID: keyID, KeySecret: keySecret}, nil
}

func createRazorpayOrder(c *gin.Context, plan payments.Plan, keys razorpayKeys, notes map[string]interface{}) (string, int64, string, error) {
	amountPaise := int64(plan.PriceINR) * 100
	if amountPaise <= 0 {
		return "", 0, "", errors.New("invalid amount")
	}

	body := map[string]interface{}{
		"amount":   amountPaise,
		"currency": "INR",
		"receipt":  "mcpx_" + strconv.FormatInt(time.Now().Unix(), 10),
		"notes":    notes,
	}

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
	if out.ID == "" {
		return "", 0, "", errors.New("invalid razorpay response")
	}
	return out.ID, out.Amount, out.Currency, nil
}

func postRazorpay(c *gin.Context, keys razorpayKeys, url string, payload map[string]interface{}) ([]byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.SetBasicAuth(keys.KeyID, keys.KeySecret)
	request.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 {
		return nil, errors.New("razorpay error")
	}
	return data, nil
}

func verifyRazorpaySignature(orderID, paymentID, signature, secret string) bool {
	payload := orderID + "|" + paymentID
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func verifyWebhookSignature(payload []byte, signature, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

func (h *PaymentsHandler) applyEntitlement(payment *stores.PaymentModel, userID uint) error {
	if payment == nil {
		return errors.New("payment missing")
	}
	plan, ok := h.Plans.PlanByCode(payment.PlanCode)
	if !ok {
		return errors.New("plan missing")
	}

	current, _ := h.Store.GetLatestEntitlement(userID)
	window := services.ComputeEntitlementWindow(services.EntitlementWindowInput{
		Now:          time.Now().UTC(),
		Current:      current,
		DurationDays: plan.EntitlementDays,
	})

	_, err := h.Store.UpsertEntitlement(stores.EntitlementUpsertInput{
		UserID:       userID,
		PlanCode:     plan.Code,
		StartAt:      window.StartAt,
		EndAt:        window.EndAt,
		LastPaymentID: payment.ID,
	})
	return err
}

type razorpayWebhookEvent struct {
	Event   string `json:"event"`
	Payload struct {
		Payment struct {
			Entity struct {
				ID      string `json:"id"`
				OrderID string `json:"order_id"`
			} `json:"entity"`
		} `json:"payment"`
	} `json:"payload"`
}
