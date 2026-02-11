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

	key := courseLookupKey(slug)
	if cachedCourse, ok := s.getCachedCourseLookup(key); ok {
		return cachedCourse, nil
	}

	courseModel := CourseModel{}
	if err := s.db.Where("slug = ?", slug).First(&courseModel).Error; err != nil {
		return nil, err
	}

	s.cacheSet(key, &courseModel)
	return &courseModel, nil
}

type CourseByIDLookupInput struct {
	CourseID uint
}

func (s *Store) GetCourseByID(input CourseByIDLookupInput) (*CourseModel, error) {
	if input.CourseID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	courseModel := CourseModel{}
	if err := s.db.Where("id = ?", input.CourseID).First(&courseModel).Error; err != nil {
		return nil, err
	}

	return &courseModel, nil
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
	key := courseListKey(input)
	if cachedCourses, ok := s.getCachedCourseList(key); ok {
		return cachedCourses, nil
	}

	courseListOutput, err := s.fetchCourseList(input)
	if err != nil {
		return CourseListOutput{}, err
	}

	s.cacheSet(key, courseListOutput)
	return courseListOutput, nil
}

func (s *Store) fetchCourseList(input CourseListInput) (CourseListOutput, error) {
	courseQuery := s.db.Model(&CourseModel{})
	if len(input.AccessLevels) > 0 {
		courseQuery = courseQuery.Where("access_level IN ?", input.AccessLevels)
	}

	if input.Status != "" {
		courseQuery = courseQuery.Where("status = ?", input.Status)
	}

	searchQuery := strings.TrimSpace(input.Query)
	if searchQuery != "" {
		likePattern := "%" + searchQuery + "%"
		courseQuery = courseQuery.Where("title LIKE ? OR description LIKE ? OR excerpt LIKE ?", likePattern, likePattern, likePattern)
	}

	var totalCourses int64
	if err := courseQuery.Count(&totalCourses).Error; err != nil {
		return CourseListOutput{}, err
	}

	pageSize := clampPageSize(input.PageSize)
	offset := clampPage(input.Page) * pageSize

	courseModels := []CourseModel{}
	if err := courseQuery.Order("published_at desc, id desc").Limit(pageSize).Offset(offset).Find(&courseModels).Error; err != nil {
		return CourseListOutput{}, err
	}

	return CourseListOutput{Courses: courseModels, Total: totalCourses}, nil
}

type CourseCreateInput struct {
	Slug            string
	Title           string
	Description     string
	Excerpt         string
	ThumbnailURL    string
	Metadata        CourseMetadata
	BodyMarkdown    string
	AccessLevel     string
	Status          string
	PublishedAt     *time.Time
	MetaTitle       string
	MetaDescription string
	MetaImageURL    string
	CanonicalURL    string
	NoIndex         bool
}

func (s *Store) CreateCourse(input CourseCreateInput) (*CourseModel, error) {
	slug := strings.TrimSpace(input.Slug)
	title := strings.TrimSpace(input.Title)
	if slug == "" || title == "" {
		return nil, gorm.ErrInvalidData
	}

	courseDescription := resolveCourseDescription(input.Description, input.Excerpt)
	now := time.Now().UTC()

	courseModel := CourseModel{
		Slug:            slug,
		Title:           title,
		Excerpt:         courseDescription,
		Description:     courseDescription,
		ThumbnailURL:    strings.TrimSpace(input.ThumbnailURL),
		MetadataJSON:    SerializeCourseMetadata(input.Metadata),
		BodyMarkdown:    input.BodyMarkdown,
		AccessLevel:     normalizeCourseAccessLevel(input.AccessLevel),
		Status:          normalizeCourseStatus(input.Status),
		PublishedAt:     input.PublishedAt,
		MetaTitle:       strings.TrimSpace(input.MetaTitle),
		MetaDescription: strings.TrimSpace(input.MetaDescription),
		MetaImageURL:    strings.TrimSpace(input.MetaImageURL),
		CanonicalURL:    strings.TrimSpace(input.CanonicalURL),
		NoIndex:         input.NoIndex,
		CreatedAt:       now,
		UpdatedAt:       now,
	}

	if err := s.db.Create(&courseModel).Error; err != nil {
		return nil, err
	}

	return &courseModel, nil
}

