package routes

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/core"
)

type AdminHandler struct {
	Store      *core.Store
	Logger     *log.Logger
	ProjectKey string
}

type ipRuleRequest struct {
	RuleType    string `json:"ruleType" binding:"required"`
	CIDR        string `json:"cidr" binding:"required"`
	Enabled     *bool  `json:"enabled"`
	Description string `json:"description"`
}

func (h *AdminHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/ip-rules", h.listIPRules)
	rg.POST("/ip-rules", h.upsertIPRule)
	rg.DELETE("/ip-rules/:id", h.deleteIPRule)
}

func (h *AdminHandler) listIPRules(c *gin.Context) {
	rules, err := h.Store.ListIPRules(h.ProjectKey)
	if err != nil {
		h.Logger.Printf("error [admin/ip-rules]: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load ip rules"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

func (h *AdminHandler) upsertIPRule(c *gin.Context) {
	var req ipRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	rule, err := h.Store.UpsertIPRule(h.ProjectKey, req.RuleType, req.CIDR, req.Description, enabled)
	if err != nil {
		h.Logger.Printf("error [admin/ip-rules]: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to update ip rule"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"rule": rule})
}

func (h *AdminHandler) deleteIPRule(c *gin.Context) {
	idRaw := c.Param("id")
	id64, err := strconv.ParseUint(idRaw, 10, 64)
	if err != nil || id64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := h.Store.DeleteIPRule(h.ProjectKey, uint(id64)); err != nil {
		h.Logger.Printf("error [admin/ip-rules/delete]: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete ip rule"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
