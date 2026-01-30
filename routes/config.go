package routes

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/stores"
)

// ConfigResponse bundles client-side configuration.
type ConfigResponse struct {
	AppName        string `json:"appName"`
	AppKey         string `json:"appKey"`
	GoogleClientID string `json:"googleClientId"`
	RazorpayKeyID  string `json:"razorpayKeyId"`
	SiteName       string `json:"siteName"`
	SiteURL        string `json:"siteUrl"`
	PrimaryColor   string `json:"primaryColor"`
}

const (
	defaultSiteName     = "explore"
	defaultSiteURL      = "https://explore.mcpx.in"
	defaultPrimaryColor = "#38bdf8"
)

// RegisterConfig routes a GET /config endpoint without requiring auth.
func RegisterConfig(r *gin.Engine, store *stores.Store) {
	r.GET("/config", func(c *gin.Context) {
		settings, _ := store.GetSiteSettings()

		cfg := ConfigResponse{
			AppName:        strings.TrimSpace(os.Getenv("APP_NAME")),
			AppKey:         strings.TrimSpace(os.Getenv("APP_KEY")),
			GoogleClientID: strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")),
			RazorpayKeyID:  strings.TrimSpace(os.Getenv("RAZORPAY_KEY_ID")),
			SiteName:       resolveSiteName(settings),
			SiteURL:        resolveSiteURL(settings),
			PrimaryColor:   resolvePrimaryColor(settings),
		}
		c.JSON(http.StatusOK, cfg)
	})
}

func resolveSiteName(settings *stores.SiteSettingsModel) string {
	if settings == nil {
		return defaultSiteName
	}
	name := strings.TrimSpace(settings.SiteName)
	if name == "" {
		return defaultSiteName
	}
	return name
}

func resolveSiteURL(settings *stores.SiteSettingsModel) string {
	if settings == nil {
		return defaultSiteURL
	}
	url := strings.TrimSpace(settings.SiteURL)
	if url == "" {
		return defaultSiteURL
	}
	return strings.TrimRight(url, "/")
}

func resolvePrimaryColor(settings *stores.SiteSettingsModel) string {
	if settings == nil {
		return defaultPrimaryColor
	}
	color := strings.TrimSpace(settings.PrimaryColor)
	if color == "" {
		return defaultPrimaryColor
	}
	return color
}
