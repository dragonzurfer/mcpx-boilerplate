package core

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type GooglePlayPurchaseModel struct {
	ID uint `gorm:"primaryKey"`

	UserID   uint   `gorm:"index"`
	PlanCode string `gorm:"type:varchar(32);index"`

	PackageName string `gorm:"type:varchar(255);index"`
	ProductID   string `gorm:"type:varchar(255);index"`

	PurchaseTokenHash string `gorm:"type:char(64);not null;uniqueIndex"`
	SubscriptionRef   string `gorm:"type:varchar(64);index"`
	OrderID           string `gorm:"type:varchar(128);index"`

	StartTime  time.Time `gorm:"index"`
	ExpiryTime time.Time `gorm:"index"`

	AutoRenewing         bool   `gorm:"not null;default:false"`
	AcknowledgementState int64  `gorm:"not null;default:0"`
	CancelReason         int64  `gorm:"not null;default:0"`
	PaymentState         *int64 `gorm:"index"`

	ObfuscatedExternalAccountID string `gorm:"type:varchar(191)"`

	RawPayload           []byte    `gorm:"type:json"`
	LastNotificationType int64     `gorm:"index"`
	LastEventTime        time.Time `gorm:"index"`
	LastVerifiedAt       time.Time `gorm:"index"`

	CreatedAt time.Time
	UpdatedAt time.Time
}

func (GooglePlayPurchaseModel) TableName() string {
	return "google_play_purchases"
}

func GooglePlayPurchaseTokenHash(purchaseToken string) string {
	purchaseToken = strings.TrimSpace(purchaseToken)
	sum := sha256.Sum256([]byte(purchaseToken))
	return hex.EncodeToString(sum[:])
}

// GooglePlaySubscriptionRef encodes the purchase token hash into a compact reference.
func GooglePlaySubscriptionRef(purchaseTokenHash string) (string, error) {
	purchaseTokenHash = strings.TrimSpace(purchaseTokenHash)
	if len(purchaseTokenHash) < 61 {
		return "", fmt.Errorf("invalid purchase token hash")
	}
	return "gp_" + purchaseTokenHash[:61], nil
}

func (s *Store) GetGooglePlayPurchaseByTokenHash(purchaseTokenHash string) (*GooglePlayPurchaseModel, error) {
	purchaseTokenHash = strings.TrimSpace(purchaseTokenHash)
	if purchaseTokenHash == "" {
		return nil, fmt.Errorf("purchase token hash required")
	}
	var m GooglePlayPurchaseModel
	if err := s.db.Where("purchase_token_hash = ?", purchaseTokenHash).First(&m).Error; err != nil {
		return nil, err
	}
	return &m, nil
}

func (s *Store) UpsertGooglePlayPurchase(in GooglePlayPurchaseModel) (*GooglePlayPurchaseModel, error) {
	in.PackageName = strings.TrimSpace(in.PackageName)
	in.ProductID = strings.TrimSpace(in.ProductID)
	in.PlanCode = strings.TrimSpace(in.PlanCode)
	in.PurchaseTokenHash = strings.TrimSpace(in.PurchaseTokenHash)
	in.SubscriptionRef = strings.TrimSpace(in.SubscriptionRef)
	in.OrderID = strings.TrimSpace(in.OrderID)
	in.ObfuscatedExternalAccountID = strings.TrimSpace(in.ObfuscatedExternalAccountID)
	if in.PurchaseTokenHash == "" {
		return nil, fmt.Errorf("purchase token hash required")
	}

	var existing GooglePlayPurchaseModel
	err := s.db.Where("purchase_token_hash = ?", in.PurchaseTokenHash).First(&existing).Error
	if err == nil {
		updates := map[string]interface{}{
			"auto_renewing":          in.AutoRenewing,
			"acknowledgement_state":  in.AcknowledgementState,
			"cancel_reason":          in.CancelReason,
			"last_notification_type": in.LastNotificationType,
			"last_event_time":        in.LastEventTime,
			"last_verified_at":       in.LastVerifiedAt,
		}
		if in.UserID != 0 {
			updates["user_id"] = in.UserID
		}
		if in.PlanCode != "" {
			updates["plan_code"] = in.PlanCode
		}
		if in.PackageName != "" {
			updates["package_name"] = in.PackageName
		}
		if in.ProductID != "" {
			updates["product_id"] = in.ProductID
		}
		if in.SubscriptionRef != "" {
			updates["subscription_ref"] = in.SubscriptionRef
		}
		if in.OrderID != "" {
			updates["order_id"] = in.OrderID
		}
		if !in.StartTime.IsZero() {
			updates["start_time"] = in.StartTime
		}
		if !in.ExpiryTime.IsZero() {
			updates["expiry_time"] = in.ExpiryTime
		}
		if in.PaymentState != nil {
			updates["payment_state"] = *in.PaymentState
		}
		if in.ObfuscatedExternalAccountID != "" {
			updates["obfuscated_external_account_id"] = in.ObfuscatedExternalAccountID
		}
		if len(in.RawPayload) > 0 {
			updates["raw_payload"] = in.RawPayload
		}

		if err := s.db.Model(&existing).Updates(updates).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if in.LastVerifiedAt.IsZero() {
		in.LastVerifiedAt = time.Now().UTC()
	}
	if err := s.db.Create(&in).Error; err != nil {
		return nil, err
	}
	return &in, nil
}
