package stores

import "gorm.io/gorm"

type PublishedPostsInput struct {
	AccessLevel string
}

func (s *Store) ListPublishedPosts(input PublishedPostsInput) ([]PostModel, error) {
	if input.AccessLevel == "" {
		return []PostModel{}, gorm.ErrInvalidData
	}
	posts := []PostModel{}
	if err := s.db.Where("status = ? AND access_level = ?", PostStatusPublished, input.AccessLevel).Order("published_at desc").Find(&posts).Error; err != nil {
		return nil, err
	}
	return posts, nil
}
