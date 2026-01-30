package stores

import "time"

type PostModel struct {
	ID            uint      `gorm:"primaryKey"`
	Slug          string    `gorm:"type:varchar(191);uniqueIndex"`
	Title         string    `gorm:"type:varchar(255);not null"`
	Excerpt       string    `gorm:"type:varchar(500)"`
	BodyMarkdown  string    `gorm:"type:longtext"`
	BodyHTMLCache string    `gorm:"type:longtext"`
	AccessLevel   string    `gorm:"type:varchar(16);index"`
	Status        string    `gorm:"type:varchar(16);index"`
	PublishedAt   *time.Time
	CreatedBy     uint      `gorm:"index"`
	MetaTitle     string    `gorm:"type:varchar(255)"`
	MetaDescription string  `gorm:"type:varchar(500)"`
	MetaImageURL  string    `gorm:"type:varchar(500)"`
	CanonicalURL  string    `gorm:"type:varchar(500)"`
	NoIndex       bool      `gorm:"not null;default:false"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (PostModel) TableName() string {
	return "posts"
}

type TagModel struct {
	ID        uint      `gorm:"primaryKey"`
	Name      string    `gorm:"type:varchar(191);uniqueIndex"`
	TagType   string    `gorm:"type:varchar(32);index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (TagModel) TableName() string {
	return "tags"
}

type PostTagModel struct {
	PostID uint `gorm:"primaryKey"`
	TagID  uint `gorm:"primaryKey"`
}

func (PostTagModel) TableName() string {
	return "post_tags"
}
