package stores

import "time"

type DatasetModel struct {
	ID                   uint   `gorm:"primaryKey"`
	ProblemID            uint   `gorm:"index"`
	Type                 string `gorm:"type:varchar(16);index"`
	ScoringMode          string `gorm:"type:varchar(16);index"`
	ExecutionPolicyJSON  string `gorm:"type:longtext"`
	ValidatorDefaultJSON string `gorm:"type:longtext"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (DatasetModel) TableName() string {
	return "datasets"
}

type TestcaseModel struct {
	ID                    uint   `gorm:"primaryKey"`
	DatasetID             uint   `gorm:"index"`
	InputText             string `gorm:"type:longtext"`
	ExpectedText          string `gorm:"type:longtext"`
	Visibility            string `gorm:"type:varchar(16);index"`
	Weight                int    `gorm:"not null;default:1"`
	Group                 string `gorm:"type:varchar(16);index"`
	Position              int    `gorm:"not null;default:0"`
	ValidatorOverrideJSON string `gorm:"type:longtext"`
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func (TestcaseModel) TableName() string {
	return "testcases"
}
