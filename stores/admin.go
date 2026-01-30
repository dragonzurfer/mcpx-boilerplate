package stores

import "time"

type AdminAuditLogModel struct {
	ID          uint   `gorm:"primaryKey"`
	AdminUserID uint   `gorm:"index"`
	Action      string `gorm:"type:varchar(64);index"`
	EntityType  string `gorm:"type:varchar(64);index"`
	EntityID    *uint  `gorm:"index"`
	BeforeJSON  string `gorm:"type:longtext"`
	AfterJSON   string `gorm:"type:longtext"`
	CreatedAt   time.Time
}

func (AdminAuditLogModel) TableName() string {
	return "admin_audit_logs"
}

type SiteSettingsModel struct {
	ID                    uint   `gorm:"primaryKey"`
	SiteName              string `gorm:"type:varchar(191)"`
	SiteURL               string `gorm:"type:varchar(255)"`
	DefaultOgImageURL     string `gorm:"type:varchar(500)"`
	PrimaryColor          string `gorm:"type:varchar(32)"`
	TwitterSite           string `gorm:"type:varchar(191)"`
	TwitterCreator        string `gorm:"type:varchar(191)"`
	GoogleVerification    string `gorm:"type:varchar(191)"`
	BingVerification      string `gorm:"type:varchar(191)"`
	PinterestVerification string `gorm:"type:varchar(191)"`
	UpdatedAt             time.Time
}

func (SiteSettingsModel) TableName() string {
	return "site_settings"
}
