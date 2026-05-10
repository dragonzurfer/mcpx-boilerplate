package stores

import "time"

type DesktopLoginSessionModel struct {
	ID             uint   `gorm:"primaryKey"`
	DeviceCode     string `gorm:"type:varchar(128);uniqueIndex"`
	UserCode       string `gorm:"type:varchar(32);uniqueIndex"`
	Status         string `gorm:"type:varchar(16);index"`
	UserID         uint   `gorm:"index"`
	DeviceID       string `gorm:"type:varchar(128);index"`
	DeviceName     string `gorm:"type:varchar(191)"`
	DevicePlatform string `gorm:"type:varchar(64)"`
	AppVersion     string `gorm:"type:varchar(64)"`
	ApprovedAt     *time.Time
	ExpiresAt      time.Time `gorm:"index"`
	LastPolledAt   *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (DesktopLoginSessionModel) TableName() string {
	return "desktop_login_sessions"
}

type DesktopDeviceModel struct {
	ID         uint      `gorm:"primaryKey"`
	UserID     uint      `gorm:"index"`
	DeviceID   string    `gorm:"type:varchar(128);uniqueIndex"`
	DeviceName string    `gorm:"type:varchar(191)"`
	Platform   string    `gorm:"type:varchar(64);index"`
	AppVersion string    `gorm:"type:varchar(64)"`
	LastSeenAt time.Time `gorm:"index"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (DesktopDeviceModel) TableName() string {
	return "desktop_devices"
}

type DesktopExportModel struct {
	ID              uint   `gorm:"primaryKey"`
	UserID          uint   `gorm:"index"`
	DeviceID        string `gorm:"type:varchar(128);index"`
	ClientExportID  string `gorm:"type:varchar(128);uniqueIndex"`
	Status          string `gorm:"type:varchar(16);index"`
	VideoDurationMs int    `gorm:"not null;default:0"`
	VideoWidth      int    `gorm:"not null;default:0"`
	VideoHeight     int    `gorm:"not null;default:0"`
	AppVersion      string `gorm:"type:varchar(64)"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (DesktopExportModel) TableName() string {
	return "desktop_exports"
}
