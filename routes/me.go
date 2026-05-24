package routes

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/middleware"
	"github.com/mcpx/boilerplate/stores"
)

type MeHandler struct {
	Store *stores.Store
}

func (h *MeHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/me", h.me)
}

func (h *MeHandler) me(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	metrics, _ := h.Store.GetUserMetrics(user.ID)
	activeEntitlement, _ := h.Store.GetActiveEntitlement(stores.EntitlementLookupInput{UserID: user.ID, Now: time.Now().UTC()})
	entitlement := activeEntitlement
	if entitlement == nil {
		entitlement, _ = h.Store.GetLatestEntitlement(user.ID)
	}
	entitlementPayload := buildEntitlementPayload(entitlement)

	stage := resolveStage(metrics, activeEntitlement, h.Store, user.ID)

	c.JSON(http.StatusOK, gin.H{
		"user": gin.H{
			"id":                 user.ID,
			"email":              user.Email,
			"name":               user.Name,
			"avatar":             user.AvatarURL,
			"role":               user.Role,
			"phone_country_code": user.PhoneCountryCode,
			"phone_e164":         user.PhoneE164,
		},
		"stage":       stage,
		"entitlement": entitlementPayload,
	})
}

func resolveStage(metrics *stores.UserMetricsModel, entitlement *stores.EntitlementModel, store *stores.Store, userID uint) string {
	if entitlement != nil && entitlement.Status == stores.EntitlementStatusActive {
		return stores.FunnelStagePaidActive
	}

	latest, err := store.GetLatestEntitlement(userID)
	if err == nil && latest.Status == stores.EntitlementStatusExpired {
		return stores.FunnelStagePaidExpired
	}

	if metrics != nil && metrics.Stage != "" {
		return metrics.Stage
	}
	return stores.FunnelStageNew
}

func buildEntitlementPayload(entitlement *stores.EntitlementModel) gin.H {
	if entitlement == nil {
		return nil
	}
	return gin.H{
		"plan_code": entitlement.PlanCode,
		"status":    entitlement.Status,
		"start_at":  entitlement.StartAt,
		"end_at":    entitlement.EndAt,
	}
}
