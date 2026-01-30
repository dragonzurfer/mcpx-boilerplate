package stores

import "time"

type UserModel struct {
	ID        uint      `gorm:"primaryKey"`
	Email     string    `gorm:"type:varchar(191);uniqueIndex"`
	Name      string    `gorm:"type:varchar(191)"`
	AvatarURL string    `gorm:"type:varchar(500)"`
	Role      string    `gorm:"type:varchar(32);index"`
	Status    string    `gorm:"type:varchar(32);index"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (UserModel) TableName() string {
	return "users"
}

type OAuthIdentityModel struct {
	ID             uint      `gorm:"primaryKey"`
	UserID         uint      `gorm:"index"`
	Provider       string    `gorm:"type:varchar(32);index"`
	ProviderUserID string    `gorm:"type:varchar(191);uniqueIndex:idx_oauth_provider_user"`
	CreatedAt      time.Time
}

func (OAuthIdentityModel) TableName() string {
	return "oauth_identities"
}
