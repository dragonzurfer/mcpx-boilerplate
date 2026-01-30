package stores

import "time"

type UserMetricsModel struct {
	UserID       uint      `gorm:"primaryKey"`
	Score        int       `gorm:"not null;default:0"`
	Stage        string    `gorm:"type:varchar(32);index"`
	LastActiveAt *time.Time
	Reads14d     int       `gorm:"not null;default:0"`
	Completes14d int       `gorm:"not null;default:0"`
	UpdatedAt    time.Time
}

func (UserMetricsModel) TableName() string {
	return "user_metrics"
}

type FunnelConfigModel struct {
	ID                  uint      `gorm:"primaryKey"`
	ScoringWindowDays   int       `gorm:"not null;default:14"`
	DecayEnabled        bool      `gorm:"not null;default:false"`
	DailyDecayFactor    float64   `gorm:"not null;default:1"`
	DormantDaysThreshold int      `gorm:"not null;default:21"`
	UpdatedAt           time.Time
}

func (FunnelConfigModel) TableName() string {
	return "funnel_config"
}

type FunnelEventWeightModel struct {
	EventType string `gorm:"primaryKey;type:varchar(64)"`
	Weight    int    `gorm:"not null;default:0"`
	Enabled   bool   `gorm:"not null;default:true"`
}

func (FunnelEventWeightModel) TableName() string {
	return "funnel_event_weights"
}

type FunnelStageThresholdModel struct {
	Stage    string `gorm:"primaryKey;type:varchar(32)"`
	MinScore int    `gorm:"not null;default:0"`
	MaxScore *int
	Enabled  bool `gorm:"not null;default:true"`
}

func (FunnelStageThresholdModel) TableName() string {
	return "funnel_stage_thresholds"
}
