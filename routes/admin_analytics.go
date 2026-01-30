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
	rows := []stageCountRow{}
	if err := h.Store.DB().Model(&stores.UserMetricsModel{}).Select("stage, count(*) as count").Group("stage").Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load funnel analytics"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"stages": rows})
}

func (h *AdminAnalyticsHandler) promos(c *gin.Context) {
	impressions := []promoCountRow{}
	if err := h.Store.DB().Model(&stores.PromoImpressionModel{}).Select("promo_id, count(*) as count").Group("promo_id").Scan(&impressions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load promo analytics"})
		return
	}
	clicks := []promoCountRow{}
	if err := h.Store.DB().Model(&stores.PromoClickModel{}).Select("promo_id, count(*) as count").Group("promo_id").Scan(&clicks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load promo analytics"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"impressions": impressions, "clicks": clicks})
}

func (h *AdminAnalyticsHandler) content(c *gin.Context) {
	rows := []promoCountRow{}
	if err := h.Store.DB().Model(&stores.EventModel{}).Select("entity_id as promo_id, count(*) as count").Where("event_type = ?", "post_open").Group("entity_id").Scan(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load content analytics"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"posts": rows})
}

type stageCountRow struct {
	Stage string `json:"stage"`
	Count int    `json:"count"`
}

type promoCountRow struct {
	PromoID uint `json:"promo_id"`
	Count   int  `json:"count"`
}
