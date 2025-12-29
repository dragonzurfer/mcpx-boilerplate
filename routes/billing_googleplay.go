package routes

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/core"
	"github.com/mcpx/boilerplate/middleware"
	"google.golang.org/api/androidpublisher/v3"
	"google.golang.org/api/option"
)

type verifyGooglePlayPurchaseRequest struct {
	ProductID     string `json:"productId" binding:"required"`
	PurchaseToken string `json:"purchaseToken" binding:"required"`
}

func (h *BillingHandler) registerGooglePlayRoutesPublic(rg *gin.RouterGroup) {
	rg.POST("/billing/webhook/googleplay", h.googlePlayWebhook)
}

func (h *BillingHandler) registerGooglePlayRoutesAuthed(rg *gin.RouterGroup) {
	rg.POST("/billing/googleplay/verify", h.verifyGooglePlayPurchase)
}

func (h *BillingHandler) verifyGooglePlayPurchase(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req verifyGooglePlayPurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	productID := strings.TrimSpace(req.ProductID)
	purchaseToken := strings.TrimSpace(req.PurchaseToken)
	if productID == "" || purchaseToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "productId and purchaseToken required"})
		return
	}

	purchaseTokenHash := core.GooglePlayPurchaseTokenHash(purchaseToken)
	if shouldLogPlayTokenHash() {
		h.Logger.Printf("info [billing/googleplay/verify]: product=%s tokenHash=%s", productID, purchaseTokenHash)
	}
	if existing, err := h.Store.GetGooglePlayPurchaseByTokenHash(purchaseTokenHash); err == nil {
		if existing.UserID != 0 && existing.UserID != user.ID {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "purchase already linked to another user"})
			return
		}
	}

	plan, ok := h.Plans.PlanForGooglePlayProduct(productID)
	if !ok || plan.Code == "free" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid product"})
		return
	}

	packageName := strings.TrimSpace(os.Getenv("GOOGLE_PLAY_PACKAGE_NAME"))
	if packageName == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server misconfigured"})
		return
	}

	svc, err := newAndroidPublisherService(c.Request.Context())
	if err != nil {
		h.Logger.Printf("error [billing/googleplay/verify]: androidpublisher client: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server misconfigured"})
		return
	}

	purchase, err := svc.Purchases.Subscriptions.Get(packageName, productID, purchaseToken).Do()
	if err != nil {
		h.Logger.Printf("warn [billing/googleplay/verify]: lookup failed product=%s: %v", productID, err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid purchase"})
		return
	}

	if !googlePlayPurchaseBelongsToUser(purchase, user.ID) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "purchase not linked to user"})
		return
	}

	// Acknowledge to prevent Play refunding unacknowledged subscriptions.
	if purchase.AcknowledgementState == 0 {
		ackReq := &androidpublisher.SubscriptionPurchasesAcknowledgeRequest{}
		if err := svc.Purchases.Subscriptions.Acknowledge(packageName, productID, purchaseToken, ackReq).Do(); err != nil {
			h.Logger.Printf("warn [billing/googleplay/verify]: acknowledge failed product=%s: %v", productID, err)
		}
	}

	now := time.Now().UTC()
	status, start, end := googlePlaySubscriptionWindow(purchase, now)

	subRef, err := core.GooglePlaySubscriptionRef(purchaseTokenHash)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid purchase token"})
		return
	}

	raw, _ := json.Marshal(purchase)
	_, _ = h.Store.UpsertGooglePlayPurchase(core.GooglePlayPurchaseModel{
		UserID:                      user.ID,
		PlanCode:                    plan.Code,
		PackageName:                 packageName,
		ProductID:                   productID,
		PurchaseTokenHash:           purchaseTokenHash,
		SubscriptionRef:             subRef,
		OrderID:                     strings.TrimSpace(purchase.OrderId),
		StartTime:                   start,
		ExpiryTime:                  end,
		AutoRenewing:                purchase.AutoRenewing,
		AcknowledgementState:        purchase.AcknowledgementState,
		CancelReason:                purchase.CancelReason,
		PaymentState:                purchase.PaymentState,
		ObfuscatedExternalAccountID: strings.TrimSpace(purchase.ObfuscatedExternalAccountId),
		RawPayload:                  raw,
		LastVerifiedAt:              now,
	})

	// Update entitlements table (shared across providers).
	_, _ = h.Store.UpsertSubscription(user.ID, plan.Code, "googleplay", subRef, status, start, end)

	c.JSON(http.StatusOK, gin.H{
		"ok":               true,
		"status":           status,
		"currentPeriodEnd": end,
	})
}

