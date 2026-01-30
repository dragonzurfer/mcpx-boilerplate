package routes

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/stores"
)

type AdminUsersHandler struct {
	Store *stores.Store
}

type adminUserResponse struct {
	ID        uint      `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Avatar    string    `json:"avatar"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type adminUserMetricsResponse struct {
	UserID       uint       `json:"user_id"`
	Score        int        `json:"score"`
	Stage        string     `json:"stage"`
	LastActiveAt *time.Time `json:"last_active_at"`
	Reads14d     int        `json:"reads_14d"`
	Completes14d int        `json:"completes_14d"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type adminEntitlementResponse struct {
	ID            uint      `json:"id"`
	UserID        uint      `json:"user_id"`
	PlanCode      string    `json:"plan_code"`
	Status        string    `json:"status"`
	StartAt       time.Time `json:"start_at"`
	EndAt         time.Time `json:"end_at"`
	LastPaymentID uint      `json:"last_payment_id"`
}

func (h *AdminUsersHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/users", h.list)
	rg.GET("/users/:id", h.detail)
}

func (h *AdminUsersHandler) list(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	page := parseIntDefault(c.Query("page"), 0)
	pageSize := parseIntDefault(c.Query("page_size"), 20)

	output, err := h.Store.ListUsers(stores.UserListInput{Query: query, Page: page, PageSize: pageSize})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load users"})
		return
	}

	items := make([]gin.H, 0, len(output.Users))
	for _, user := range output.Users {
		items = append(items, gin.H{
			"id":         user.ID,
			"email":      user.Email,
			"name":       user.Name,
			"avatar":     user.AvatarURL,
			"role":       user.Role,
			"status":     user.Status,
			"created_at": user.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "total": output.Total})
}

func (h *AdminUsersHandler) detail(c *gin.Context) {
	idRaw := strings.TrimSpace(c.Param("id"))
	id64, err := strconv.ParseUint(idRaw, 10, 64)
	if err != nil || id64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	user, err := h.Store.GetUserByID(uint(id64))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	metrics, _ := h.Store.GetUserMetrics(user.ID)
	entitlement, _ := h.Store.GetLatestEntitlement(user.ID)

	c.JSON(http.StatusOK, gin.H{
		"user":        buildAdminUserResponse(user),
		"metrics":     buildAdminUserMetricsResponse(metrics),
		"entitlement": buildAdminEntitlementResponse(entitlement),
	})
}

func buildAdminUserResponse(user *stores.UserModel) adminUserResponse {
	if user == nil {
		return adminUserResponse{}
	}
	return adminUserResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Avatar:    user.AvatarURL,
		Role:      user.Role,
		Status:    user.Status,
		CreatedAt: user.CreatedAt,
	}
}

func buildAdminUserMetricsResponse(metrics *stores.UserMetricsModel) *adminUserMetricsResponse {
	if metrics == nil {
		return nil
	}
	return &adminUserMetricsResponse{
		UserID:       metrics.UserID,
		Score:        metrics.Score,
		Stage:        metrics.Stage,
		LastActiveAt: metrics.LastActiveAt,
		Reads14d:     metrics.Reads14d,
		Completes14d: metrics.Completes14d,
		UpdatedAt:    metrics.UpdatedAt,
	}
}

func buildAdminEntitlementResponse(entitlement *stores.EntitlementModel) *adminEntitlementResponse {
	if entitlement == nil {
		return nil
	}
	return &adminEntitlementResponse{
		ID:            entitlement.ID,
		UserID:        entitlement.UserID,
		PlanCode:      entitlement.PlanCode,
		Status:        entitlement.Status,
		StartAt:       entitlement.StartAt,
		EndAt:         entitlement.EndAt,
		LastPaymentID: entitlement.LastPaymentID,
	}
}
