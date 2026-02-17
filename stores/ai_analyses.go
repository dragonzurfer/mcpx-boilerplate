package stores

import "time"

type AIAnalysisModel struct {
	AnalysisID   string `gorm:"primaryKey;type:varchar(64)"`
	SubmissionID uint   `gorm:"index"`
	UserID       uint   `gorm:"index"`
	CodeHash     string `gorm:"type:varchar(128);index"`
	ResultHash   string `gorm:"type:varchar(128);index"`
	PolicyJSON   string `gorm:"type:longtext"`
	Status       string `gorm:"type:varchar(16);index"`
	ResponseJSON string `gorm:"type:longtext"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (AIAnalysisModel) TableName() string {
	return "ai_analyses"
}
