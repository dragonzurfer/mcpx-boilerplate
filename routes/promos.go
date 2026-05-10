package routes

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mcpx/boilerplate/middleware"
	"github.com/mcpx/boilerplate/services"
	"github.com/mcpx/boilerplate/stores"
)

type PromosHandler struct {
	Store *stores.Store
}

func (h *PromosHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/promos/decide", h.decide)
	rg.POST("/promos/impression", h.impression)
	rg.POST("/promos/click", h.click)
}

type promoDecisionResponse struct {
	DecisionID string        `json:"decision_id"`
	Promo      *promoPayload `json:"promo"`
}

type promoPayload struct {
	PromoID    uint                   `json:"promo_id"`
	VariantID  uint                   `json:"variant_id"`
	Slot       string                 `json:"slot"`
	Headline   string                 `json:"headline"`
	Body       string                 `json:"body"`
	CTAText    string                 `json:"cta_text"`
	CTAAction  string                 `json:"cta_action"`
	CTAPayload map[string]interface{} `json:"cta_payload"`
}

type promoDecisionContext struct {
	EntityType    string
	EntityID      uint
	PromosAllowed bool
	PostID        *uint
}

type promoContextResult struct {
	Context      promoDecisionContext
	StatusCode   int
	ErrorMessage string
}

func (h *PromosHandler) decide(c *gin.Context) {
	slot := strings.TrimSpace(c.Query("slot"))
	if slot == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slot is required"})
		return
	}

	contextResult := h.resolvePromoDecisionContext(c)
	if contextResult.ErrorMessage != "" {
		c.JSON(contextResult.StatusCode, gin.H{"error": contextResult.ErrorMessage})
		return
	}
	context := contextResult.Context

	user, _ := middleware.CurrentUser(c)
	anonID := strings.TrimSpace(c.Query("anon_id"))
	now := time.Now().UTC()
	entitlementActive := false
	if user != nil {
		entitlementActive = entitlementActiveForUser(h.Store, user.ID)
	}

	stage := resolveUserStage(h.Store, user)
	promos, err := h.Store.ListPromosWithVariants(stores.PromoListInput{Slot: slot, Now: now})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load promos"})
		return
	}

	candidates := make([]services.PromoCandidate, 0, len(promos))
	promoIDs := make([]uint, 0, len(promos))
	for _, item := range promos {
		eligibleStages := parseEligibleStages(item.Promo.EligibleStagesJSON)
		candidate := services.PromoCandidate{
			ID:                   item.Promo.ID,
			Code:                 item.Promo.Code,
			Slot:                 item.Promo.Slot,
			Status:               item.Promo.Status,
			Priority:             item.Promo.Priority,
			EligibleStages:       eligibleStages,
			CooldownHours:        item.Promo.CooldownHours,
			MaxImpressionsPerDay: item.Promo.MaxImpressionsPerDay,
			MaxClicksPerDay:      item.Promo.MaxClicksPerDay,
			StartAt:              item.Promo.StartAt,
			EndAt:                item.Promo.EndAt,
			Variants:             mapVariants(item.Variants),
		}
		candidates = append(candidates, candidate)
		promoIDs = append(promoIDs, item.Promo.ID)
	}

	metrics, err := h.Store.GetPromoMetrics(stores.PromoMetricsInput{
		UserID:   userIDPointer(user),
		AnonID:   anonID,
		PromoIDs: promoIDs,
		DayStart: startOfDay(time.Now().UTC()),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load promo metrics"})
		return
	}

	decision := services.DecidePromo(services.PromoDecisionInput{
		PromosAllowed:          context.PromosAllowed,
		UserStage:              stage,
		Slot:                   slot,
		EntitlementActive:      entitlementActive,
		Now:                    now,
		Promos:                 candidates,
		Metrics:                toPromoMetrics(metrics),
		GlobalDailyCap:         0,
		GlobalImpressionsToday: 0,
	})

	decisionID := uuid.NewString()
	decisionInput := stores.PromoDecisionInput{
		DecisionID: decisionID,
		UserID:     userIDPointer(user),
		AnonID:     anonPointer(anonID),
		EntityType: context.EntityType,
		EntityID:   uintPointer(context.EntityID),
		PostID:     context.PostID,
		Slot:       slot,
		CreatedAt:  now,
	}

	if decision.Promo == nil || decision.Variant == nil {
		_ = h.Store.CreatePromoDecision(decisionInput)
		c.JSON(http.StatusOK, promoDecisionResponse{DecisionID: decisionID, Promo: nil})
		return
	}

	payload := promoPayload{
		PromoID:    decision.Promo.ID,
		VariantID:  decision.Variant.ID,
		Slot:       decision.Promo.Slot,
		Headline:   decision.Variant.Headline,
		Body:       decision.Variant.Body,
		CTAText:    decision.Variant.CTAText,
		CTAAction:  decision.Variant.CTAAction,
		CTAPayload: parseCTAPayload(decision.Variant.CTAPayload),
	}

	promoID := decision.Promo.ID
	variantID := decision.Variant.ID
	decisionInput.PromoID = &promoID
	decisionInput.VariantID = &variantID
	_ = h.Store.CreatePromoDecision(decisionInput)

	c.JSON(http.StatusOK, promoDecisionResponse{DecisionID: decisionID, Promo: &payload})
}

