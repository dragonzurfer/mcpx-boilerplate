package stores

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type PostLookupInput struct {
	Slug        string
	IncludeTags bool
}

func (s *Store) GetPostBySlug(input PostLookupInput) (*PostModel, []TagModel, error) {
	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		return nil, nil, gorm.ErrRecordNotFound
	}

	key := postLookupKeyBySlug(slug, input.IncludeTags)
	if post, tags, ok := s.getCachedPostLookup(key); ok {
		return post, tags, nil
	}

	post, tags, err := s.fetchPostBySlug(PostLookupInput{Slug: slug, IncludeTags: input.IncludeTags})
	if err != nil {
		return nil, nil, err
	}

	s.cacheSet(key, postLookupCacheValue{Post: post, Tags: tags})
	return post, tags, nil
}

type PostIDLookupInput struct {
	PostID      uint
	IncludeTags bool
}

func (s *Store) GetPostByID(input PostIDLookupInput) (*PostModel, []TagModel, error) {
	if input.PostID == 0 {
		return nil, nil, gorm.ErrRecordNotFound
	}

	key := postLookupKeyByID(input.PostID, input.IncludeTags)
	if post, tags, ok := s.getCachedPostLookup(key); ok {
		return post, tags, nil
	}

	post, tags, err := s.fetchPostByID(input)
	if err != nil {
		return nil, nil, err
	}

	s.cacheSet(key, postLookupCacheValue{Post: post, Tags: tags})
	return post, tags, nil
}

type PostListInput struct {
	AccessLevels []string
	TagID        uint
	Query        string
	Status       string
	Page         int
	PageSize     int
}

type PostListOutput struct {
	Posts []PostModel
	Total int64
}

func (s *Store) ListPosts(input PostListInput) (PostListOutput, error) {
	key := postListKey(input)
	if cached, ok := s.getCachedPostList(key); ok {
		return cached, nil
	}

	output, err := s.fetchPostList(input)
	if err != nil {
		return PostListOutput{}, err
	}

	s.cacheSet(key, output)
	return output, nil
}

func (s *Store) fetchPostBySlug(input PostLookupInput) (*PostModel, []TagModel, error) {
	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		return nil, nil, gorm.ErrRecordNotFound
	}

	post := PostModel{}
	if err := s.db.Where("slug = ?", slug).First(&post).Error; err != nil {
		return nil, nil, err
	}

	tags, err := s.fetchPostTags(post.ID, input.IncludeTags)
	if err != nil {
		return &post, tags, err
	}
	return &post, tags, nil
}

func (s *Store) fetchPostByID(input PostIDLookupInput) (*PostModel, []TagModel, error) {
	if input.PostID == 0 {
		return nil, nil, gorm.ErrRecordNotFound
	}

	post := PostModel{}
	if err := s.db.Where("id = ?", input.PostID).First(&post).Error; err != nil {
		return nil, nil, err
	}

	tags, err := s.fetchPostTags(post.ID, input.IncludeTags)
	if err != nil {
		return &post, tags, err
	}
	return &post, tags, nil
}

func (s *Store) fetchPostTags(postID uint, includeTags bool) ([]TagModel, error) {
	tags := []TagModel{}
	if !includeTags {
		return tags, nil
	}

	tagIDs, err := s.tagIDsForPost(postID)
	if err != nil {
		return tags, err
	}
	if len(tagIDs) == 0 {
		return tags, nil
	}

	if err := s.db.Where("id IN ?", tagIDs).Find(&tags).Error; err != nil {
		return tags, err
	}
	return tags, nil
}

