package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/stores"
)

type AdminFunnelHandler struct {
	Store *stores.Store
}

type funnelConfigRequest struct {
	ScoringWindowDays    int     `json:"scoring_window_days"`
	DecayEnabled         bool    `json:"decay_enabled"`
	DailyDecayFactor     float64 `json:"daily_decay_factor"`
	DormantDaysThreshold int     `json:"dormant_days_threshold"`
}

type funnelWeightRequest struct {
	EventType string `json:"event_type"`
	Weight    int    `json:"weight"`
	Enabled   bool   `json:"enabled"`
}

type funnelStageRequest struct {
	Stage    string `json:"stage"`
	MinScore int    `json:"min_score"`
	MaxScore *int   `json:"max_score"`
	Enabled  bool   `json:"enabled"`
}

func (h *AdminFunnelHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/funnel/config", h.getConfig)
	rg.PUT("/funnel/config", h.updateConfig)
	rg.GET("/funnel/weights", h.listWeights)
	rg.PUT("/funnel/weights", h.updateWeights)
	rg.GET("/funnel/stages", h.listStages)
	rg.PUT("/funnel/stages", h.updateStages)
}

func (h *AdminFunnelHandler) getConfig(c *gin.Context) {
	cfg, err := h.Store.GetFunnelConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load config"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"config": cfg})
}

func (h *AdminFunnelHandler) updateConfig(c *gin.Context) {
	var req funnelConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	cfg, err := h.Store.UpdateFunnelConfig(stores.FunnelConfigUpdateInput{
		ScoringWindowDays:    req.ScoringWindowDays,
		DecayEnabled:         req.DecayEnabled,
		DailyDecayFactor:     req.DailyDecayFactor,
		DormantDaysThreshold: req.DormantDaysThreshold,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update config"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"config": cfg})
}

func (h *AdminFunnelHandler) listWeights(c *gin.Context) {
	weights, err := h.Store.ListFunnelEventWeights()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load weights"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"weights": weights})
}

func (h *AdminFunnelHandler) updateWeights(c *gin.Context) {
	var req []funnelWeightRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	for _, weight := range req {
		row := stores.FunnelEventWeightModel{EventType: weight.EventType, Weight: weight.Weight, Enabled: weight.Enabled}
		if err := h.Store.UpsertFunnelEventWeight(row); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update weights"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *AdminFunnelHandler) listStages(c *gin.Context) {
	stages, err := h.Store.ListFunnelStageThresholds()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load stages"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"stages": stages})
}

func (h *AdminFunnelHandler) updateStages(c *gin.Context) {
	var req []funnelStageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	for _, stage := range req {
		row := stores.FunnelStageThresholdModel{Stage: stage.Stage, MinScore: stage.MinScore, MaxScore: stage.MaxScore, Enabled: stage.Enabled}
		if err := h.Store.UpsertFunnelStageThreshold(row); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update stages"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
