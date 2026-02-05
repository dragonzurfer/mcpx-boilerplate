package stores

import (
	"strings"
	"time"

	driver "github.com/go-sql-driver/mysql"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Store struct {
	db    *gorm.DB
	cache *Cache
}

func NewStore(dsn string) (*Store, error) {
	prepared := normalizeDSN(dsn)

	db, err := gorm.Open(gormmysql.Open(prepared), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	if err := configureConnectionPool(db); err != nil {
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
	if trimmed == "" {
		return ""
	}

	parsedDSN, err := driver.ParseDSN(trimmed)
	if err != nil {
		return normalizeDSNFallback(trimmed)
	}

	parsedDSN.ParseTime = true
	parsedDSN.InterpolateParams = true
	parsedDSN.TLSConfig = normalizeTLSPolicy(parsedDSN.TLSConfig)
	if parsedDSN.Loc == nil {
		parsedDSN.Loc = time.UTC
	}

	return parsedDSN.FormatDSN()
}

func configureConnectionPool(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}

	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(25)
	sqlDB.SetConnMaxIdleTime(2 * time.Minute)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	return nil
}

func normalizeTLSPolicy(rawTLS string) string {
	tlsPolicy := strings.TrimSpace(rawTLS)
	if tlsPolicy == "" || tlsPolicy == "true" {
		return "skip-verify"
	}
	return tlsPolicy
}

func normalizeDSNFallback(dsn string) string {
	normalized := dsn
	if strings.Contains(normalized, "tls=true") {
		normalized = strings.ReplaceAll(normalized, "tls=true", "tls=skip-verify")
	}
	if !strings.Contains(normalized, "tls=") {
		normalized = appendDSNParam(normalized, "tls=skip-verify")
	}
	if !strings.Contains(normalized, "parseTime=") {
		normalized = appendDSNParam(normalized, "parseTime=true")
	}
	if !strings.Contains(normalized, "interpolateParams=") {
		normalized = appendDSNParam(normalized, "interpolateParams=true")
	}
	if !strings.Contains(normalized, "loc=") {
		normalized = appendDSNParam(normalized, "loc=UTC")
	}
	return normalized
}

func appendDSNParam(dsn string, queryParam string) string {
	if strings.Contains(dsn, "?") {
		return dsn + "&" + queryParam
	}
	return dsn + "?" + queryParam
}
