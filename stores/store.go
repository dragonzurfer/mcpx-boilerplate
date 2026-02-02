package stores

import (
	"strings"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Store struct {
	db    *gorm.DB
	cache *Cache
}

func NewStore(dsn string) (*Store, error) {
	prepared := normalizeDSN(dsn)

	db, err := gorm.Open(mysql.Open(prepared), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(
		&UserModel{},
		&OAuthIdentityModel{},
		&PostModel{},
		&TagModel{},
		&PostTagModel{},
		&CourseModel{},
		&CourseModuleModel{},
		&CourseLessonModel{},
		&EventModel{},
		&UserMetricsModel{},
		&FunnelConfigModel{},
		&FunnelEventWeightModel{},
		&FunnelStageThresholdModel{},
		&PromoModel{},
		&PromoVariantModel{},
		&PromoDecisionModel{},
		&PromoImpressionModel{},
		&PromoClickModel{},
		&ToolModel{},
		&ToolUsageModel{},
		&ToolEventModel{},
		&ToolDailyMetricModel{},
		&PostImpressionModel{},
		&PostDailyMetricModel{},
		&PostPromoDailyMetricModel{},
		&PaymentModel{},
		&EntitlementModel{},
		&AdminAuditLogModel{},
		&SiteSettingsModel{},
	); err != nil {
		return nil, err
	}

	store := &Store{db: db}
	store.EnableCache(CacheConfigFromEnv())
	return store, nil
}

func (s *Store) DB() *gorm.DB {
	return s.db
}

// NewStoreWithDB builds a Store from an existing gorm DB handle (used in tests).
func NewStoreWithDB(db *gorm.DB) *Store {
	store := &Store{db: db}
	store.EnableCache(CacheConfigFromEnv())
	return store
}

func normalizeDSN(dsn string) string {
	trimmed := strings.TrimSpace(dsn)
	if strings.Contains(trimmed, "tls=true") {
		return strings.ReplaceAll(trimmed, "tls=true", "tls=skip-verify")
	}
	if !strings.Contains(trimmed, "tls=") {
		if strings.Contains(trimmed, "?") {
			trimmed += "&tls=skip-verify"
		} else {
			trimmed += "?tls=skip-verify"
		}
	}
	if !strings.Contains(trimmed, "parseTime=") {
		if strings.Contains(trimmed, "?") {
			trimmed += "&parseTime=true"
		} else {
			trimmed += "?parseTime=true"
		}
	}
	return trimmed
}