func (h *PromosHandler) resolvePromoDecisionContext(c *gin.Context) promoContextResult {
	entityType := strings.ToUpper(strings.TrimSpace(c.Query("entity_type")))
	entityID := parseUintDefault(c.Query("entity_id"))
	legacyPostID := parseUintDefault(c.Query("post_id"))

	if entityType == "" && entityID == 0 && legacyPostID > 0 {
		entityType = "POST"
		entityID = legacyPostID
	}

	if entityType == "" || entityID == 0 {
		return promoContextResult{
			StatusCode:   http.StatusBadRequest,
			ErrorMessage: "slot and entity context are required",
		}
	}

	context := promoDecisionContext{
		EntityType:    entityType,
		EntityID:      entityID,
		PromosAllowed: true,
	}
	if strings.EqualFold(entityType, "POST") {
		context.PostID = uintPointer(entityID)
	}

	switch entityType {
	case "POST":
		post, _, err := h.Store.GetPostByID(stores.PostIDLookupInput{PostID: entityID, IncludeTags: false})
		if err != nil {
			return promoContextResult{
				StatusCode:   http.StatusNotFound,
				ErrorMessage: "post not found",
			}
		}
		context.PromosAllowed = strings.EqualFold(post.AccessLevel, stores.AccessLevelTrial)
	case "COURSE":
		_, err := h.Store.GetCourseByID(stores.CourseByIDLookupInput{CourseID: entityID})
		if err != nil {
			return promoContextResult{
				StatusCode:   http.StatusNotFound,
				ErrorMessage: "course not found",
			}
		}
	case "PROBLEM":
		_, err := h.Store.GetProblemByID(stores.ProblemByIDLookupInput{ProblemID: entityID})
		if err != nil {
			return promoContextResult{
				StatusCode:   http.StatusNotFound,
				ErrorMessage: "problem not found",
			}
		}
	default:
		return promoContextResult{
			StatusCode:   http.StatusBadRequest,
			ErrorMessage: "unsupported entity type",
		}
	}

	return promoContextResult{Context: context}
}

type promoInteractionRequest struct {
	DecisionID string `json:"decision_id"`
	PromoID    uint   `json:"promo_id"`
	VariantID  uint   `json:"variant_id"`
	EntityType string `json:"entity_type"`
	EntityID   uint   `json:"entity_id"`
	PostID     uint   `json:"post_id"`
	AnonID     string `json:"anon_id"`
}

