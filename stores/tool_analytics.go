package stores

import "time"

type ToolDailyMetricModel struct {
	ID         uint      `gorm:"primaryKey"`
	ToolID     uint      `gorm:"index;index:idx_tool_day_event,unique"`
	DayDate    time.Time `gorm:"index;index:idx_tool_day_event,unique"`
	EventName  string    `gorm:"type:varchar(64);index;index:idx_tool_day_event,unique"`
	TotalCount int       `gorm:"not null;default:0"`
	AudioCount int       `gorm:"not null;default:0"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (ToolDailyMetricModel) TableName() string {
	return "tool_daily_metrics"
}
