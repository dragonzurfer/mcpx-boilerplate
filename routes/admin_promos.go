package routes

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/stores"
	"gorm.io/gorm"
)

type AdminPromosHandler struct {
	Store *stores.Store
}

type promoVariantRequest struct {
	Headline   string                 `json:"headline"`
	Body       string                 `json:"body"`
	CTAText    string                 `json:"cta_text"`
	CTAAction  string                 `json:"cta_action"`
	CTAPayload map[string]interface{} `json:"cta_payload"`
	Weight     int                    `json:"weight"`
}

type promoRequest struct {
	Code                 string                `json:"code"`
	Name                 string                `json:"name"`
	Slot                 string                `json:"slot"`
	Status               string                `json:"status"`
	Priority             int                   `json:"priority"`
	EligibleStages       []string              `json:"eligible_stages"`
	CooldownHours        int                   `json:"cooldown_hours"`
	MaxImpressionsPerDay int                   `json:"max_impressions_per_day"`
	MaxClicksPerDay      int                   `json:"max_clicks_per_day"`
	StartAt              *time.Time            `json:"start_at"`
	EndAt                *time.Time            `json:"end_at"`
	Variants             []promoVariantRequest `json:"variants"`
}

func (h *AdminPromosHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/promos", h.list)
	rg.GET("/promos/:id", h.get)
	rg.POST("/promos", h.create)
	rg.PUT("/promos/:id", h.update)
}

func (h *AdminPromosHandler) list(c *gin.Context) {
	rows := []stores.PromoModel{}
	if err := h.Store.DB().Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load promos"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"promos": rows})
}

func (h *AdminPromosHandler) get(c *gin.Context) {
	idRaw := strings.TrimSpace(c.Param("id"))
	id64, err := strconv.ParseUint(idRaw, 10, 64)
	if err != nil || id64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var promo stores.PromoModel
	err = h.Store.DB().First(&promo, id64).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "promo not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load promo"})
		return
	}

	var variants []stores.PromoVariantModel
	if err := h.Store.DB().Where("promo_id = ?", promo.ID).Find(&variants).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load promo variants"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"promo": promo, "variants": variants})
}

func (h *AdminPromosHandler) create(c *gin.Context) {
	var req promoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if err := validatePromoRequest(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	promo, err := h.upsertPromo(0, req)
	if err != nil {
		if isDuplicateKey(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "promo code already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create promo"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"promo": promo})
}

func (h *AdminPromosHandler) update(c *gin.Context) {
	idRaw := strings.TrimSpace(c.Param("id"))
	id64, err := strconv.ParseUint(idRaw, 10, 64)
	if err != nil || id64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req promoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if err := validatePromoRequest(req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	promo, err := h.upsertPromo(uint(id64), req)
	if err != nil {
		if isDuplicateKey(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "promo code already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update promo"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"promo": promo})
}

func (h *AdminPromosHandler) upsertPromo(promoID uint, req promoRequest) (*stores.PromoModel, error) {
	eligibleJSON, _ := json.Marshal(req.EligibleStages)
	payload := map[string]interface{}{
		"code":                    strings.TrimSpace(req.Code),
		"name":                    strings.TrimSpace(req.Name),
		"slot":                    strings.TrimSpace(req.Slot),
		"status":                  strings.TrimSpace(req.Status),
		"priority":                req.Priority,
		"eligible_stages_json":    string(eligibleJSON),
		"cooldown_hours":          req.CooldownHours,
		"max_impressions_per_day": req.MaxImpressionsPerDay,
		"max_clicks_per_day":      req.MaxClicksPerDay,
		"start_at":                req.StartAt,
		"end_at":                  req.EndAt,
		"updated_at":              time.Now().UTC(),
	}

	var promo stores.PromoModel
	if promoID == 0 {
		promo = stores.PromoModel{
			Code:                 payload["code"].(string),
			Name:                 payload["name"].(string),
			Slot:                 payload["slot"].(string),
			Status:               payload["status"].(string),
			Priority:             req.Priority,
			EligibleStagesJSON:   string(eligibleJSON),
			CooldownHours:        req.CooldownHours,
			MaxImpressionsPerDay: req.MaxImpressionsPerDay,
			MaxClicksPerDay:      req.MaxClicksPerDay,
			StartAt:              req.StartAt,
			EndAt:                req.EndAt,
		}
		if err := h.Store.DB().Create(&promo).Error; err != nil {
			return nil, err
		}
	} else {
		if err := h.Store.DB().First(&promo, promoID).Error; err != nil {
			return nil, err
		}
		if err := h.Store.DB().Model(&promo).Updates(payload).Error; err != nil {
			return nil, err
		}
		promo.Code = payload["code"].(string)
		promo.Name = payload["name"].(string)
		promo.Slot = payload["slot"].(string)
		promo.Status = payload["status"].(string)
		promo.Priority = req.Priority
		promo.EligibleStagesJSON = string(eligibleJSON)
		promo.CooldownHours = req.CooldownHours
		promo.MaxImpressionsPerDay = req.MaxImpressionsPerDay
		promo.MaxClicksPerDay = req.MaxClicksPerDay
		promo.StartAt = req.StartAt
		promo.EndAt = req.EndAt
		promo.UpdatedAt = payload["updated_at"].(time.Time)
	}

	if err := h.Store.DB().Where("promo_id = ?", promo.ID).Delete(&stores.PromoVariantModel{}).Error; err != nil {
		return nil, err
	}

	variants := make([]stores.PromoVariantModel, 0, len(req.Variants))
	for _, variant := range req.Variants {
		payload := ""
		if len(variant.CTAPayload) > 0 {
			if raw, err := json.Marshal(variant.CTAPayload); err == nil {
				payload = string(raw)
			}
		}
		variants = append(variants, stores.PromoVariantModel{
			PromoID:    promo.ID,
			Headline:   variant.Headline,
			Body:       variant.Body,
			CTAText:    variant.CTAText,
			CTAAction:  variant.CTAAction,
			CTAPayload: payload,
			Weight:     variant.Weight,
		})
	}
	if len(variants) > 0 {
		if err := h.Store.DB().Create(&variants).Error; err != nil {
			return nil, err
		}
	}

	return &promo, nil
}

func validatePromoRequest(req promoRequest) error {
	if strings.TrimSpace(req.Code) == "" {
		return errInvalid("code is required")
	}
	if strings.TrimSpace(req.Name) == "" {
		return errInvalid("name is required")
	}
	if strings.TrimSpace(req.Slot) == "" {
		return errInvalid("slot is required")
	}
	if strings.TrimSpace(req.Status) == "" {
		return errInvalid("status is required")
	}
	if len(req.Variants) == 0 {
		return errInvalid("at least one variant is required")
	}
	for index, variant := range req.Variants {
		label := fmt.Sprintf("variant %d", index+1)
		if strings.TrimSpace(variant.Headline) == "" {
			return errInvalid(label + " headline is required")
		}
		if strings.TrimSpace(variant.Body) == "" {
			return errInvalid(label + " body is required")
		}
		if strings.TrimSpace(variant.CTAText) == "" {
			return errInvalid(label + " CTA text is required")
		}
		if strings.TrimSpace(variant.CTAAction) == "" {
			return errInvalid(label + " CTA action is required")
		}
	}
	return nil
}

func isDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "Duplicate entry") || strings.Contains(message, "Error 1062")
}

type errInvalid string

func (e errInvalid) Error() string {
	return string(e)
}
