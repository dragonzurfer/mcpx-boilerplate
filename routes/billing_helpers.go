package routes

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

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

func getEnvInt(key string, fallback int) int {
	val := strings.TrimSpace(os.Getenv(key))
	if val == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return parsed
}
