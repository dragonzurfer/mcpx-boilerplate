package stores

type courseLookupCacheKey struct {
	Slug string `json:"slug"`
}

type courseListCacheKey struct {
	AccessLevels []string `json:"access_levels,omitempty"`
	Status       string   `json:"status,omitempty"`
	Query        string   `json:"query,omitempty"`
	Page         int      `json:"page,omitempty"`
	PageSize     int      `json:"page_size,omitempty"`
}

func (s *Store) getCachedCourseLookup(key string) (*CourseModel, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return nil, false
	}
	typed, ok := value.(*CourseModel)
	if !ok || typed == nil {
		return nil, false
	}
	return typed, true
}

func (s *Store) getCachedCourseList(key string) (CourseListOutput, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return CourseListOutput{}, false
	}
	typed, ok := value.(CourseListOutput)
	if !ok {
		return CourseListOutput{}, false
	}
	return typed, true
}

func courseLookupKey(slug string) string {
	key := courseLookupCacheKey{Slug: normalizeString(slug)}
	return cacheKey("courses:slug", key)
}

func courseListKey(input CourseListInput) string {
	key := courseListCacheKey{
		AccessLevels: normalizeAccessLevels(input.AccessLevels),
		Status:       normalizeString(input.Status),
		Query:        normalizeString(input.Query),
		Page:         input.Page,
		PageSize:     input.PageSize,
	}
	return cacheKey("courses:list", key)
}
