package stores

import "time"

type ProblemModel struct {
	ID              uint   `gorm:"primaryKey"`
	Slug            string `gorm:"type:varchar(191);uniqueIndex"`
	Title           string `gorm:"type:varchar(255);not null"`
	Difficulty      string `gorm:"type:varchar(16);index"`
	Status          string `gorm:"type:varchar(16);index"`
	StatementJSON   string `gorm:"type:longtext"`
	IOSpecJSON      string `gorm:"type:longtext"`
	ConstraintsJSON string `gorm:"type:longtext"`
	TagsJSON        string `gorm:"type:json"`
	EditorialJSON   string `gorm:"type:longtext"`
	SolutionsJSON   string `gorm:"type:longtext"`
	CreatedBy       uint   `gorm:"index"`
	PublishedAt     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (ProblemModel) TableName() string {
	return "problems"
}

type ProblemStatement struct {
	Markdown string           `json:"markdown,omitempty"`
	Examples []ProblemExample `json:"examples,omitempty"`
	Notes    []string         `json:"notes,omitempty"`
}

type ProblemExample struct {
	Input       string `json:"input,omitempty"`
	Output      string `json:"output,omitempty"`
	Explanation string `json:"explanation,omitempty"`
}

type ProblemEditorial struct {
	Markdown string   `json:"markdown,omitempty"`
	Hints    []string `json:"hints,omitempty"`
}