func (h *PromosHandler) impression(c *gin.Context) {
	var req promoInteractionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	user, _ := middleware.CurrentUser(c)
	entity := resolvePromoInteractionEntity(req)
	if err := h.Store.LogPromoImpression(stores.PromoInteractionInput{
		PromoID:    req.PromoID,
		VariantID:  req.VariantID,
		UserID:     userIDPointer(user),
		AnonID:     anonPointer(req.AnonID),
		EntityType: entity.EntityType,
		EntityID:   entity.EntityID,
		PostID:     entity.PostID,
		CreatedAt:  time.Now().UTC(),
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to log impression"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (h *PromosHandler) click(c *gin.Context) {
	var req promoInteractionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	user, _ := middleware.CurrentUser(c)
	entity := resolvePromoInteractionEntity(req)
	if err := h.Store.LogPromoClick(stores.PromoInteractionInput{
		PromoID:    req.PromoID,
		VariantID:  req.VariantID,
		UserID:     userIDPointer(user),
		AnonID:     anonPointer(req.AnonID),
		EntityType: entity.EntityType,
		EntityID:   entity.EntityID,
		PostID:     entity.PostID,
		CreatedAt:  time.Now().UTC(),
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to log click"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type promoInteractionEntity struct {
	EntityType string
	EntityID   *uint
	PostID     *uint
}

func resolvePromoInteractionEntity(req promoInteractionRequest) promoInteractionEntity {
	entityType := strings.ToUpper(strings.TrimSpace(req.EntityType))
	entityID := uintPointer(req.EntityID)
	postID := uintPointer(req.PostID)
	if entityType == "" && req.PostID > 0 {
		entityType = "POST"
		entityID = postID
	}
	if strings.EqualFold(entityType, "POST") && postID == nil {
		postID = entityID
	}
	return promoInteractionEntity{
		EntityType: entityType,
		EntityID:   entityID,
		PostID:     postID,
	}
}

func mapVariants(variants []stores.PromoVariantModel) []services.PromoVariant {
	out := make([]services.PromoVariant, 0, len(variants))
	for _, variant := range variants {
		out = append(out, services.PromoVariant{
			ID:         variant.ID,
			Headline:   variant.Headline,
			Body:       variant.Body,
			CTAText:    variant.CTAText,
			CTAAction:  variant.CTAAction,
			CTAPayload: variant.CTAPayload,
			Weight:     variant.Weight,
		})
	}
	return out
}

func parseEligibleStages(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return []string{}
	}
	var stages []string
	if err := json.Unmarshal([]byte(raw), &stages); err != nil {
		return []string{}
	}
	return stages
}

func parseCTAPayload(raw string) map[string]interface{} {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil
	}
	return payload
}

func userIDPointer(user *stores.UserModel) *uint {
	if user == nil {
		return nil
	}
	return &user.ID
}

func anonPointer(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func uintPointer(value uint) *uint {
	if value == 0 {
		return nil
	}
	return &value
}

func startOfDay(now time.Time) time.Time {
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

func toPromoMetrics(metrics stores.PromoMetricsOutput) map[uint]services.PromoMetrics {
	out := map[uint]services.PromoMetrics{}
	for promoID, count := range metrics.Impressions {
		value := out[promoID]
		value.ImpressionsToday = count
		out[promoID] = value
	}
	for promoID, count := range metrics.Clicks {
		value := out[promoID]
		value.ClicksToday = count
		out[promoID] = value
	}
	for promoID, lastClick := range metrics.LastClickAt {
		value := out[promoID]
		value.LastClickAt = lastClick
		out[promoID] = value
	}
	return out
}

func resolveUserStage(store *stores.Store, user *stores.UserModel) string {
	if user == nil {
		return stores.FunnelStageNew
	}
	metrics, err := store.GetUserMetrics(user.ID)
	if err != nil || metrics == nil || metrics.Stage == "" {
		return stores.FunnelStageNew
	}
	return metrics.Stage
}

func entitlementActiveForUser(store *stores.Store, userID uint) bool {
	_, err := store.GetActiveEntitlement(stores.EntitlementLookupInput{UserID: userID, Now: time.Now().UTC()})
	return err == nil
}
