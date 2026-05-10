package stores

import "time"

type UserEventListInput struct {
	UserID   uint
	Page     int
	PageSize int
}

type UserEventListOutput struct {
	Items []EventModel
	Total int64
}

func (s *Store) ListUserEvents(input UserEventListInput) (UserEventListOutput, error) {
	query := s.db.Model(&EventModel{}).Where("user_id = ?", input.UserID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return UserEventListOutput{}, err
	}

	pageSize := clampPageSize(input.PageSize)
	offset := clampPage(input.Page) * pageSize
	items := []EventModel{}
	if err := query.Order("created_at desc, id desc").Limit(pageSize).Offset(offset).Find(&items).Error; err != nil {
		return UserEventListOutput{}, err
	}

	return UserEventListOutput{
		Items: items,
		Total: total,
	}, nil
}

type UserPromoActivityListInput struct {
	UserID   uint
	Page     int
	PageSize int
}

type UserPromoActivityModel struct {
	ActivityType string    `gorm:"column:activity_type"`
	ActivityID   uint      `gorm:"column:activity_id"`
	PromoID      *uint     `gorm:"column:promo_id"`
	VariantID    *uint     `gorm:"column:variant_id"`
	DecisionID   string    `gorm:"column:decision_id"`
	Slot         string    `gorm:"column:slot"`
	EntityType   string    `gorm:"column:entity_type"`
	EntityID     *uint     `gorm:"column:entity_id"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

type UserPromoActivityListOutput struct {
	Items []UserPromoActivityModel
	Total int64
}

func (s *Store) ListUserPromoActivities(input UserPromoActivityListInput) (UserPromoActivityListOutput, error) {
	total, err := s.countAllUserPromoActivities(input.UserID)
	if err != nil {
		return UserPromoActivityListOutput{}, err
	}

	pageSize := clampPageSize(input.PageSize)
	offset := clampPage(input.Page) * pageSize
	items := []UserPromoActivityModel{}
	err = s.db.Raw(userPromoActivityQuery, input.UserID, input.UserID, input.UserID, pageSize, offset).Scan(&items).Error
	if err != nil {
		return UserPromoActivityListOutput{}, err
	}

	return UserPromoActivityListOutput{
		Items: items,
		Total: total,
	}, nil
}

func (s *Store) countAllUserPromoActivities(userID uint) (int64, error) {
	decisionCount, err := s.countUserPromoRows(&PromoDecisionModel{}, userID)
	if err != nil {
		return 0, err
	}

	impressionCount, err := s.countUserPromoRows(&PromoImpressionModel{}, userID)
	if err != nil {
		return 0, err
	}

	clickCount, err := s.countUserPromoRows(&PromoClickModel{}, userID)
	if err != nil {
		return 0, err
	}

	return decisionCount + impressionCount + clickCount, nil
}

func (s *Store) countUserPromoRows(model interface{}, userID uint) (int64, error) {
	var count int64
	err := s.db.Model(model).Where("user_id = ?", userID).Count(&count).Error
	return count, err
}

const userPromoActivityQuery = `
SELECT
  activity_type,
  activity_id,
  promo_id,
  variant_id,
  decision_id,
  slot,
  entity_type,
  entity_id,
  created_at
FROM (
  SELECT
    'DECISION' AS activity_type,
    id AS activity_id,
    promo_id,
    variant_id,
    decision_id,
    slot,
    entity_type,
    entity_id,
    created_at
  FROM promo_decisions
  WHERE user_id = ?
  UNION ALL
  SELECT
    'IMPRESSION' AS activity_type,
    id AS activity_id,
    promo_id,
    variant_id,
    '' AS decision_id,
    '' AS slot,
    entity_type,
    entity_id,
    created_at
  FROM promo_impressions
  WHERE user_id = ?
  UNION ALL
  SELECT
    'CLICK' AS activity_type,
    id AS activity_id,
    promo_id,
    variant_id,
    '' AS decision_id,
    '' AS slot,
    entity_type,
    entity_id,
    created_at
  FROM promo_clicks
  WHERE user_id = ?
) user_promo_activities
ORDER BY created_at DESC, activity_id DESC
LIMIT ? OFFSET ?
`
