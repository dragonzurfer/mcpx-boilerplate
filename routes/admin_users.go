package routes

import (
	"encoding/json"
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

type adminUserListItemResponse struct {
	ID           uint       `json:"id"`
	Email        string     `json:"email"`
	Name         string     `json:"name"`
	Avatar       string     `json:"avatar"`
	Role         string     `json:"role"`
	Status       string     `json:"status"`
	Stage        string     `json:"stage"`
	LastActiveAt *time.Time `json:"last_active_at"`
	CreatedAt    time.Time  `json:"created_at"`
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

type adminUserEventResponse struct {
	ID         uint64                 `json:"id"`
	EventType  string                 `json:"event_type"`
	EntityType string                 `json:"entity_type"`
	EntityID   *uint                  `json:"entity_id"`
	Metadata   map[string]interface{} `json:"metadata"`
	CreatedAt  time.Time              `json:"created_at"`
}

type adminPromoActivityResponse struct {
	ActivityType string    `json:"activity_type"`
	ActivityID   uint      `json:"activity_id"`
	PromoID      *uint     `json:"promo_id"`
	VariantID    *uint     `json:"variant_id"`
	DecisionID   string    `json:"decision_id"`
	Slot         string    `json:"slot"`
	EntityType   string    `json:"entity_type"`
	EntityID     *uint     `json:"entity_id"`
	CreatedAt    time.Time `json:"created_at"`
}

func (h *AdminUsersHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/users", h.list)
	rg.GET("/users/:id", h.detail)
	rg.GET("/users/:id/activity", h.activity)
}

func (h *AdminUsersHandler) list(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	stage := strings.TrimSpace(c.Query("stage"))
	page := parseIntDefault(c.Query("page"), 0)
	pageSize := parseIntDefault(c.Query("page_size"), 20)
	page = normalizePage(page)
	pageSize = normalizePageSize(pageSize)

	output, err := h.Store.ListUsers(stores.UserListInput{
		Query:    query,
		Stage:    stage,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load users"})
		return
	}

	items := make([]adminUserListItemResponse, 0, len(output.Users))
	for _, user := range output.Users {
		metrics := loadAdminUserMetrics(h.Store, user.ID)
		items = append(items, buildAdminUserListItem(user, metrics))
	}

	c.JSON(http.StatusOK, gin.H{
		"items":     items,
		"total":     output.Total,
		"page":      page,
		"page_size": pageSize,
	})
}

func (h *AdminUsersHandler) detail(c *gin.Context) {
	userID, err := parseAdminUserID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	user, err := h.Store.GetUserByID(userID)
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

func (h *AdminUsersHandler) activity(c *gin.Context) {
	userID, err := parseAdminUserID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	_, err = h.Store.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	eventsPage := normalizePage(parseIntDefault(c.Query("events_page"), 0))
	eventsPageSize := normalizePageSize(parseIntDefault(c.Query("events_page_size"), 20))
	promosPage := normalizePage(parseIntDefault(c.Query("promos_page"), 0))
	promosPageSize := normalizePageSize(parseIntDefault(c.Query("promos_page_size"), 20))

	eventsOutput, err := h.Store.ListUserEvents(stores.UserEventListInput{
		UserID:   userID,
		Page:     eventsPage,
		PageSize: eventsPageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load user events"})
		return
	}

	promoOutput, err := h.Store.ListUserPromoActivities(stores.UserPromoActivityListInput{
		UserID:   userID,
		Page:     promosPage,
		PageSize: promosPageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load promo activities"})
		return
	}

	eventItems := make([]adminUserEventResponse, 0, len(eventsOutput.Items))
	for _, item := range eventsOutput.Items {
		eventItems = append(eventItems, buildAdminUserEventResponse(item))
	}

	promoItems := make([]adminPromoActivityResponse, 0, len(promoOutput.Items))
	for _, item := range promoOutput.Items {
		promoItems = append(promoItems, buildAdminPromoActivityResponse(item))
	}

	c.JSON(http.StatusOK, gin.H{
		"events": gin.H{
			"items":     eventItems,
			"total":     eventsOutput.Total,
			"page":      eventsPage,
			"page_size": eventsPageSize,
		},
		"promo_activities": gin.H{
			"items":     promoItems,
			"total":     promoOutput.Total,
			"page":      promosPage,
			"page_size": promosPageSize,
		},
	})
}

func buildAdminUserListItem(user stores.UserModel, metrics *stores.UserMetricsModel) adminUserListItemResponse {
	stage := ""
	var lastActiveAt *time.Time

	if metrics != nil {
		stage = metrics.Stage
		lastActiveAt = metrics.LastActiveAt
	}

	return adminUserListItemResponse{
		ID:           user.ID,
		Email:        user.Email,
		Name:         user.Name,
		Avatar:       user.AvatarURL,
		Role:         user.Role,
		Status:       user.Status,
		Stage:        stage,
		LastActiveAt: lastActiveAt,
		CreatedAt:    user.CreatedAt,
	}
}

func loadAdminUserMetrics(store *stores.Store, userID uint) *stores.UserMetricsModel {
	metrics, err := store.GetUserMetrics(userID)
	if err != nil {
		return nil
	}
	return metrics
}

func buildAdminUserEventResponse(model stores.EventModel) adminUserEventResponse {
	return adminUserEventResponse{
		ID:         model.ID,
		EventType:  model.EventType,
		EntityType: model.EntityType,
		EntityID:   model.EntityID,
		Metadata:   parseAdminEventMetadata(model.Metadata),
		CreatedAt:  model.CreatedAt,
	}
}

func buildAdminPromoActivityResponse(model stores.UserPromoActivityModel) adminPromoActivityResponse {
	return adminPromoActivityResponse{
		ActivityType: model.ActivityType,
		ActivityID:   model.ActivityID,
		PromoID:      model.PromoID,
		VariantID:    model.VariantID,
		DecisionID:   model.DecisionID,
		Slot:         model.Slot,
		EntityType:   model.EntityType,
		EntityID:     model.EntityID,
		CreatedAt:    model.CreatedAt,
	}
}

func parseAdminUserID(rawID string) (uint, error) {
	idRaw := strings.TrimSpace(rawID)
	id64, err := strconv.ParseUint(idRaw, 10, 64)
	if err != nil || id64 == 0 {
		return 0, strconv.ErrSyntax
	}
	return uint(id64), nil
}

func parseAdminEventMetadata(raw string) map[string]interface{} {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	value := map[string]interface{}{}
	if err := json.Unmarshal([]byte(trimmed), &value); err != nil {
		return nil
	}
	return value
}

func normalizePage(page int) int {
	if page < 0 {
		return 0
	}
	return page
}

func normalizePageSize(pageSize int) int {
	if pageSize <= 0 {
		return 20
	}
	if pageSize > 100 {
		return 100
	}
	return pageSize
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
