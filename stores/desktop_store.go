package stores

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type DesktopLoginSessionCreateInput struct {
	DeviceCode     string
	UserCode       string
	DeviceID       string
	DeviceName     string
	DevicePlatform string
	AppVersion     string
	ExpiresAt      time.Time
}

func (s *Store) CreateDesktopLoginSession(input DesktopLoginSessionCreateInput) (*DesktopLoginSessionModel, error) {
	deviceCode := strings.TrimSpace(input.DeviceCode)
	userCode := strings.ToUpper(strings.TrimSpace(input.UserCode))
	if deviceCode == "" || userCode == "" {
		return nil, gorm.ErrInvalidData
	}

	now := time.Now().UTC()
	session := DesktopLoginSessionModel{
		DeviceCode:     deviceCode,
		UserCode:       userCode,
		Status:         DesktopLoginSessionStatusPending,
		DeviceID:       strings.TrimSpace(input.DeviceID),
		DeviceName:     strings.TrimSpace(input.DeviceName),
		DevicePlatform: strings.TrimSpace(input.DevicePlatform),
		AppVersion:     strings.TrimSpace(input.AppVersion),
		ExpiresAt:      input.ExpiresAt.UTC(),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.db.Create(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func (s *Store) GetDesktopLoginSessionByDeviceCode(deviceCode string) (*DesktopLoginSessionModel, error) {
	deviceCode = strings.TrimSpace(deviceCode)
	if deviceCode == "" {
		return nil, gorm.ErrRecordNotFound
	}

	session := DesktopLoginSessionModel{}
	if err := s.db.Where("device_code = ?", deviceCode).First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

type DesktopLoginSessionApproveInput struct {
	UserCode string
	UserID   uint
	Now      time.Time
}

func (s *Store) ApproveDesktopLoginSession(input DesktopLoginSessionApproveInput) (*DesktopLoginSessionModel, error) {
	userCode := strings.ToUpper(strings.TrimSpace(input.UserCode))
	if userCode == "" || input.UserID == 0 {
		return nil, gorm.ErrInvalidData
	}
	now := input.Now.UTC()

	session := DesktopLoginSessionModel{}
	if err := s.db.Where("user_code = ?", userCode).First(&session).Error; err != nil {
		return nil, err
	}

	if session.ExpiresAt.Before(now) {
		_ = s.db.Model(&session).Updates(map[string]interface{}{
			"status":         DesktopLoginSessionStatusExpired,
			"updated_at":     now,
			"last_polled_at": now,
		}).Error
		return nil, gorm.ErrRecordNotFound
	}

	if session.Status == DesktopLoginSessionStatusApproved && session.UserID == input.UserID {
		return &session, nil
	}

	updates := map[string]interface{}{
		"status":      DesktopLoginSessionStatusApproved,
		"user_id":     input.UserID,
		"approved_at": now,
		"updated_at":  now,
	}
	if err := s.db.Model(&session).Updates(updates).Error; err != nil {
		return nil, err
	}
	session.Status = DesktopLoginSessionStatusApproved
	session.UserID = input.UserID
	session.UpdatedAt = now
	session.ApprovedAt = &now
	return &session, nil
}

func (s *Store) TouchDesktopLoginSessionPoll(deviceCode string, now time.Time) error {
	deviceCode = strings.TrimSpace(deviceCode)
	if deviceCode == "" {
		return nil
	}
	return s.db.Model(&DesktopLoginSessionModel{}).
		Where("device_code = ?", deviceCode).
		Updates(map[string]interface{}{
			"last_polled_at": now.UTC(),
			"updated_at":     now.UTC(),
		}).Error
}

func (s *Store) ExpireDesktopLoginSession(deviceCode string, now time.Time) error {
	deviceCode = strings.TrimSpace(deviceCode)
	if deviceCode == "" {
		return nil
	}
	return s.db.Model(&DesktopLoginSessionModel{}).
		Where("device_code = ? AND status <> ?", deviceCode, DesktopLoginSessionStatusApproved).
		Updates(map[string]interface{}{
			"status":         DesktopLoginSessionStatusExpired,
			"updated_at":     now.UTC(),
			"last_polled_at": now.UTC(),
		}).Error
}

type DesktopDeviceUpsertInput struct {
	UserID     uint
	DeviceID   string
	DeviceName string
	Platform   string
	AppVersion string
	Now        time.Time
}

func (s *Store) UpsertDesktopDevice(input DesktopDeviceUpsertInput) (*DesktopDeviceModel, error) {
	deviceID := strings.TrimSpace(input.DeviceID)
	if input.UserID == 0 || deviceID == "" {
		return nil, gorm.ErrInvalidData
	}
	now := input.Now.UTC()

	device := DesktopDeviceModel{}
	err := s.db.Where("device_id = ?", deviceID).First(&device).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if err == nil {
		updates := map[string]interface{}{
			"user_id":      input.UserID,
			"last_seen_at": now,
			"updated_at":   now,
		}
		if v := strings.TrimSpace(input.DeviceName); v != "" {
			updates["device_name"] = v
		}
		if v := strings.TrimSpace(input.Platform); v != "" {
			updates["platform"] = v
		}
		if v := strings.TrimSpace(input.AppVersion); v != "" {
			updates["app_version"] = v
		}
		if err := s.db.Model(&device).Updates(updates).Error; err != nil {
			return nil, err
		}
		return &device, nil
	}

	device = DesktopDeviceModel{
		UserID:     input.UserID,
		DeviceID:   deviceID,
		DeviceName: strings.TrimSpace(input.DeviceName),
		Platform:   strings.TrimSpace(input.Platform),
		AppVersion: strings.TrimSpace(input.AppVersion),
		LastSeenAt: now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if err := s.db.Create(&device).Error; err != nil {
		return nil, err
	}
	return &device, nil
}

func (s *Store) CountCompletedDesktopExports(userID uint) (int64, error) {
	if userID == 0 {
		return 0, gorm.ErrRecordNotFound
	}
	var count int64
	err := s.db.Model(&DesktopExportModel{}).
		Where("user_id = ? AND status = ?", userID, DesktopExportStatusCompleted).
		Count(&count).Error
	return count, err
}

type DesktopExportCompletionInput struct {
	UserID          uint
	DeviceID        string
	ClientExportID  string
	VideoDurationMs int
	VideoWidth      int
	VideoHeight     int
	AppVersion      string
	Now             time.Time
}

func (s *Store) RecordDesktopExportCompletion(input DesktopExportCompletionInput) (*DesktopExportModel, error) {
	clientExportID := strings.TrimSpace(input.ClientExportID)
	if input.UserID == 0 || clientExportID == "" {
		return nil, gorm.ErrInvalidData
	}
	now := input.Now.UTC()

	existing := DesktopExportModel{}
	err := s.db.Where("client_export_id = ?", clientExportID).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	record := DesktopExportModel{
		UserID:          input.UserID,
		DeviceID:        strings.TrimSpace(input.DeviceID),
		ClientExportID:  clientExportID,
		Status:          DesktopExportStatusCompleted,
		VideoDurationMs: maxInt(input.VideoDurationMs, 0),
		VideoWidth:      maxInt(input.VideoWidth, 0),
		VideoHeight:     maxInt(input.VideoHeight, 0),
		AppVersion:      strings.TrimSpace(input.AppVersion),
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := s.db.Create(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

func maxInt(value, min int) int {
	if value < min {
		return min
	}
	return value
}