func (s *Store) fetchPostList(input PostListInput) (PostListOutput, error) {
	query := s.db.Model(&PostModel{})
	if len(input.AccessLevels) > 0 {
		query = query.Where("access_level IN ?", input.AccessLevels)
	}
	if input.Status != "" {
		query = query.Where("status = ?", input.Status)
	}
	if input.Query != "" {
		like := "%" + input.Query + "%"
		query = query.Where("title LIKE ? OR excerpt LIKE ?", like, like)
	}
	if input.TagID > 0 {
		query = query.Joins("JOIN post_tags ON post_tags.post_id = posts.id").Where("post_tags.tag_id = ?", input.TagID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return PostListOutput{}, err
	}

	pageSize := clampPageSize(input.PageSize)
	offset := clampPage(input.Page) * pageSize

	posts := []PostModel{}
	if err := query.Order("published_at desc").Limit(pageSize).Offset(offset).Find(&posts).Error; err != nil {
		return PostListOutput{}, err
	}

	return PostListOutput{Posts: posts, Total: total}, nil
}

type PostCreateInput struct {
	Slug           string
	Title          string
	Excerpt        string
	BodyMarkdown   string
	AccessLevel    string
	Status         string
	PublishedAt    *time.Time
	CreatedBy      uint
	MetaTitle      string
	MetaDescription string
	MetaImageURL   string
	CanonicalURL   string
	NoIndex        bool
	TagIDs         []uint
}

func (s *Store) CreatePost(input PostCreateInput) (*PostModel, error) {
	slug := strings.TrimSpace(input.Slug)
	title := strings.TrimSpace(input.Title)
	if slug == "" || title == "" {
		return nil, gorm.ErrInvalidData
	}

	post := PostModel{
		Slug:           slug,
		Title:          title,
		Excerpt:        strings.TrimSpace(input.Excerpt),
		BodyMarkdown:   input.BodyMarkdown,
		AccessLevel:    input.AccessLevel,
		Status:         input.Status,
		PublishedAt:    input.PublishedAt,
		CreatedBy:      input.CreatedBy,
		MetaTitle:      strings.TrimSpace(input.MetaTitle),
		MetaDescription: strings.TrimSpace(input.MetaDescription),
		MetaImageURL:   strings.TrimSpace(input.MetaImageURL),
		CanonicalURL:   strings.TrimSpace(input.CanonicalURL),
		NoIndex:        input.NoIndex,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if err := s.db.Create(&post).Error; err != nil {
		return nil, err
	}

	if err := s.replacePostTags(post.ID, input.TagIDs); err != nil {
		return &post, err
	}
	return &post, nil
}

type PostUpdateInput struct {
	PostID         uint
	Title          string
	Excerpt        string
	BodyMarkdown   string
	AccessLevel    string
	Status         string
	PublishedAt    *time.Time
	MetaTitle      string
	MetaDescription string
	MetaImageURL   string
	CanonicalURL   string
	NoIndex        *bool
	TagIDs         []uint
}

func (s *Store) UpdatePost(input PostUpdateInput) (*PostModel, error) {
	if input.PostID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	post := PostModel{}
	if err := s.db.Where("id = ?", input.PostID).First(&post).Error; err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if input.Title != "" {
		updates["title"] = strings.TrimSpace(input.Title)
	}
	if input.Excerpt != "" {
		updates["excerpt"] = strings.TrimSpace(input.Excerpt)
	}
	if input.BodyMarkdown != "" {
		updates["body_markdown"] = input.BodyMarkdown
		updates["body_html_cache"] = ""
	}
	if input.AccessLevel != "" {
		updates["access_level"] = input.AccessLevel
	}
	if input.Status != "" {
		updates["status"] = input.Status
	}
	if input.PublishedAt != nil {
		updates["published_at"] = input.PublishedAt
	}
	if input.MetaTitle != "" {
		updates["meta_title"] = strings.TrimSpace(input.MetaTitle)
	}
	if input.MetaDescription != "" {
		updates["meta_description"] = strings.TrimSpace(input.MetaDescription)
	}
	if input.MetaImageURL != "" {
		updates["meta_image_url"] = strings.TrimSpace(input.MetaImageURL)
	}
	if input.CanonicalURL != "" {
		updates["canonical_url"] = strings.TrimSpace(input.CanonicalURL)
	}
	if input.NoIndex != nil {
		updates["no_index"] = *input.NoIndex
	}

	if len(updates) > 0 {
		updates["updated_at"] = time.Now().UTC()
		if err := s.db.Model(&post).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	if input.TagIDs != nil {
		if err := s.replacePostTags(post.ID, input.TagIDs); err != nil {
			return &post, err
		}
	}

	return &post, nil
}

type PostHTMLCacheInput struct {
	PostID uint
	HTML   string
}

func (s *Store) UpdatePostHTMLCache(input PostHTMLCacheInput) error {
	if input.PostID == 0 {
		return gorm.ErrRecordNotFound
	}

	updates := map[string]interface{}{
		"body_html_cache": input.HTML,
		"updated_at":      time.Now().UTC(),
	}
	return s.db.Model(&PostModel{}).Where("id = ?", input.PostID).Updates(updates).Error
}

func (s *Store) replacePostTags(postID uint, tagIDs []uint) error {
	if postID == 0 {
		return gorm.ErrRecordNotFound
	}

	if err := s.db.Where("post_id = ?", postID).Delete(&PostTagModel{}).Error; err != nil {
		return err
	}
	if len(tagIDs) == 0 {
		return nil
	}

	rows := make([]PostTagModel, 0, len(tagIDs))
	for _, tagID := range tagIDs {
		if tagID == 0 {
			continue
		}
		rows = append(rows, PostTagModel{PostID: postID, TagID: tagID})
	}
	if len(rows) == 0 {
		return nil
	}

	return s.db.Create(&rows).Error
}

func (s *Store) tagIDsForPost(postID uint) ([]uint, error) {
	ids := []uint{}
	if postID == 0 {
		return ids, nil
	}

	tags := []PostTagModel{}
	if err := s.db.Where("post_id = ?", postID).Find(&tags).Error; err != nil {
		return ids, err
	}
	for _, tag := range tags {
		ids = append(ids, tag.TagID)
	}
	return ids, nil
}

func clampPage(page int) int {
	if page < 0 {
		return 0
	}
	return page
}

func clampPageSize(size int) int {
	if size <= 0 {
		return 20
	}
	if size > 100 {
		return 100
	}
	return size
}

func (s *Store) GetOrCreateTag(tagName, tagType string) (*TagModel, error) {
	tagName = strings.TrimSpace(tagName)
	if tagName == "" {
		return nil, gorm.ErrInvalidData
	}
	tagType = strings.TrimSpace(tagType)

	model := TagModel{}
	err := s.db.Where("name = ?", tagName).First(&model).Error
	if err == nil {
		if tagType != "" && model.TagType != tagType {
			updates := map[string]interface{}{
				"tag_type":  tagType,
				"updated_at": time.Now().UTC(),
			}
			if err := s.db.Model(&model).Updates(updates).Error; err != nil {
				return nil, err
			}
		}
		return &model, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	model = TagModel{
		Name:      tagName,
		TagType:   tagType,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.db.Create(&model).Error; err != nil {
		return nil, err
	}
	return &model, nil
}
