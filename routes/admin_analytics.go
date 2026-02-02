package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/stores"
)

type AdminAnalyticsHandler struct {
	Store *stores.Store
}

func (h *AdminAnalyticsHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/analytics/funnel", h.funnel)
	rg.GET("/analytics/promos", h.promos)
	rg.GET("/analytics/content", h.content)
}

func (h *AdminAnalyticsHandler) funnel(c *gin.Context) {
	rows, err := h.Store.ListStageCounts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load funnel analytics"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"stages": rows})
}

func (h *AdminAnalyticsHandler) promos(c *gin.Context) {
	counts, err := h.Store.GetPromoAnalyticsCounts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load promo analytics"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"impressions": counts.Impressions, "clicks": counts.Clicks})
}

func (h *AdminAnalyticsHandler) content(c *gin.Context) {
	rows, err := h.Store.ListContentAnalyticsCounts()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load content analytics"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"posts": rows})
}
