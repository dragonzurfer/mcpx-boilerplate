package stores

import "time"

type ToolModel struct {
	ID         uint   `gorm:"primaryKey"`
	Name       string `gorm:"type:varchar(191)"`
	Slug       string `gorm:"type:varchar(191);uniqueIndex"`
	Category   string `gorm:"type:varchar(64)"`
	IsActive   bool   `gorm:"default:true"`
	IsPaidTool bool   `gorm:"default:false"`
	ConfigJSON string `gorm:"type:json"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (ToolModel) TableName() string {
	return "tools"
}

type ToolUsageModel struct {
	ID             uint   `gorm:"primaryKey"`
	UserID         uint   `gorm:"index;index:uniq_user_tool,unique"`
	ToolID         uint   `gorm:"index;index:uniq_user_tool,unique"`
	UsageStateJSON string `gorm:"type:json"`
	FirstUsedAt    time.Time
	LastUsedAt     time.Time
	TotalSessions  int  `gorm:"not null;default:0"`
	IsConverted    bool `gorm:"default:false"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (ToolUsageModel) TableName() string {
	return "tool_usages"
}

type ToolEventModel struct {
	ID        uint      `gorm:"primaryKey"`
	ToolID    uint      `gorm:"index"`
	UserID    uint      `gorm:"index"`
	EventName string    `gorm:"type:varchar(64);index"`
	Metadata  string    `gorm:"type:json"`
	CreatedAt time.Time `gorm:"index"`
}

func (ToolEventModel) TableName() string {
	return "tool_events"
}
