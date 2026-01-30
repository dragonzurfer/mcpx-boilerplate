package stores

import "time"

type PaymentModel struct {
	ID               uint      `gorm:"primaryKey"`
	UserID           uint      `gorm:"index"`
	PlanCode         string    `gorm:"type:varchar(32);index"`
	RazorpayOrderID  string    `gorm:"type:varchar(128);index"`
	RazorpayPaymentID string   `gorm:"type:varchar(128);index"`
	Status           string    `gorm:"type:varchar(32);index"`
	AmountINR        int       `gorm:"not null;default:0"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (PaymentModel) TableName() string {
	return "payments"
}

type EntitlementModel struct {
	ID            uint      `gorm:"primaryKey"`
	UserID        uint      `gorm:"index"`
	PlanCode      string    `gorm:"type:varchar(32);index"`
	Status        string    `gorm:"type:varchar(16);index"`
	StartAt       time.Time
	EndAt         time.Time
	LastPaymentID uint
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

func (EntitlementModel) TableName() string {
	return "entitlements"
}
