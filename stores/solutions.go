package stores

import "time"

type SolutionModel struct {
	ID              uint   `gorm:"primaryKey"`
	ProblemID       uint   `gorm:"index"`
	Language        string `gorm:"type:varchar(32);index"`
	CodeText        string `gorm:"type:longtext"`
	ComplexityJSON  string `gorm:"type:longtext"`
	ApproachSummary string `gorm:"type:varchar(1000)"`
	IsReference     bool   `gorm:"not null;default:false"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (SolutionModel) TableName() string {
	return "solutions"
}
