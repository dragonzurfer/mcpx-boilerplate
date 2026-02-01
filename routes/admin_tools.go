package routes

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/stores"
)

type AdminToolsHandler struct {
	Store *stores.Store
}

func (h *AdminToolsHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/tools", h.list)
	rg.PUT("/tools/:id", h.update)
}

type adminToolUpdateRequest struct {
	Name     string            `json:"name"`
	Category string            `json:"category"`
	IsActive bool              `json:"is_active"`
	IsPaid   bool              `json:"is_paid_tool"`
	Config   stores.ToolConfig `json:"config"`
}

func (h *AdminToolsHandler) list(c *gin.Context) {
	rows, err := h.Store.ListTools(stores.ToolListInput{ActiveOnly: false})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load tools"})
		return
	}

	windowDays := 7
	now := time.Now().UTC()
	from := now.AddDate(0, 0, -windowDays+1)

	items := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		config := stores.ParseToolConfig(row.ConfigJSON)
		summary, err := h.Store.SummarizeToolMetrics(stores.ToolMetricsSummaryInput{
			ToolID: row.ID,
			From:   from,
			To:     now,
		})
		if err != nil {
			summary = stores.ToolMetricsSummary{}
		}
		items = append(items, gin.H{
			"id":           row.ID,
			"name":         row.Name,
			"slug":         row.Slug,
			"category":     row.Category,
			"is_active":    row.IsActive,
			"is_paid_tool": row.IsPaidTool,
			"config":       config,
			"metrics": gin.H{
				"window_days": windowDays,
				"summary":     summary,
			},
		})
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *AdminToolsHandler) update(c *gin.Context) {
	idRaw := strings.TrimSpace(c.Param("id"))
	toolID, err := strconv.ParseUint(idRaw, 10, 64)
	if err != nil || toolID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tool id"})
		return
	}

	var req adminToolUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	existing := stores.ToolModel{}
	if err := h.Store.DB().Where("id = ?", toolID).First(&existing).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "tool not found"})
		return
	}
	currentConfig := stores.ParseToolConfig(existing.ConfigJSON)
	if len(req.Config.Stages) == 0 {
		req.Config.Stages = currentConfig.Stages
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = existing.Name
	}
	category := strings.TrimSpace(req.Category)
	if category == "" {
		category = existing.Category
	}

	updated, err := h.Store.UpdateTool(stores.ToolUpdateInput{
		ToolID:    uint(toolID),
		Name:      name,
		Category:  category,
		IsActive:  req.IsActive,
		IsPaid:    req.IsPaid,
		Config:    req.Config,
		UpdatedAt: time.Now().UTC(),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update tool"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tool": gin.H{
			"id":           updated.ID,
			"name":         updated.Name,
			"slug":         updated.Slug,
			"category":     updated.Category,
			"is_active":    updated.IsActive,
			"is_paid_tool": updated.IsPaidTool,
			"config":       stores.ParseToolConfig(updated.ConfigJSON),
		},
	})
}
