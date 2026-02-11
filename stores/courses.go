package stores

import "time"

type CourseModel struct {
	ID              uint   `gorm:"primaryKey"`
	Slug            string `gorm:"type:varchar(191);uniqueIndex"`
	Title           string `gorm:"type:varchar(255);not null"`
	Excerpt         string `gorm:"type:varchar(500)"`
	Description     string `gorm:"type:varchar(500)"`
	ThumbnailURL    string `gorm:"type:varchar(500)"`
	MetadataJSON    string `gorm:"type:json"`
	BodyMarkdown    string `gorm:"type:longtext"`
	BodyHTMLCache   string `gorm:"type:longtext"`
	AccessLevel     string `gorm:"type:varchar(16);index"`
	Status          string `gorm:"type:varchar(16);index"`
	PublishedAt     *time.Time
	MetaTitle       string `gorm:"type:varchar(255)"`
	MetaDescription string `gorm:"type:varchar(500)"`
	MetaImageURL    string `gorm:"type:varchar(500)"`
	CanonicalURL    string `gorm:"type:varchar(500)"`
	NoIndex         bool   `gorm:"not null;default:false"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (CourseModel) TableName() string {
	return "courses"
}

type CourseModuleModel struct {
	ID        uint   `gorm:"primaryKey"`
	CourseID  uint   `gorm:"index"`
	Title     string `gorm:"type:varchar(255);not null"`
	Position  int    `gorm:"not null;default:0"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (CourseModuleModel) TableName() string {
	return "course_modules"
}

type CourseLessonModel struct {
	ID            uint   `gorm:"primaryKey"`
	CourseID      uint   `gorm:"index"`
	ModuleID      uint   `gorm:"index"`
	Title         string `gorm:"type:varchar(255);not null"`
	Slug          string `gorm:"type:varchar(191);uniqueIndex"`
	BodyMarkdown  string `gorm:"type:longtext"`
	BodyHTMLCache string `gorm:"type:longtext"`
	VimeoURL      string `gorm:"type:varchar(500)"`
	Position      int    `gorm:"not null;default:0"`
	IsFree        bool   `gorm:"not null;default:false"`
	AccessLevel   string `gorm:"type:varchar(16);index"`
	Status        string `gorm:"type:varchar(16);index"`
	PublishedAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (CourseLessonModel) TableName() string {
	return "course_lessons"
}