type CourseUpdateInput struct {
	CourseID        uint
	Title           string
	Description     string
	Excerpt         string
	ThumbnailURL    string
	Metadata        *CourseMetadata
	BodyMarkdown    string
	AccessLevel     string
	Status          string
	PublishedAt     *time.Time
	MetaTitle       string
	MetaDescription string
	MetaImageURL    string
	CanonicalURL    string
	NoIndex         *bool
}

func (s *Store) UpdateCourse(input CourseUpdateInput) (*CourseModel, error) {
	if input.CourseID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	courseModel := CourseModel{}
	if err := s.db.Where("id = ?", input.CourseID).First(&courseModel).Error; err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if input.Title != "" {
		updates["title"] = strings.TrimSpace(input.Title)
	}

	if input.Description != "" || input.Excerpt != "" {
		courseDescription := resolveCourseDescription(input.Description, input.Excerpt)
		updates["description"] = courseDescription
		updates["excerpt"] = courseDescription
	}

	if input.ThumbnailURL != "" {
		updates["thumbnail_url"] = strings.TrimSpace(input.ThumbnailURL)
	}
	if input.Metadata != nil {
		updates["metadata_json"] = SerializeCourseMetadata(*input.Metadata)
	}
	if input.BodyMarkdown != "" {
		updates["body_markdown"] = input.BodyMarkdown
		updates["body_html_cache"] = ""
	}
	if input.AccessLevel != "" {
		updates["access_level"] = normalizeCourseAccessLevel(input.AccessLevel)
	}
	if input.Status != "" {
		updates["status"] = normalizeCourseStatus(input.Status)
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
		if err := s.db.Model(&courseModel).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	if err := s.db.Where("id = ?", input.CourseID).First(&courseModel).Error; err != nil {
		return nil, err
	}
	return &courseModel, nil
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

type CourseLessonHTMLCacheInput struct {
	LessonID uint
	HTML     string
}

func (s *Store) UpdateCourseLessonHTMLCache(input CourseLessonHTMLCacheInput) error {
	if input.LessonID == 0 {
		return gorm.ErrRecordNotFound
	}

	updates := map[string]interface{}{
		"body_html_cache": input.HTML,
		"updated_at":      time.Now().UTC(),
	}
	query := s.db.Model(&CourseLessonModel{}).Where("id = ?", input.LessonID)
	return query.Updates(updates).Error
}

type CourseDeleteInput struct {
	CourseID uint
}

func (s *Store) DeleteCourse(input CourseDeleteInput) error {
	if input.CourseID == 0 {
		return gorm.ErrRecordNotFound
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("course_id = ?", input.CourseID).Delete(&CourseLessonModel{}).Error; err != nil {
			return err
		}
		if err := tx.Where("course_id = ?", input.CourseID).Delete(&CourseModuleModel{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", input.CourseID).Delete(&CourseModel{}).Error
	})
}

type CourseStructureInput struct {
	CourseID           uint
	IncludeUnpublished bool
}

type CourseModuleWithLessons struct {
	Module  CourseModuleModel
	Lessons []CourseLessonModel
}

type CourseStructureOutput struct {
	Modules     []CourseModuleWithLessons
	LessonCount int
}

func (s *Store) GetCourseStructure(input CourseStructureInput) (CourseStructureOutput, error) {
	if input.CourseID == 0 {
		return CourseStructureOutput{}, gorm.ErrRecordNotFound
	}

	moduleModels := []CourseModuleModel{}
	moduleQuery := s.db.Where("course_id = ?", input.CourseID).Order("position asc, id asc")
	if err := moduleQuery.Find(&moduleModels).Error; err != nil {
		return CourseStructureOutput{}, err
	}

	moduleIDs := collectModuleIDs(moduleModels)
	lessonModels, err := s.listLessonsForStructure(input.CourseID, moduleIDs, input.IncludeUnpublished)
	if err != nil {
		return CourseStructureOutput{}, err
	}

	modulesWithLessons := attachLessonsToModules(moduleModels, lessonModels)
	return CourseStructureOutput{Modules: modulesWithLessons, LessonCount: len(lessonModels)}, nil
}

func collectModuleIDs(moduleModels []CourseModuleModel) []uint {
	moduleIDs := make([]uint, 0, len(moduleModels))
	for _, moduleModel := range moduleModels {
		moduleIDs = append(moduleIDs, moduleModel.ID)
	}
	return moduleIDs
}

func (s *Store) listLessonsForStructure(courseID uint, moduleIDs []uint, includeUnpublished bool) ([]CourseLessonModel, error) {
	if len(moduleIDs) == 0 {
		return []CourseLessonModel{}, nil
	}

	lessonModels := []CourseLessonModel{}
	lessonQuery := s.db.Where("course_id = ?", courseID).Where("module_id IN ?", moduleIDs)
	if !includeUnpublished {
		lessonQuery = lessonQuery.Where("status = ?", CourseStatusPublished)
	}

	if err := lessonQuery.Order("module_id asc, position asc, id asc").Find(&lessonModels).Error; err != nil {
		return nil, err
	}
	return lessonModels, nil
}

func attachLessonsToModules(moduleModels []CourseModuleModel, lessonModels []CourseLessonModel) []CourseModuleWithLessons {
	lessonsByModule := map[uint][]CourseLessonModel{}
	for _, lessonModel := range lessonModels {
		lessonsByModule[lessonModel.ModuleID] = append(lessonsByModule[lessonModel.ModuleID], lessonModel)
	}

	modulesWithLessons := make([]CourseModuleWithLessons, 0, len(moduleModels))
	for _, moduleModel := range moduleModels {
		modulesWithLessons = append(modulesWithLessons, CourseModuleWithLessons{
			Module:  moduleModel,
			Lessons: lessonsByModule[moduleModel.ID],
		})
	}

	return modulesWithLessons
}

type CourseContentCount struct {
	ModuleCount int
	LessonCount int
}

type CourseContentCountsInput struct {
	CourseIDs          []uint
	IncludeUnpublished bool
}

func (s *Store) GetCourseContentCounts(input CourseContentCountsInput) (map[uint]CourseContentCount, error) {
	if len(input.CourseIDs) == 0 {
		return map[uint]CourseContentCount{}, nil
	}

	courseCounts := map[uint]CourseContentCount{}
	for _, courseID := range input.CourseIDs {
		courseCounts[courseID] = CourseContentCount{}
	}

	if err := s.fillModuleCounts(input.CourseIDs, courseCounts); err != nil {
		return nil, err
	}
	if err := s.fillLessonCounts(input, courseCounts); err != nil {
		return nil, err
	}

	return courseCounts, nil
}

type groupedCountRow struct {
	CourseID uint
	Count    int
}

func (s *Store) fillModuleCounts(courseIDs []uint, courseCounts map[uint]CourseContentCount) error {
	rows := []groupedCountRow{}
	query := s.db.Model(&CourseModuleModel{}).
		Select("course_id as course_id, count(*) as count").
		Where("course_id IN ?", courseIDs).
		Group("course_id")
	if err := query.Scan(&rows).Error; err != nil {
		return err
	}

	for _, row := range rows {
		count := courseCounts[row.CourseID]
		count.ModuleCount = row.Count
		courseCounts[row.CourseID] = count
	}
	return nil
}

func (s *Store) fillLessonCounts(input CourseContentCountsInput, courseCounts map[uint]CourseContentCount) error {
	rows := []groupedCountRow{}
	query := s.db.Model(&CourseLessonModel{}).
		Select("course_id as course_id, count(*) as count").
		Where("course_id IN ?", input.CourseIDs)
	if !input.IncludeUnpublished {
		query = query.Where("status = ?", CourseStatusPublished)
	}
	query = query.Group("course_id")

	if err := query.Scan(&rows).Error; err != nil {
		return err
	}

	for _, row := range rows {
		count := courseCounts[row.CourseID]
		count.LessonCount = row.Count
		courseCounts[row.CourseID] = count
	}
	return nil
}

type CourseModuleCreateInput struct {
	CourseID uint
	Title    string
	Position int
}

func (s *Store) CreateCourseModule(input CourseModuleCreateInput) (*CourseModuleModel, error) {
	if input.CourseID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	title := strings.TrimSpace(input.Title)
	if title == "" {
		return nil, gorm.ErrInvalidData
	}

	modulePosition, err := s.resolveModulePosition(input)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	moduleModel := CourseModuleModel{
		CourseID:  input.CourseID,
		Title:     title,
		Position:  modulePosition,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.db.Create(&moduleModel).Error; err != nil {
		return nil, err
	}
	if err := s.syncCourseMetadataCounts(input.CourseID); err != nil {
		return nil, err
	}
	return &moduleModel, nil
}

func (s *Store) resolveModulePosition(input CourseModuleCreateInput) (int, error) {
	if input.Position > 0 {
		return input.Position, nil
	}

	nextPosition, err := s.nextModulePosition(input.CourseID)
	if err != nil {
		return 0, err
	}
	return nextPosition, nil
}

func (s *Store) nextModulePosition(courseID uint) (int, error) {
	var maxPosition int
	query := s.db.Model(&CourseModuleModel{}).
		Where("course_id = ?", courseID).
		Select("COALESCE(MAX(position), 0)")
	if err := query.Scan(&maxPosition).Error; err != nil {
		return 0, err
	}
	return maxPosition + 1, nil
}

type CourseModuleUpdateInput struct {
	CourseID uint
	ModuleID uint
	Title    string
	Position *int
}

func (s *Store) UpdateCourseModule(input CourseModuleUpdateInput) (*CourseModuleModel, error) {
	moduleModel, err := s.findCourseModule(input.CourseID, input.ModuleID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if input.Title != "" {
		updates["title"] = strings.TrimSpace(input.Title)
	}
	if input.Position != nil && *input.Position > 0 {
		updates["position"] = *input.Position
	}

	if len(updates) > 0 {
		updates["updated_at"] = time.Now().UTC()
		if err := s.db.Model(moduleModel).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	if err := s.db.Where("id = ?", input.ModuleID).First(moduleModel).Error; err != nil {
		return nil, err
	}
	return moduleModel, nil
}

func (s *Store) findCourseModule(courseID uint, moduleID uint) (*CourseModuleModel, error) {
	if courseID == 0 || moduleID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	moduleModel := CourseModuleModel{}
	query := s.db.Where("id = ? AND course_id = ?", moduleID, courseID)
	if err := query.First(&moduleModel).Error; err != nil {
		return nil, err
	}
	return &moduleModel, nil
}

type CourseModuleDeleteInput struct {
	CourseID uint
	ModuleID uint
}

func (s *Store) DeleteCourseModule(input CourseModuleDeleteInput) error {
	if input.CourseID == 0 || input.ModuleID == 0 {
		return gorm.ErrRecordNotFound
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("course_id = ? AND module_id = ?", input.CourseID, input.ModuleID).Delete(&CourseLessonModel{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ? AND course_id = ?", input.ModuleID, input.CourseID).Delete(&CourseModuleModel{}).Error
	}); err != nil {
		return err
	}

	return s.syncCourseMetadataCounts(input.CourseID)
}

type CourseModuleReorderInput struct {
	CourseID  uint
	ModuleIDs []uint
}

func (s *Store) ReorderCourseModules(input CourseModuleReorderInput) error {
	if input.CourseID == 0 || len(input.ModuleIDs) == 0 {
		return gorm.ErrInvalidData
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		for index, moduleID := range input.ModuleIDs {
			if moduleID == 0 {
				continue
			}
			updates := map[string]interface{}{
				"position":   index + 1,
				"updated_at": time.Now().UTC(),
			}
			query := tx.Model(&CourseModuleModel{}).Where("id = ? AND course_id = ?", moduleID, input.CourseID)
			if err := query.Updates(updates).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

type CourseLessonCreateInput struct {
	CourseID     uint
	ModuleID     uint
	Title        string
	Slug         string
	BodyMarkdown string
	VimeoURL     string
	Position     int
	IsFree       bool
	Status       string
	PublishedAt  *time.Time
}

func (s *Store) CreateCourseLesson(input CourseLessonCreateInput) (*CourseLessonModel, error) {
	if input.CourseID == 0 || input.ModuleID == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	if _, err := s.findCourseModule(input.CourseID, input.ModuleID); err != nil {
		return nil, err
	}

	lessonSlug := strings.TrimSpace(input.Slug)
	lessonTitle := strings.TrimSpace(input.Title)
	if lessonSlug == "" || lessonTitle == "" {
		return nil, gorm.ErrInvalidData
	}

	lessonPosition, err := s.resolveLessonPosition(input)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	lessonModel := CourseLessonModel{
		CourseID:     input.CourseID,
		ModuleID:     input.ModuleID,
		Title:        lessonTitle,
		Slug:         lessonSlug,
		BodyMarkdown: input.BodyMarkdown,
		VimeoURL:     strings.TrimSpace(input.VimeoURL),
		Position:     lessonPosition,
		IsFree:       input.IsFree,
		AccessLevel:  lessonAccessLevelFromFree(input.IsFree),
		Status:       normalizeCourseStatus(input.Status),
		PublishedAt:  input.PublishedAt,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.db.Create(&lessonModel).Error; err != nil {
		return nil, err
	}
	if err := s.syncCourseMetadataCounts(input.CourseID); err != nil {
		return nil, err
	}

	return &lessonModel, nil
}

func (s *Store) resolveLessonPosition(input CourseLessonCreateInput) (int, error) {
	if input.Position > 0 {
		return input.Position, nil
	}

	nextPosition, err := s.nextLessonPosition(input.CourseID, input.ModuleID)
	if err != nil {
		return 0, err
	}
	return nextPosition, nil
}

func (s *Store) nextLessonPosition(courseID uint, moduleID uint) (int, error) {
	var maxPosition int
	query := s.db.Model(&CourseLessonModel{}).
		Where("course_id = ? AND module_id = ?", courseID, moduleID).
		Select("COALESCE(MAX(position), 0)")
	if err := query.Scan(&maxPosition).Error; err != nil {
		return 0, err
	}
	return maxPosition + 1, nil
}

type CourseLessonUpdateInput struct {
	CourseID     uint
	ModuleID     uint
	LessonID     uint
	Title        string
	Slug         string
	BodyMarkdown string
	VimeoURL     string
	Position     *int
	IsFree       *bool
	Status       string
	PublishedAt  *time.Time
}

func (s *Store) UpdateCourseLesson(input CourseLessonUpdateInput) (*CourseLessonModel, error) {
	lessonModel, err := s.findCourseLessonByID(input)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if input.Title != "" {
		updates["title"] = strings.TrimSpace(input.Title)
	}
	if input.Slug != "" {
		updates["slug"] = strings.TrimSpace(input.Slug)
	}
	if input.BodyMarkdown != "" {
		updates["body_markdown"] = input.BodyMarkdown
		updates["body_html_cache"] = ""
	}
	if input.VimeoURL != "" {
		updates["vimeo_url"] = strings.TrimSpace(input.VimeoURL)
		updates["body_html_cache"] = ""
	}
	if input.Position != nil && *input.Position > 0 {
		updates["position"] = *input.Position
	}
	if input.IsFree != nil {
		updates["is_free"] = *input.IsFree
		updates["access_level"] = lessonAccessLevelFromFree(*input.IsFree)
	}
	if input.Status != "" {
		updates["status"] = normalizeCourseStatus(input.Status)
	}
	if input.PublishedAt != nil {
		updates["published_at"] = input.PublishedAt
	}

	if len(updates) > 0 {
		updates["updated_at"] = time.Now().UTC()
		if err := s.db.Model(lessonModel).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	if err := s.db.Where("id = ?", input.LessonID).First(lessonModel).Error; err != nil {
		return nil, err
	}
	return lessonModel, nil
}

func (s *Store) findCourseLessonByID(input CourseLessonUpdateInput) (*CourseLessonModel, error) {
	if input.CourseID == 0 || input.ModuleID == 0 || input.LessonID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	lessonModel := CourseLessonModel{}
	query := s.db.Where("id = ? AND module_id = ? AND course_id = ?", input.LessonID, input.ModuleID, input.CourseID)
	if err := query.First(&lessonModel).Error; err != nil {
		return nil, err
	}

	return &lessonModel, nil
}

type CourseLessonDeleteInput struct {
	CourseID uint
	ModuleID uint
	LessonID uint
}

func (s *Store) DeleteCourseLesson(input CourseLessonDeleteInput) error {
	if input.CourseID == 0 || input.ModuleID == 0 || input.LessonID == 0 {
		return gorm.ErrRecordNotFound
	}

	query := s.db.Where("id = ? AND module_id = ? AND course_id = ?", input.LessonID, input.ModuleID, input.CourseID)
	if err := query.Delete(&CourseLessonModel{}).Error; err != nil {
		return err
	}

	return s.syncCourseMetadataCounts(input.CourseID)
}

type CourseLessonReorderInput struct {
	CourseID  uint
	ModuleID  uint
	LessonIDs []uint
}

func (s *Store) ReorderCourseLessons(input CourseLessonReorderInput) error {
	if input.CourseID == 0 || input.ModuleID == 0 || len(input.LessonIDs) == 0 {
		return gorm.ErrInvalidData
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		for index, lessonID := range input.LessonIDs {
			if lessonID == 0 {
				continue
			}
			updates := map[string]interface{}{
				"position":   index + 1,
				"updated_at": time.Now().UTC(),
			}
			query := tx.Model(&CourseLessonModel{}).
				Where("id = ? AND course_id = ? AND module_id = ?", lessonID, input.CourseID, input.ModuleID)
			if err := query.Updates(updates).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

type CourseLessonSlugLookupInput struct {
	CourseID   uint
	LessonSlug string
}

func (s *Store) GetCourseLessonBySlug(input CourseLessonSlugLookupInput) (*CourseLessonModel, error) {
	if input.CourseID == 0 {
		return nil, gorm.ErrRecordNotFound
	}

	lessonSlug := strings.TrimSpace(input.LessonSlug)
	if lessonSlug == "" {
		return nil, gorm.ErrRecordNotFound
	}

	lessonModel := CourseLessonModel{}
	query := s.db.Where("course_id = ? AND slug = ?", input.CourseID, lessonSlug)
	if err := query.First(&lessonModel).Error; err != nil {
		return nil, err
	}

	return &lessonModel, nil
}

func resolveCourseDescription(description string, excerpt string) string {
	trimmedDescription := strings.TrimSpace(description)
	if trimmedDescription != "" {
		return trimmedDescription
	}
	return strings.TrimSpace(excerpt)
}

func normalizeCourseAccessLevel(rawAccessLevel string) string {
	accessLevel := strings.ToUpper(strings.TrimSpace(rawAccessLevel))
	if accessLevel == "" {
		return AccessLevelPublic
	}
	return accessLevel
}

func normalizeCourseStatus(rawStatus string) string {
	status := strings.ToUpper(strings.TrimSpace(rawStatus))
	if status == "" {
		return CourseStatusDraft
	}
	return status
}

func lessonAccessLevelFromFree(isFree bool) string {
	if isFree {
		return AccessLevelPublic
	}
	return AccessLevelPaid
}

func (s *Store) syncCourseMetadataCounts(courseID uint) error {
	if courseID == 0 {
		return gorm.ErrRecordNotFound
	}

	courseModel, err := s.GetCourseByID(CourseByIDLookupInput{CourseID: courseID})
	if err != nil {
		return err
	}

	courseCounts, err := s.GetCourseContentCounts(CourseContentCountsInput{
		CourseIDs:          []uint{courseID},
		IncludeUnpublished: true,
	})
	if err != nil {
		return err
	}
	count := courseCounts[courseID]

	courseMetadata := ParseCourseMetadata(courseModel.MetadataJSON)
	courseMetadata.ModuleCount = count.ModuleCount
	courseMetadata.LessonCount = count.LessonCount

	updates := map[string]interface{}{
		"metadata_json": SerializeCourseMetadata(courseMetadata),
		"updated_at":    time.Now().UTC(),
	}
	return s.db.Model(&CourseModel{}).Where("id = ?", courseID).Updates(updates).Error
}
