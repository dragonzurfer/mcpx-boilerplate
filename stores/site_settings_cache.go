package stores

type siteSettingsCacheKey struct {
	Static string `json:"static"`
}

func (s *Store) getCachedSiteSettings(key string) (*SiteSettingsModel, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return nil, false
	}
	typed, ok := value.(*SiteSettingsModel)
	if !ok || typed == nil {
		return nil, false
	}
	return typed, true
}

func siteSettingsKey() string {
	return cacheKey("site:settings", siteSettingsCacheKey{Static: "v1"})
}
