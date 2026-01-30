package stores

import "time"

type PostImpressionModel struct {
	ID        uint      `gorm:"primaryKey"`
	PostID    uint      `gorm:"index;index:uniq_post_impression_user,unique;index:uniq_post_impression_anon,unique"`
	UserID    *uint     `gorm:"index;index:uniq_post_impression_user,unique"`
	AnonID    *string   `gorm:"type:varchar(191);index;index:uniq_post_impression_anon,unique"`
	DayDate   time.Time `gorm:"type:date;index;index:uniq_post_impression_user,unique;index:uniq_post_impression_anon,unique"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (PostImpressionModel) TableName() string {
	return "post_impressions"
}

type PostDailyMetricModel struct {
	ID                uint      `gorm:"primaryKey"`
	PostID            uint      `gorm:"index;index:uniq_post_day,unique"`
	DayDate           time.Time `gorm:"type:date;index;index:uniq_post_day,unique"`
	TotalViews        int       `gorm:"not null;default:0"`
	UniqueImpressions int       `gorm:"not null;default:0"`
	Scroll25          int       `gorm:"not null;default:0"`
	Scroll50          int       `gorm:"not null;default:0"`
	Scroll75          int       `gorm:"not null;default:0"`
	Scroll90          int       `gorm:"not null;default:0"`
	Time15            int       `gorm:"not null;default:0"`
	Time45            int       `gorm:"not null;default:0"`
	Time90            int       `gorm:"not null;default:0"`
	Completes         int       `gorm:"not null;default:0"`
	PromoImpressions  int       `gorm:"not null;default:0"`
	PromoClicks       int       `gorm:"not null;default:0"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

func (PostDailyMetricModel) TableName() string {
	return "post_daily_metrics"
}

type PostPromoDailyMetricModel struct {
	ID          uint      `gorm:"primaryKey"`
	PostID      uint      `gorm:"index;index:uniq_post_promo_day,unique"`
	PromoID     uint      `gorm:"index;index:uniq_post_promo_day,unique"`
	VariantID   uint      `gorm:"index;index:uniq_post_promo_day,unique"`
	DayDate     time.Time `gorm:"type:date;index;index:uniq_post_promo_day,unique"`
	Impressions int       `gorm:"not null;default:0"`
	Clicks      int       `gorm:"not null;default:0"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (PostPromoDailyMetricModel) TableName() string {
	return "post_promo_daily_metrics"
}
