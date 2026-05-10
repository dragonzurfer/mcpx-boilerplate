package stores

import "time"

type ProblemListModel struct {
	ID          uint   `gorm:"primaryKey"`
	Name        string `gorm:"type:varchar(191);not null"`
	Slug        string `gorm:"type:varchar(191);uniqueIndex"`
	Description string `gorm:"type:text"`
	IsDefault   bool   `gorm:"index"`
	CreatedBy   uint   `gorm:"index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (ProblemListModel) TableName() string {
	return "problem_lists"
}

type ProblemListProblemModel struct {
	ID            uint `gorm:"primaryKey"`
	ProblemListID uint `gorm:"not null;index:idx_problem_list_position,priority:1;uniqueIndex:idx_problem_list_problem,priority:1"`
	ProblemID     uint `gorm:"not null;index:idx_problem_list_position,priority:2;uniqueIndex:idx_problem_list_problem,priority:2"`
	Position      int  `gorm:"not null;default:0;index:idx_problem_list_position,priority:3"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (ProblemListProblemModel) TableName() string {
	return "problem_list_problems"
}
