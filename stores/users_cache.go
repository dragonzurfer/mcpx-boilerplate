package stores

type userByIDCacheKey struct {
	UserID uint `json:"user_id"`
}

type userMetricsCacheKey struct {
	UserID uint `json:"user_id"`
}

type userListCacheKey struct {
	Query    string `json:"query,omitempty"`
	Stage    string `json:"stage,omitempty"`
	Page     int    `json:"page,omitempty"`
	PageSize int    `json:"page_size,omitempty"`
}

func (s *Store) getCachedUserByID(key string) (*UserModel, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return nil, false
	}
	typed, ok := value.(*UserModel)
	if !ok || typed == nil {
		return nil, false
	}
	return typed, true
}

func (s *Store) getCachedUserMetrics(key string) (*UserMetricsModel, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return nil, false
	}
	typed, ok := value.(*UserMetricsModel)
	if !ok || typed == nil {
		return nil, false
	}
	return typed, true
}

func (s *Store) getCachedUserList(key string) (UserListOutput, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return UserListOutput{}, false
	}
	typed, ok := value.(UserListOutput)
	if !ok {
		return UserListOutput{}, false
	}
	return typed, true
}

func userByIDKey(userID uint) string {
	return cacheKey("users:id", userByIDCacheKey{UserID: userID})
}

func userMetricsKey(userID uint) string {
	return cacheKey("users:metrics", userMetricsCacheKey{UserID: userID})
}

func userListKey(input UserListInput) string {
	key := userListCacheKey{
		Query:    normalizeString(input.Query),
		Stage:    normalizeString(input.Stage),
		Page:     input.Page,
		PageSize: input.PageSize,
	}
	return cacheKey("users:list", key)
}
