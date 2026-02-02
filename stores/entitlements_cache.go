package stores

import "time"

type entitlementActiveCacheKey struct {
	UserID uint      `json:"user_id"`
	Now    time.Time `json:"now"`
}

type entitlementLatestCacheKey struct {
	UserID uint `json:"user_id"`
}

func (s *Store) getCachedActiveEntitlement(key string) (*EntitlementModel, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return nil, false
	}
	typed, ok := value.(*EntitlementModel)
	if !ok {
		return nil, false
	}
	return typed, true
}

func (s *Store) getCachedLatestEntitlement(key string) (*EntitlementModel, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return nil, false
	}
	typed, ok := value.(*EntitlementModel)
	if !ok {
		return nil, false
	}
	return typed, true
}

func activeEntitlementKey(input EntitlementLookupInput) string {
	key := entitlementActiveCacheKey{
		UserID: input.UserID,
		Now:    timeBucket(input.Now),
	}
	return cacheKey("entitlements:active", key)
}

func latestEntitlementKey(userID uint) string {
	return cacheKey("entitlements:latest", entitlementLatestCacheKey{UserID: userID})
}
