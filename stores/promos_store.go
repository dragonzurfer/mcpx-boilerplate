package stores

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type PromoListInput struct {
	Slot string
	Now  time.Time
}

type PromoWithVariants struct {
	Promo    PromoModel
	Variants []PromoVariantModel
}

func (s *Store) ListPromosWithVariants(input PromoListInput) ([]PromoWithVariants, error) {
	slot := strings.TrimSpace(input.Slot)
	if slot == "" {
		return []PromoWithVariants{}, nil
	}

	promos := []PromoModel{}
	query := s.db.Where("slot = ? AND status = ?", slot, PromoStatusActive)
	query = query.Where("(start_at IS NULL OR start_at <= ?) AND (end_at IS NULL OR end_at >= ?)", input.Now, input.Now)
	if err := query.Order("priority desc").Find(&promos).Error; err != nil {
		return nil, err
	}

	out := make([]PromoWithVariants, 0, len(promos))
	for _, promo := range promos {
		variants := []PromoVariantModel{}
		if err := s.db.Where("promo_id = ?", promo.ID).Find(&variants).Error; err != nil {
			return nil, err
		}
		out = append(out, PromoWithVariants{Promo: promo, Variants: variants})
	}
	return out, nil
}

type PromoMetricsInput struct {
	UserID   *uint
	AnonID   string
	PromoIDs []uint
	DayStart time.Time
}

type PromoMetricsOutput struct {
	Impressions map[uint]int
	Clicks      map[uint]int
	LastClickAt map[uint]*time.Time
}

func (s *Store) GetPromoMetrics(input PromoMetricsInput) (PromoMetricsOutput, error) {
	out := PromoMetricsOutput{
		Impressions: map[uint]int{},
		Clicks:      map[uint]int{},
		LastClickAt: map[uint]*time.Time{},
	}
	if len(input.PromoIDs) == 0 {
		return out, nil
	}

	queryImpressions := s.promoUserQuery(&PromoImpressionModel{}, input)
	queryImpressions = queryImpressions.Select("promo_id, count(*) as count").Group("promo_id")
	impressions := []promoCountRow{}
	if err := queryImpressions.Scan(&impressions).Error; err != nil {
		return out, err
	}
	for _, row := range impressions {
		out.Impressions[row.PromoID] = row.Count
	}

	queryClicks := s.promoUserQuery(&PromoClickModel{}, input)
	queryClicks = queryClicks.Select("promo_id, count(*) as count").Group("promo_id")
	clicks := []promoCountRow{}
	if err := queryClicks.Scan(&clicks).Error; err != nil {
		return out, err
	}
	for _, row := range clicks {
		out.Clicks[row.PromoID] = row.Count
	}

	lastClickRows := []promoLastClickRow{}
	queryLast := s.promoUserQuery(&PromoClickModel{}, input)
	queryLast = queryLast.Select("promo_id, max(created_at) as last_click_at").Group("promo_id")
	if err := queryLast.Scan(&lastClickRows).Error; err != nil {
		return out, err
	}
	for _, row := range lastClickRows {
		value := row.LastClickAt
		out.LastClickAt[row.PromoID] = &value
	}

	return out, nil
}

type promoCountRow struct {
	PromoID uint
	Count   int
}

type promoLastClickRow struct {
	PromoID     uint
	LastClickAt time.Time
}

func (s *Store) promoUserQuery(model interface{}, input PromoMetricsInput) *gorm.DB {
	query := s.db.Model(model).Where("promo_id IN ?", input.PromoIDs).Where("created_at >= ?", input.DayStart)
	if input.UserID != nil {
		query = query.Where("user_id = ?", *input.UserID)
	} else {
		query = query.Where("anon_id = ?", strings.TrimSpace(input.AnonID))
	}
	return query
}

type PromoDecisionInput struct {
	DecisionID string
	PromoID    *uint
	VariantID  *uint
	UserID     *uint
	AnonID     *string
	EntityType string
	EntityID   *uint
	PostID     *uint
	Slot       string
	CreatedAt  time.Time
}

func (s *Store) CreatePromoDecision(input PromoDecisionInput) error {
	if strings.TrimSpace(input.DecisionID) == "" {
		return gorm.ErrInvalidData
	}

	row := PromoDecisionModel{
		DecisionID: input.DecisionID,
		PromoID:    input.PromoID,
		VariantID:  input.VariantID,
		UserID:     input.UserID,
		AnonID:     input.AnonID,
		EntityType: strings.ToUpper(strings.TrimSpace(input.EntityType)),
		EntityID:   input.EntityID,
		PostID:     resolvePromoPostID(input.PostID, input.EntityType, input.EntityID),
		Slot:       strings.TrimSpace(input.Slot),
		CreatedAt:  input.CreatedAt,
	}
	return s.db.Create(&row).Error
}

type PromoInteractionInput struct {
	PromoID    uint
	VariantID  uint
	UserID     *uint
	AnonID     *string
	EntityType string
	EntityID   *uint
	PostID     *uint
	CreatedAt  time.Time
}

func (s *Store) LogPromoImpression(input PromoInteractionInput) error {
	row := PromoImpressionModel{
		PromoID:    input.PromoID,
		VariantID:  input.VariantID,
		UserID:     input.UserID,
		AnonID:     input.AnonID,
		EntityType: strings.ToUpper(strings.TrimSpace(input.EntityType)),
		EntityID:   input.EntityID,
		PostID:     resolvePromoPostID(input.PostID, input.EntityType, input.EntityID),
		CreatedAt:  input.CreatedAt,
	}
	return s.db.Create(&row).Error
}

func (s *Store) LogPromoClick(input PromoInteractionInput) error {
	row := PromoClickModel{
		PromoID:    input.PromoID,
		VariantID:  input.VariantID,
		UserID:     input.UserID,
		AnonID:     input.AnonID,
		EntityType: strings.ToUpper(strings.TrimSpace(input.EntityType)),
		EntityID:   input.EntityID,
		PostID:     resolvePromoPostID(input.PostID, input.EntityType, input.EntityID),
		CreatedAt:  input.CreatedAt,
	}
	return s.db.Create(&row).Error
}

func resolvePromoPostID(postID *uint, entityType string, entityID *uint) *uint {
	if postID != nil && *postID > 0 {
		return postID
	}
	if strings.EqualFold(entityType, "POST") && entityID != nil && *entityID > 0 {
		return entityID
	}
	return nil
}