func (h *BillingHandler) googlePlayWebhook(c *gin.Context) {
	secret := strings.TrimSpace(os.Getenv("GOOGLE_PLAY_PUBSUB_TOKEN"))
	if secret == "" {
		h.Logger.Printf("warn [billing/webhook/googleplay]: missing GOOGLE_PLAY_PUBSUB_TOKEN")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server misconfigured"})
		return
	}
	incoming := strings.TrimSpace(c.GetHeader("X-PubSub-Token"))
	if incoming == "" {
		incoming = strings.TrimSpace(c.Query("token"))
	}
	if incoming == "" || incoming != secret {
		h.Logger.Printf("warn [billing/webhook/googleplay]: invalid token from ip=%s", c.ClientIP())
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	var envelope struct {
		Message struct {
			Data       string            `json:"data"`
			Attributes map[string]string `json:"attributes"`
			MessageID  string            `json:"messageId"`
		} `json:"message"`
		Subscription string `json:"subscription"`
	}
	if err := c.ShouldBindJSON(&envelope); err != nil {
		h.Logger.Printf("warn [billing/webhook/googleplay]: invalid json: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json"})
		return
	}
	data, err := base64.StdEncoding.DecodeString(strings.TrimSpace(envelope.Message.Data))
	if err != nil || len(data) == 0 {
		h.Logger.Printf("warn [billing/webhook/googleplay]: invalid message data: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message"})
		return
	}

	var notif struct {
		Version                  string `json:"version"`
		PackageName              string `json:"packageName"`
		EventTimeMillis          string `json:"eventTimeMillis"`
		SubscriptionNotification struct {
			Version          string `json:"version"`
			NotificationType int64  `json:"notificationType"`
			PurchaseToken    string `json:"purchaseToken"`
			SubscriptionID   string `json:"subscriptionId"`
		} `json:"subscriptionNotification"`
	}
	if err := json.Unmarshal(data, &notif); err != nil {
		h.Logger.Printf("warn [billing/webhook/googleplay]: invalid notification json: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid notification"})
		return
	}
	h.Logger.Printf(
		"info [billing/webhook/googleplay]: received messageId=%s subscription=%s package=%s product=%s type=%d",
		strings.TrimSpace(envelope.Message.MessageID),
		strings.TrimSpace(envelope.Subscription),
		strings.TrimSpace(notif.PackageName),
		strings.TrimSpace(notif.SubscriptionNotification.SubscriptionID),
		notif.SubscriptionNotification.NotificationType,
	)

	productID := strings.TrimSpace(notif.SubscriptionNotification.SubscriptionID)
	purchaseToken := strings.TrimSpace(notif.SubscriptionNotification.PurchaseToken)
	if productID == "" || purchaseToken == "" {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	tokenHash := core.GooglePlayPurchaseTokenHash(purchaseToken)
	if shouldLogPlayTokenHash() {
		h.Logger.Printf("info [billing/webhook/googleplay]: product=%s tokenHash=%s", productID, tokenHash)
	}

	plan, ok := h.Plans.PlanForGooglePlayProduct(productID)
	if !ok || plan.Code == "free" {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	packageName := strings.TrimSpace(os.Getenv("GOOGLE_PLAY_PACKAGE_NAME"))
	if packageName == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server misconfigured"})
		return
	}
	if strings.TrimSpace(notif.PackageName) != "" && strings.TrimSpace(notif.PackageName) != packageName {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "package mismatch"})
		return
	}

	svc, err := newAndroidPublisherService(c.Request.Context())
	if err != nil {
		h.Logger.Printf("error [billing/webhook/googleplay]: androidpublisher client: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server misconfigured"})
		return
	}

	purchase, err := svc.Purchases.Subscriptions.Get(packageName, productID, purchaseToken).Do()
	if err != nil {
		h.Logger.Printf("warn [billing/webhook/googleplay]: lookup failed product=%s: %v", productID, err)
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	now := time.Now().UTC()
	status, start, end := googlePlaySubscriptionWindow(purchase, now)

	subRef, err := core.GooglePlaySubscriptionRef(tokenHash)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	userID := uint(0)
	if existing, err := h.Store.GetGooglePlayPurchaseByTokenHash(tokenHash); err == nil {
		userID = existing.UserID
	}
	if userID == 0 {
		userID = googlePlayUserIDFromPurchase(purchase)
	}

	eventTime := now
	if ms, err := strconv.ParseInt(strings.TrimSpace(notif.EventTimeMillis), 10, 64); err == nil && ms > 0 {
		eventTime = time.UnixMilli(ms).UTC()
	}

	raw, _ := json.Marshal(purchase)
	_, _ = h.Store.UpsertGooglePlayPurchase(core.GooglePlayPurchaseModel{
		UserID:                      userID,
		PlanCode:                    plan.Code,
		PackageName:                 packageName,
		ProductID:                   productID,
		PurchaseTokenHash:           tokenHash,
		SubscriptionRef:             subRef,
		OrderID:                     strings.TrimSpace(purchase.OrderId),
		StartTime:                   start,
		ExpiryTime:                  end,
		AutoRenewing:                purchase.AutoRenewing,
		AcknowledgementState:        purchase.AcknowledgementState,
		CancelReason:                purchase.CancelReason,
		PaymentState:                purchase.PaymentState,
		ObfuscatedExternalAccountID: strings.TrimSpace(purchase.ObfuscatedExternalAccountId),
		RawPayload:                  raw,
		LastNotificationType:        notif.SubscriptionNotification.NotificationType,
		LastEventTime:               eventTime,
		LastVerifiedAt:              now,
	})

	if userID != 0 {
		_, _ = h.Store.UpsertSubscription(userID, plan.Code, "googleplay", subRef, status, start, end)
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func shouldLogPlayTokenHash() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("GOOGLE_PLAY_LOG_TOKEN_HASH")))
	return v == "1" || v == "true" || v == "yes"
}

