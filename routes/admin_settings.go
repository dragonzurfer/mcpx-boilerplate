package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/stores"
)

type AdminSettingsHandler struct {
	Store *stores.Store
}

func (h *AdminSettingsHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/settings/site", h.getSite)
	rg.PUT("/settings/site", h.updateSite)
}

type siteSettingsRequest struct {
	SiteName              string `json:"site_name"`
	SiteURL               string `json:"site_url"`
	DefaultOgImageURL     string `json:"default_og_image_url"`
	PrimaryColor          string `json:"primary_color"`
	TwitterSite           string `json:"twitter_site"`
	TwitterCreator        string `json:"twitter_creator"`
	GoogleVerification    string `json:"google_verification"`
	BingVerification      string `json:"bing_verification"`
	PinterestVerification string `json:"pinterest_verification"`
}

func (h *AdminSettingsHandler) getSite(c *gin.Context) {
	settings, err := h.Store.GetSiteSettings()
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "settings not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"settings": settings})
}

func (h *AdminSettingsHandler) updateSite(c *gin.Context) {
	var req siteSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	settings, err := h.Store.UpdateSiteSettings(stores.SiteSettingsInput{
		SiteName:              req.SiteName,
		SiteURL:               req.SiteURL,
		DefaultOgImageURL:     req.DefaultOgImageURL,
		PrimaryColor:          req.PrimaryColor,
		TwitterSite:           req.TwitterSite,
		TwitterCreator:        req.TwitterCreator,
		GoogleVerification:    req.GoogleVerification,
		BingVerification:      req.BingVerification,
		PinterestVerification: req.PinterestVerification,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update settings"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"settings": settings})
}
