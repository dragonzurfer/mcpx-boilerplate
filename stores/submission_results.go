package stores

import "time"

type SubmissionResultModel struct {
	SubmissionID  uint   `gorm:"primaryKey"`
	OverallJSON   string `gorm:"type:longtext"`
	CompileJSON   string `gorm:"type:longtext"`
	TestsJSON     string `gorm:"type:longtext"`
	TimingJSON    string `gorm:"type:longtext"`
	ArtifactsJSON string `gorm:"type:longtext"`
	ReceiptJSON   string `gorm:"type:longtext"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (SubmissionResultModel) TableName() string {
	return "submission_results"
}
