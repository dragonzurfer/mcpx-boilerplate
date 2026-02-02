package stores

import (
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	cacheTTLMinutesEnvKey  = "CACHE_TTL_MINUTES"
	cacheMaxEntriesEnvKey  = "CACHE_MAX_ENTRIES"
	defaultCacheMaxEntries = 500
)

func CacheConfigFromEnv() CacheConfig {
	ttlMinutes := readEnvInt(cacheTTLMinutesEnvKey)
	if ttlMinutes <= 0 {
		return CacheConfig{}
	}

	maxEntries := readEnvInt(cacheMaxEntriesEnvKey)
	if maxEntries <= 0 {
		maxEntries = defaultCacheMaxEntries
	}

	return CacheConfig{
		TTL:        time.Duration(ttlMinutes) * time.Minute,
		MaxEntries: maxEntries,
	}
}

func readEnvInt(key string) int {
	value, ok := os.LookupEnv(key)
	if !ok {
		return 0
	}

	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}

	parsed, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0
	}

	return parsed
}
