package stores

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

type CourseLookupInput struct {
	Slug string
}

func (s *Store) GetCourseBySlug(input CourseLookupInput) (*CourseModel, error) {
	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		return nil, gorm.ErrRecordNotFound
	}
	course := CourseModel{}
	if err := s.db.Where("slug = ?", slug).First(&course).Error; err != nil {
		return nil, err
	}
	return &course, nil
}

type CourseListInput struct {
	AccessLevels []string
	Status       string
	Query        string
	Page         int
	PageSize     int
}

type CourseListOutput struct {
	Courses []CourseModel
	Total   int64
}

func (s *Store) ListCourses(input CourseListInput) (CourseListOutput, error) {
	query := s.db.Model(&CourseModel{})
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

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return CourseListOutput{}, err
	}

	pageSize := clampPageSize(input.PageSize)
	offset := clampPage(input.Page) * pageSize
	courses := []CourseModel{}
	if err := query.Order("published_at desc").Limit(pageSize).Offset(offset).Find(&courses).Error; err != nil {
		return CourseListOutput{}, err
	}

	return CourseListOutput{Courses: courses, Total: total}, nil
}

type CourseCreateInput struct {
	Slug           string
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
	NoIndex        bool
}

func (s *Store) CreateCourse(input CourseCreateInput) (*CourseModel, error) {
	slug := strings.TrimSpace(input.Slug)
	title := strings.TrimSpace(input.Title)
	if slug == "" || title == "" {
		return nil, gorm.ErrInvalidData
	}

	course := CourseModel{
		Slug:            slug,
		Title:           title,
		Excerpt:         strings.TrimSpace(input.Excerpt),
		BodyMarkdown:    input.BodyMarkdown,
		AccessLevel:     input.AccessLevel,
		Status:          input.Status,
		PublishedAt:     input.PublishedAt,
		MetaTitle:       strings.TrimSpace(input.MetaTitle),
		MetaDescription: strings.TrimSpace(input.MetaDescription),
		MetaImageURL:    strings.TrimSpace(input.MetaImageURL),
		CanonicalURL:    strings.TrimSpace(input.CanonicalURL),
		NoIndex:         input.NoIndex,
		CreatedAt:       time.Now().UTC(),
		UpdatedAt:       time.Now().UTC(),
	}

	if err := s.db.Create(&course).Error; err != nil {
		return nil, err
	}
	return &course, nil
}

type CourseUpdateInput struct {
	CourseID       uint
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
}

func (s *Store) UpdateCourse(input CourseUpdateInput) (*CourseModel, error) {
	if input.CourseID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	course := CourseModel{}
	if err := s.db.Where("id = ?", input.CourseID).First(&course).Error; err != nil {
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
		if err := s.db.Model(&course).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	return &course, nil
}

func (s *Store) UpdateCourseHTMLCache(courseID uint, html string) error {
	if courseID == 0 {
		return gorm.ErrRecordNotFound
	}
	updates := map[string]interface{}{
		"body_html_cache": html,
		"updated_at":      time.Now().UTC(),
	}
	return s.db.Model(&CourseModel{}).Where("id = ?", courseID).Updates(updates).Error
}