func newAndroidPublisherService(ctx context.Context) (*androidpublisher.Service, error) {
	// Prefer explicit service account config for local/dev. On GKE, you can use
	// Workload Identity + Play Console-linked service account and omit these.
	credsFile := strings.TrimSpace(os.Getenv("GOOGLE_PLAY_SERVICE_ACCOUNT_FILE"))
	credsJSON := strings.TrimSpace(os.Getenv("GOOGLE_PLAY_SERVICE_ACCOUNT_JSON"))

	opts := []option.ClientOption{
		option.WithScopes(androidpublisher.AndroidpublisherScope),
	}
	if credsFile != "" {
		opts = append(opts, option.WithCredentialsFile(credsFile))
	} else if credsJSON != "" {
		opts = append(opts, option.WithCredentialsJSON([]byte(credsJSON)))
	}
	return androidpublisher.NewService(ctx, opts...)
}

func googlePlayPurchaseBelongsToUser(p *androidpublisher.SubscriptionPurchase, userID uint) bool {
	if userID == 0 {
		return false
	}
	enforce := strings.ToLower(strings.TrimSpace(os.Getenv("GOOGLE_PLAY_ENFORCE_ACCOUNT_ID")))
	if enforce == "" {
		enforce = "false"
	}
	if enforce == "0" || enforce == "false" || enforce == "no" {
		return true
	}
	want := strconv.FormatUint(uint64(userID), 10)
	got := strings.TrimSpace(p.ObfuscatedExternalAccountId)
	if got == "" {
		return false
	}
	return got == want
}

func googlePlayUserIDFromPurchase(p *androidpublisher.SubscriptionPurchase) uint {
	raw := strings.TrimSpace(p.ObfuscatedExternalAccountId)
	if raw == "" {
		return 0
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || n == 0 {
		return 0
	}
	return uint(n)
}

func googlePlaySubscriptionWindow(p *androidpublisher.SubscriptionPurchase, now time.Time) (status string, start, end time.Time) {
	start = time.Time{}
	if p.StartTimeMillis > 0 {
		start = time.UnixMilli(p.StartTimeMillis).UTC()
	}
	end = time.Time{}
	if p.ExpiryTimeMillis > 0 {
		end = time.UnixMilli(p.ExpiryTimeMillis).UTC()
	}

	// Default: no access until verified active.
	status = "expired"

	// Guard: payment pending should not grant access.
	if p.PaymentState != nil && *p.PaymentState == 0 {
		return "pending", start, end
	}

	if !end.IsZero() && end.After(now.Add(30*time.Second)) {
		return "active", start, end
	}
	return status, start, end
}
