package stores

type postLookupCacheKey struct {
	Slug        string `json:"slug"`
	PostID      uint   `json:"post_id"`
	IncludeTags bool   `json:"include_tags"`
}

type postListCacheKey struct {
	AccessLevels []string `json:"access_levels,omitempty"`
	TagID        uint     `json:"tag_id,omitempty"`
	Query        string   `json:"query,omitempty"`
	Status       string   `json:"status,omitempty"`
	Page         int      `json:"page,omitempty"`
	PageSize     int      `json:"page_size,omitempty"`
}

type postLookupCacheValue struct {
	Post *PostModel
	Tags []TagModel
}

type publishedPostsCacheKey struct {
	AccessLevel string `json:"access_level,omitempty"`
}

func (s *Store) getCachedPostLookup(key string) (*PostModel, []TagModel, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return nil, nil, false
	}
	typed, ok := value.(postLookupCacheValue)
	if !ok || typed.Post == nil {
		return nil, nil, false
	}
	return typed.Post, typed.Tags, true
}

func (s *Store) getCachedPostList(key string) (PostListOutput, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return PostListOutput{}, false
	}
	typed, ok := value.(PostListOutput)
	if !ok {
		return PostListOutput{}, false
	}
	return typed, true
}

func (s *Store) getCachedPublishedPosts(key string) ([]PostModel, bool) {
	value, ok := s.cacheGet(key)
	if !ok {
		return nil, false
	}
	typed, ok := value.([]PostModel)
	if !ok {
		return nil, false
	}
	return typed, true
}

func postLookupKeyBySlug(slug string, includeTags bool) string {
	key := postLookupCacheKey{
		Slug:        normalizeString(slug),
		IncludeTags: includeTags,
	}
	return cacheKey("posts:slug", key)
}

func postLookupKeyByID(postID uint, includeTags bool) string {
	key := postLookupCacheKey{
		PostID:      postID,
		IncludeTags: includeTags,
	}
	return cacheKey("posts:id", key)
}

func postListKey(input PostListInput) string {
	key := postListCacheKey{
		AccessLevels: normalizeAccessLevels(input.AccessLevels),
		TagID:        input.TagID,
		Query:        normalizeString(input.Query),
		Status:       normalizeString(input.Status),
		Page:         input.Page,
		PageSize:     input.PageSize,
	}
	return cacheKey("posts:list", key)
}

func publishedPostsKey(input PublishedPostsInput) string {
	key := publishedPostsCacheKey{
		AccessLevel: normalizeString(input.AccessLevel),
	}
	return cacheKey("posts:published", key)
}
