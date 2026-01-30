package stores

import "time"

type PromoModel struct {
	ID                 uint      `gorm:"primaryKey"`
	Code               string    `gorm:"type:varchar(64);uniqueIndex"`
	Name               string    `gorm:"type:varchar(191)"`
	Slot               string    `gorm:"type:varchar(32);index"`
	Status             string    `gorm:"type:varchar(16);index"`
	Priority           int       `gorm:"not null;default:0"`
	EligibleStagesJSON string    `gorm:"type:json"`
	CooldownHours      int       `gorm:"not null;default:0"`
	MaxImpressionsPerDay int     `gorm:"not null;default:0"`
	MaxClicksPerDay    int       `gorm:"not null;default:0"`
	StartAt            *time.Time
	EndAt              *time.Time
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (PromoModel) TableName() string {
	return "promos"
}

type PromoVariantModel struct {
	ID        uint      `gorm:"primaryKey"`
	PromoID   uint      `gorm:"index"`
	Headline  string    `gorm:"type:varchar(255)"`
	Body      string    `gorm:"type:longtext"`
	CTAText   string    `gorm:"type:varchar(191)"`
	CTAAction string    `gorm:"type:varchar(64)"`
	CTAPayload string   `gorm:"type:json"`
	Weight    int       `gorm:"not null;default:1"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (PromoVariantModel) TableName() string {
	return "promo_variants"
}

type PromoDecisionModel struct {
	ID        uint      `gorm:"primaryKey"`
	DecisionID string   `gorm:"type:varchar(64);uniqueIndex"`
	PromoID   *uint     `gorm:"index"`
	VariantID *uint     `gorm:"index"`
	UserID    *uint     `gorm:"index"`
	AnonID    *string   `gorm:"type:varchar(191);index"`
	PostID    *uint     `gorm:"index"`
	Slot      string    `gorm:"type:varchar(32);index"`
	CreatedAt time.Time
}

func (PromoDecisionModel) TableName() string {
	return "promo_decisions"
}

type PromoImpressionModel struct {
	ID        uint      `gorm:"primaryKey"`
	PromoID   uint      `gorm:"index"`
	VariantID uint      `gorm:"index"`
	UserID    *uint     `gorm:"index"`
	AnonID    *string   `gorm:"type:varchar(191);index"`
	PostID    *uint     `gorm:"index"`
	CreatedAt time.Time `gorm:"index"`
}

func (PromoImpressionModel) TableName() string {
	return "promo_impressions"
}

type PromoClickModel struct {
	ID        uint      `gorm:"primaryKey"`
	PromoID   uint      `gorm:"index"`
	VariantID uint      `gorm:"index"`
	UserID    *uint     `gorm:"index"`
	AnonID    *string   `gorm:"type:varchar(191);index"`
	PostID    *uint     `gorm:"index"`
	CreatedAt time.Time `gorm:"index"`
}

func (PromoClickModel) TableName() string {
	return "promo_clicks"
}
