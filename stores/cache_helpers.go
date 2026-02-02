package stores

func (s *Store) EnableCache(config CacheConfig) {
	if s == nil {
		return
	}
	s.cache = NewCache(config)
}

func (s *Store) cacheGet(key string) (any, bool) {
	if s == nil || s.cache == nil {
		return nil, false
	}
	return s.cache.Get(key)
}

func (s *Store) cacheSet(key string, value any) {
	if s == nil || s.cache == nil {
		return
	}
	s.cache.Set(CacheSetInput{Key: key, Value: value})
}
