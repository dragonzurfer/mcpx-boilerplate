package routes

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/stores"
)

type AdminPostAnalyticsHandler struct {
	Store *stores.Store
}

func (h *AdminPostAnalyticsHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/analytics/posts/:id/summary", h.summary)
	rg.GET("/analytics/posts/:id/timeseries", h.timeseries)
	rg.GET("/analytics/posts/:id/funnel", h.funnel)
	rg.GET("/analytics/posts/:id/promos", h.promos)
}

func (h *AdminPostAnalyticsHandler) summary(c *gin.Context) {
	postID, ok := parseUintParam(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid post id"})
		return
	}

	from, to, err := parseDateRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	summary, err := h.Store.GetPostAnalyticsSummary(stores.PostAnalyticsSummaryInput{PostID: postID, From: from, To: to})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load post analytics"})
		return
	}

	completionRate := ratio(summary.Completes, summary.UniqueImpressions)
	promoCTR := ratio(summary.PromoClicks, summary.PromoImpressions)

	c.JSON(http.StatusOK, gin.H{
		"post_id": postID,
		"range": gin.H{
			"from": from.Format("2006-01-02"),
			"to":   to.Format("2006-01-02"),
		},
		"totals": gin.H{
			"total_views":        summary.TotalViews,
			"unique_impressions": summary.UniqueImpressions,
			"scroll_25":          summary.Scroll25,
			"scroll_50":          summary.Scroll50,
			"scroll_75":          summary.Scroll75,
			"scroll_90":          summary.Scroll90,
			"time_15":            summary.Time15,
			"time_45":            summary.Time45,
			"time_90":            summary.Time90,
			"completes":          summary.Completes,
			"completion_rate":    completionRate,
			"promo_impressions":  summary.PromoImpressions,
			"promo_clicks":       summary.PromoClicks,
			"promo_ctr":          promoCTR,
		},
	})
}

func (h *AdminPostAnalyticsHandler) timeseries(c *gin.Context) {
	postID, ok := parseUintParam(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid post id"})
		return
	}

	from, to, err := parseDateRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rows, err := h.Store.ListPostAnalyticsDays(stores.PostAnalyticsDaysInput{PostID: postID, From: from, To: to})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load post analytics"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"post_id": postID, "days": rows})
}

func (h *AdminPostAnalyticsHandler) funnel(c *gin.Context) {
	postID, ok := parseUintParam(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid post id"})
		return
	}

	from, to, err := parseDateRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	summary, err := h.Store.GetPostAnalyticsSummary(stores.PostAnalyticsSummaryInput{PostID: postID, From: from, To: to})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load post analytics"})
		return
	}

	steps := []gin.H{
		{"label": "Unique impressions", "value": summary.UniqueImpressions},
		{"label": "Scroll 50%", "value": summary.Scroll50},
		{"label": "Scroll 75%", "value": summary.Scroll75},
		{"label": "Completes", "value": summary.Completes},
		{"label": "Promo clicks", "value": summary.PromoClicks},
	}

	c.JSON(http.StatusOK, gin.H{"post_id": postID, "steps": steps})
}

func (h *AdminPostAnalyticsHandler) promos(c *gin.Context) {
	postID, ok := parseUintParam(c.Param("id"))
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid post id"})
		return
	}

	from, to, err := parseDateRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rows, err := h.Store.ListPostPromoAnalytics(stores.PostPromoAnalyticsInput{PostID: postID, From: from, To: to})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load post promo analytics"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"post_id": postID, "items": rows})
}

func parseUintParam(value string) (uint, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed == 0 {
		return 0, false
	}
	return uint(parsed), true
}

func parseDateRange(c *gin.Context) (time.Time, time.Time, error) {
	layout := "2006-01-02"
	now := time.Now().UTC()
	defaultTo := dateOnly(now)
	defaultFrom := dateOnly(now.AddDate(0, 0, -13))

	from := defaultFrom
	to := defaultTo

	if raw := strings.TrimSpace(c.Query("from")); raw != "" {
		parsed, err := time.Parse(layout, raw)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		from = dateOnly(parsed)
	}
	if raw := strings.TrimSpace(c.Query("to")); raw != "" {
		parsed, err := time.Parse(layout, raw)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		to = dateOnly(parsed)
	}
	if from.After(to) {
		return time.Time{}, time.Time{}, errInvalid("from must be <= to")
	}
	return from, to, nil
}

func dateOnly(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), 0, 0, 0, 0, time.UTC)
}

func ratio(numerator, denominator int) float64 {
	if denominator <= 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}
