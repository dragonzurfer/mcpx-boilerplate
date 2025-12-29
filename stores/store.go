package stores

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Store struct {
	db *gorm.DB
}

type UserModel struct {
	ID           uint   `gorm:"primaryKey"`
	IdentityUID  string `gorm:"type:varchar(191);uniqueIndex"`
	ProviderID   string `gorm:"type:varchar(100)"`
	Email        string `gorm:"type:varchar(191)"`
	DisplayName  string `gorm:"type:varchar(191)"`
	CustomClaims []byte `gorm:"type:json"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (UserModel) TableName() string {
	return "users"
}

type SubscriptionModel struct {
	ID                 uint      `gorm:"primaryKey"`
	UserID             uint      `gorm:"not null;index"`
	PlanCode           string    `gorm:"type:varchar(32);not null;index"`
	Provider           string    `gorm:"type:varchar(32);not null;index"`
	SubscriptionRef    string    `gorm:"type:varchar(128);not null;uniqueIndex"`
	Status             string    `gorm:"type:varchar(32);not null;index"`
	CurrentPeriodStart time.Time `gorm:"index"`
	CurrentPeriodEnd   time.Time `gorm:"index"`
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

func (SubscriptionModel) TableName() string {
	return "subscriptions"
}

type UsageCounterModel struct {
	ID          uint      `gorm:"primaryKey"`
	ProjectKey  string    `gorm:"type:varchar(64);not null;uniqueIndex:idx_usage_subject_period_metric"`
	SubjectType string    `gorm:"type:varchar(32);not null;uniqueIndex:idx_usage_subject_period_metric"`
	SubjectID   string    `gorm:"type:varchar(191);not null;uniqueIndex:idx_usage_subject_period_metric"`
	Metric      string    `gorm:"type:varchar(64);not null;uniqueIndex:idx_usage_subject_period_metric"`
	PeriodStart time.Time `gorm:"not null;uniqueIndex:idx_usage_subject_period_metric"`
	Count       int64     `gorm:"not null;default:0"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (UsageCounterModel) TableName() string {
	return "usage_counters"
}

type IPRuleModel struct {
	ID          uint   `gorm:"primaryKey"`
	ProjectKey  string `gorm:"type:varchar(64);not null;uniqueIndex:idx_iprule_project_cidr"`
	RuleType    string `gorm:"type:varchar(16);not null;index"` // allow or deny
	CIDR        string `gorm:"type:varchar(64);not null;uniqueIndex:idx_iprule_project_cidr"`
	Enabled     bool   `gorm:"not null;default:true"`
	Description string `gorm:"type:varchar(255)"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (IPRuleModel) TableName() string {
	return "ip_rules"
}

func NewStore(dsn string) (*Store, error) {
	if strings.Contains(dsn, "tls=true") {
		dsn = strings.ReplaceAll(dsn, "tls=true", "tls=skip-verify")
	} else if !strings.Contains(dsn, "tls=") {
		if strings.Contains(dsn, "?") {
			dsn += "&tls=skip-verify"
		} else {
			dsn += "?tls=skip-verify"
		}
	}
	if !strings.Contains(dsn, "parseTime=") {
		if strings.Contains(dsn, "?") {
			dsn += "&parseTime=true"
		} else {
			dsn += "?parseTime=true"
		}
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(
		&UserModel{},
		&SubscriptionModel{},
		&GooglePlayPurchaseModel{},
		&UsageCounterModel{},
		&IPRuleModel{},
	); err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) GetUserByIdentity(identityUID string) (*UserModel, error) {
	identityUID = strings.TrimSpace(identityUID)
	if identityUID == "" {
		return nil, fmt.Errorf("identity uid required")
	}
	var user UserModel
	if err := s.db.Where("identity_uid = ?", identityUID).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Store) GetOrCreateUser(identityUID, displayName, providerID, email string, claims map[string]interface{}) (*UserModel, error) {
	identityUID = strings.TrimSpace(identityUID)
	if identityUID == "" {
		return nil, fmt.Errorf("identity uid required")
	}

	var user UserModel
	err := s.db.Where("identity_uid = ?", identityUID).First(&user).Error
	if err == nil {
		updates := map[string]interface{}{}
		if displayName != "" && displayName != user.DisplayName {
			updates["display_name"] = displayName
		}
		if providerID != "" && providerID != user.ProviderID {
			updates["provider_id"] = providerID
		}
		if email != "" && email != user.Email {
			updates["email"] = email
		}
		if len(claims) > 0 {
			if raw, err := json.Marshal(claims); err == nil {
				updates["custom_claims"] = raw
			}
		}
		if len(updates) > 0 {
			_ = s.db.Model(&user).Updates(updates).Error
		}
		return &user, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var claimsRaw []byte
	if len(claims) > 0 {
		claimsRaw, _ = json.Marshal(claims)
	}

	user = UserModel{
		IdentityUID:  identityUID,
		ProviderID:   strings.TrimSpace(providerID),
		Email:        strings.TrimSpace(email),
		DisplayName:  strings.TrimSpace(displayName),
		CustomClaims: claimsRaw,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := s.db.Create(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *Store) UpsertSubscription(userID uint, planCode, provider, subscriptionRef, status string, periodStart, periodEnd time.Time) (*SubscriptionModel, error) {
	planCode = strings.TrimSpace(planCode)
	provider = strings.TrimSpace(provider)
	subscriptionRef = strings.TrimSpace(subscriptionRef)
	status = strings.TrimSpace(status)
	if userID == 0 || planCode == "" || provider == "" || subscriptionRef == "" {
		return nil, fmt.Errorf("user, plan, provider, subscription ref required")
	}
	if status == "" {
		status = "created"
	}

	var existing SubscriptionModel
	err := s.db.Where("subscription_ref = ?", subscriptionRef).First(&existing).Error
	if err == nil {
		updates := map[string]interface{}{
			"user_id":              userID,
			"plan_code":            planCode,
			"provider":             provider,
			"status":               status,
			"current_period_start": periodStart,
			"current_period_end":   periodEnd,
		}
		if err := s.db.Model(&existing).Updates(updates).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	model := SubscriptionModel{
		UserID:             userID,
		PlanCode:           planCode,
		Provider:           provider,
		SubscriptionRef:    subscriptionRef,
		Status:             status,
		CurrentPeriodStart: periodStart,
		CurrentPeriodEnd:   periodEnd,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}
	if err := s.db.Create(&model).Error; err != nil {
		return nil, err
	}
	return &model, nil
}

func (s *Store) GetActiveSubscription(userID uint, now time.Time) (*SubscriptionModel, error) {
	if userID == 0 {
		return nil, fmt.Errorf("user required")
	}
	var sub SubscriptionModel
	q := s.db.Where("user_id = ? AND status = ?", userID, "active")
	q = q.Order("current_period_end desc")
	if err := q.First(&sub).Error; err != nil {
		return nil, err
	}
	if !sub.CurrentPeriodEnd.IsZero() && sub.CurrentPeriodEnd.Before(now.UTC().Add(-1*time.Minute)) {
		return nil, gorm.ErrRecordNotFound
	}
	return &sub, nil
}

func (s *Store) GetLatestSubscription(userID uint) (*SubscriptionModel, error) {
	if userID == 0 {
		return nil, fmt.Errorf("user required")
	}
	var sub SubscriptionModel
	if err := s.db.Where("user_id = ?", userID).Order("updated_at desc").First(&sub).Error; err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *Store) GetSubscriptionByRef(userID uint, provider, subscriptionRef string) (*SubscriptionModel, error) {
	provider = strings.TrimSpace(provider)
	subscriptionRef = strings.TrimSpace(subscriptionRef)
	if userID == 0 || provider == "" || subscriptionRef == "" {
		return nil, fmt.Errorf("user, provider, subscription ref required")
	}
	var sub SubscriptionModel
	if err := s.db.Where("user_id = ? AND provider = ? AND subscription_ref = ?", userID, provider, subscriptionRef).First(&sub).Error; err != nil {
		return nil, err
	}
	return &sub, nil
}

func periodStartUTC(now time.Time) time.Time {
	t := now.UTC()
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func (s *Store) GetUsageCount(projectKey, subjectType, subjectID, metric string, now time.Time) (int64, error) {
	projectKey = strings.TrimSpace(projectKey)
	subjectType = strings.TrimSpace(subjectType)
	subjectID = strings.TrimSpace(subjectID)
	metric = strings.TrimSpace(metric)
	if projectKey == "" || subjectType == "" || subjectID == "" || metric == "" {
		return 0, fmt.Errorf("project, subject, and metric required")
	}
	start := periodStartUTC(now)
	var m UsageCounterModel
	err := s.db.Where(
		"project_key = ? AND subject_type = ? AND subject_id = ? AND metric = ? AND period_start = ?",
		projectKey,
		subjectType,
		subjectID,
		metric,
		start,
	).First(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return m.Count, nil
}

func (s *Store) IncrementUsage(projectKey, subjectType, subjectID, metric string, delta int64, now time.Time) (*UsageCounterModel, error) {
	projectKey = strings.TrimSpace(projectKey)
	subjectType = strings.TrimSpace(subjectType)
	subjectID = strings.TrimSpace(subjectID)
	metric = strings.TrimSpace(metric)
	if projectKey == "" || subjectType == "" || subjectID == "" || metric == "" {
		return nil, fmt.Errorf("project, subject, and metric required")
	}
	if delta <= 0 {
		return nil, fmt.Errorf("delta must be positive")
	}
	start := periodStartUTC(now)
	row := UsageCounterModel{
		ProjectKey:  projectKey,
		SubjectType: subjectType,
		SubjectID:   subjectID,
		Metric:      metric,
		PeriodStart: start,
	}
	if err := s.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "project_key"}, {Name: "subject_type"}, {Name: "subject_id"}, {Name: "metric"}, {Name: "period_start"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"count":      gorm.Expr("count + ?", delta),
			"updated_at": time.Now(),
		}),
	}).Create(&row).Error; err != nil {
		return nil, err
	}
	var out UsageCounterModel
	if err := s.db.Where(
		"project_key = ? AND subject_type = ? AND subject_id = ? AND metric = ? AND period_start = ?",
		projectKey,
		subjectType,
		subjectID,
		metric,
		start,
	).First(&out).Error; err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Store) ListUsageForPeriod(projectKey, subjectType, subjectID string, now time.Time) ([]UsageCounterModel, error) {
	projectKey = strings.TrimSpace(projectKey)
	subjectType = strings.TrimSpace(subjectType)
	subjectID = strings.TrimSpace(subjectID)
	if projectKey == "" || subjectType == "" || subjectID == "" {
		return nil, fmt.Errorf("project and subject required")
	}
	start := periodStartUTC(now)
	var rows []UsageCounterModel
	if err := s.db.Where(
		"project_key = ? AND subject_type = ? AND subject_id = ? AND period_start = ?",
		projectKey,
		subjectType,
		subjectID,
		start,
	).Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Store) ListIPRules(projectKey string) ([]IPRuleModel, error) {
	projectKey = strings.TrimSpace(projectKey)
	if projectKey == "" {
		return nil, fmt.Errorf("project required")
	}
	var rows []IPRuleModel
	if err := s.db.Where("project_key = ?", projectKey).Order("id desc").Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Store) UpsertIPRule(projectKey, ruleType, cidr, description string, enabled bool) (*IPRuleModel, error) {
	projectKey = strings.TrimSpace(projectKey)
	ruleType = strings.ToLower(strings.TrimSpace(ruleType))
	cidr = strings.TrimSpace(cidr)
	description = strings.TrimSpace(description)
	if projectKey == "" || ruleType == "" || cidr == "" {
		return nil, fmt.Errorf("project, rule type, and cidr required")
	}

	var existing IPRuleModel
	err := s.db.Where("project_key = ? AND cidr = ?", projectKey, cidr).First(&existing).Error
	if err == nil {
		updates := map[string]interface{}{
			"rule_type":   ruleType,
			"enabled":     enabled,
			"description": description,
		}
		if err := s.db.Model(&existing).Updates(updates).Error; err != nil {
			return nil, err
		}
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	row := IPRuleModel{
		ProjectKey:  projectKey,
		RuleType:    ruleType,
		CIDR:        cidr,
		Enabled:     enabled,
		Description: description,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	if err := s.db.Create(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

func (s *Store) DeleteIPRule(projectKey string, id uint) error {
	projectKey = strings.TrimSpace(projectKey)
	if projectKey == "" || id == 0 {
		return fmt.Errorf("project and id required")
	}
	return s.db.Where("project_key = ? AND id = ?", projectKey, id).Delete(&IPRuleModel{}).Error
}
