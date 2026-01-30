package stores

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type PaymentCreateInput struct {
	UserID          uint
	PlanCode        string
	RazorpayOrderID string
	AmountINR       int
}

func (s *Store) CreatePayment(input PaymentCreateInput) (*PaymentModel, error) {
	planCode := strings.TrimSpace(input.PlanCode)
	orderID := strings.TrimSpace(input.RazorpayOrderID)
	if input.UserID == 0 || planCode == "" || orderID == "" {
		return nil, gorm.ErrInvalidData
	}

	payment := PaymentModel{
		UserID:          input.UserID,
		PlanCode:        planCode,
		RazorpayOrderID: orderID,
		Status:          PaymentStatusCreated,
		AmountINR:       input.AmountINR,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	if err := s.db.Create(&payment).Error; err != nil {
		return nil, err
	}
	return &payment, nil
}

type PaymentUpdateInput struct {
	RazorpayOrderID  string
	RazorpayPaymentID string
	Status           string
}

func (s *Store) UpdatePaymentStatus(input PaymentUpdateInput) (*PaymentModel, error) {
	orderID := strings.TrimSpace(input.RazorpayOrderID)
	if orderID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	payment := PaymentModel{}
	if err := s.db.Where("razorpay_order_id = ?", orderID).First(&payment).Error; err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	status := strings.TrimSpace(input.Status)
	if status != "" {
		updates["status"] = status
	}
	if input.RazorpayPaymentID != "" {
		updates["razorpay_payment_id"] = input.RazorpayPaymentID
	}
	updates["updated_at"] = time.Now().UTC()

	if err := s.db.Model(&payment).Updates(updates).Error; err != nil {
		return nil, err
	}
	return &payment, nil
}

func (s *Store) GetPaymentByOrderID(orderID string) (*PaymentModel, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return nil, gorm.ErrRecordNotFound
	}

	payment := PaymentModel{}
	if err := s.db.Where("razorpay_order_id = ?", orderID).First(&payment).Error; err != nil {
		return nil, err
	}
	return &payment, nil
}

type EntitlementLookupInput struct {
	UserID uint
	Now    time.Time
}

func (s *Store) GetActiveEntitlement(input EntitlementLookupInput) (*EntitlementModel, error) {
	if input.UserID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	entitlement := EntitlementModel{}
	if err := s.db.Where("user_id = ? AND status = ? AND end_at > ?", input.UserID, EntitlementStatusActive, input.Now).First(&entitlement).Error; err != nil {
		return nil, err
	}
	return &entitlement, nil
}

func (s *Store) GetLatestEntitlement(userID uint) (*EntitlementModel, error) {
	if userID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	entitlement := EntitlementModel{}
	if err := s.db.Where("user_id = ?", userID).Order("end_at desc").First(&entitlement).Error; err != nil {
		return nil, err
	}
	return &entitlement, nil
}

type EntitlementUpsertInput struct {
	UserID       uint
	PlanCode     string
	StartAt      time.Time
	EndAt        time.Time
	LastPaymentID uint
}

func (s *Store) UpsertEntitlement(input EntitlementUpsertInput) (*EntitlementModel, error) {
	planCode := strings.TrimSpace(input.PlanCode)
	if input.UserID == 0 || planCode == "" {
		return nil, gorm.ErrInvalidData
	}

	existing := EntitlementModel{}
	err := s.db.Where("user_id = ?", input.UserID).Order("id desc").First(&existing).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	if err == nil {
		updates := map[string]interface{}{
			"plan_code":      planCode,
			"status":         EntitlementStatusActive,
			"start_at":       input.StartAt,
			"end_at":         input.EndAt,
			"last_payment_id": input.LastPaymentID,
			"updated_at":     time.Now().UTC(),
		}
		if err := s.db.Model(&existing).Updates(updates).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}

	entitlement := EntitlementModel{
		UserID:        input.UserID,
		PlanCode:      planCode,
		Status:        EntitlementStatusActive,
		StartAt:       input.StartAt,
		EndAt:         input.EndAt,
		LastPaymentID: input.LastPaymentID,
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
	}

	if err := s.db.Create(&entitlement).Error; err != nil {
		return nil, err
	}
	return &entitlement, nil
}

func (s *Store) ExpireEntitlements(now time.Time) error {
	updates := map[string]interface{}{
		"status":     EntitlementStatusExpired,
		"updated_at": time.Now().UTC(),
	}
	return s.db.Model(&EntitlementModel{}).Where("status = ? AND end_at < ?", EntitlementStatusActive, now).Updates(updates).Error
}
