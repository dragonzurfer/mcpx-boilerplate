package routes

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/payments"
)

// ConfigResponse bundles client-side configuration.
type ConfigResponse struct {
	AppName        string `json:"appName"`
	AppKey         string `json:"appKey"`
	GoogleClientID string `json:"googleClientId"`
	RazorpayKeyID  string `json:"razorpayKeyId"`
	BillingEnabled bool   `json:"billingEnabled"`
	Currency       string `json:"currency"`
}

// RegisterConfig routes a GET /config endpoint without requiring auth.
func RegisterConfig(r *gin.Engine, plans *payments.Manager) {
	r.GET("/config", func(c *gin.Context) {
		cfg := ConfigResponse{
			AppName:        strings.TrimSpace(os.Getenv("APP_NAME")),
			AppKey:         strings.TrimSpace(os.Getenv("APP_KEY")),
			GoogleClientID: strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")),
			RazorpayKeyID:  strings.TrimSpace(os.Getenv("RAZORPAY_KEY_ID")),
			BillingEnabled: billingEnabled(),
			Currency:       plans.Config.Currency,
		}
		c.JSON(http.StatusOK, cfg)
	})
}

func billingEnabled() bool {
	value := strings.ToLower(strings.TrimSpace(os.Getenv("ENABLE_BILLING")))
	if value == "" {
		return true
	}
	return value == "1" || value == "true" || value == "yes"
}
