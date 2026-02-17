package stores

import "time"

type SubmissionModel struct {
	ID         uint      `gorm:"primaryKey"`
	UserID     uint      `gorm:"index"`
	ProblemID  uint      `gorm:"index"`
	Language   string    `gorm:"type:varchar(32);index"`
	Mode       string    `gorm:"type:varchar(16);index"`
	DatasetID  uint      `gorm:"index"`
	Status     string    `gorm:"type:varchar(16);index"`
	CodeText   string    `gorm:"type:longtext"`
	CodeHash   string    `gorm:"type:varchar(128);index"`
	LimitsJSON string    `gorm:"type:longtext"`
	QueuedAt   time.Time `gorm:"index"`
	StartedAt  *time.Time
	FinishedAt *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (SubmissionModel) TableName() string {
	return "submissions"
}
