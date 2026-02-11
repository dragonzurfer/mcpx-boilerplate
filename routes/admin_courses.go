package routes

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/stores"
)

type AdminCoursesHandler struct {
	Store *stores.Store
}

type adminCourseRequest struct {
	Slug            string                `json:"slug"`
	Title           string                `json:"title"`
	Description     string                `json:"description"`
	ThumbnailURL    string                `json:"thumbnail_url"`
	Metadata        stores.CourseMetadata `json:"metadata"`
	BodyMarkdown    string                `json:"body_markdown"`
	AccessLevel     string                `json:"access_level"`
	Status          string                `json:"status"`
	PublishedAt     *time.Time            `json:"published_at"`
	MetaTitle       string                `json:"meta_title"`
	MetaDescription string                `json:"meta_description"`
	MetaImageURL    string                `json:"meta_image_url"`
	CanonicalURL    string                `json:"canonical_url"`
	NoIndex         *bool                 `json:"noindex"`
}

type adminCourseModuleRequest struct {
	Title    string `json:"title"`
	Position *int   `json:"position"`
}

type adminCourseLessonRequest struct {
	Title        string     `json:"title"`
	Slug         string     `json:"slug"`
	BodyMarkdown string     `json:"body_markdown"`
	VimeoURL     string     `json:"vimeo_url"`
	Position     *int       `json:"position"`
	IsFree       *bool      `json:"is_free"`
	Status       string     `json:"status"`
	PublishedAt  *time.Time `json:"published_at"`
}

type adminCourseReorderModulesRequest struct {
	ModuleIDs []uint `json:"module_ids"`
}

type adminCourseReorderLessonsRequest struct {
	LessonIDs []uint `json:"lesson_ids"`
}

func (h *AdminCoursesHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/courses", h.list)
	rg.GET("/courses/:id", h.get)
	rg.POST("/courses", h.create)
	rg.PUT("/courses/:id", h.update)
	rg.DELETE("/courses/:id", h.delete)

	rg.POST("/courses/:id/modules", h.createModule)
	rg.PUT("/courses/:id/modules/:module_id", h.updateModule)
	rg.DELETE("/courses/:id/modules/:module_id", h.deleteModule)
	rg.PUT("/courses/:id/modules/reorder", h.reorderModules)

	rg.POST("/courses/:id/modules/:module_id/lessons", h.createLesson)
	rg.PUT("/courses/:id/modules/:module_id/lessons/:lesson_id", h.updateLesson)
	rg.DELETE("/courses/:id/modules/:module_id/lessons/:lesson_id", h.deleteLesson)
	rg.PUT("/courses/:id/modules/:module_id/lessons/reorder", h.reorderLessons)
}

func (h *AdminCoursesHandler) list(c *gin.Context) {
	status := strings.TrimSpace(c.Query("status"))
	query := strings.TrimSpace(c.Query("q"))
	page := parseIntDefault(c.Query("page"), 0)
	pageSize := parseIntDefault(c.Query("page_size"), 20)

	courseListOutput, err := h.Store.ListCourses(stores.CourseListInput{
		Status:   status,
		Query:    query,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load courses"})
		return
	}

	courseIDs := collectCourseIDs(courseListOutput.Courses)
	courseCounts, err := h.Store.GetCourseContentCounts(stores.CourseContentCountsInput{
		CourseIDs:          courseIDs,
		IncludeUnpublished: true,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load course counts"})
		return
	}

	courseItems := make([]gin.H, 0, len(courseListOutput.Courses))
	for _, courseModel := range courseListOutput.Courses {
		courseItems = append(courseItems, adminCourseSummary(courseModel, courseCounts[courseModel.ID]))
	}

	c.JSON(http.StatusOK, gin.H{"items": courseItems, "total": courseListOutput.Total})
}

func collectCourseIDs(courseModels []stores.CourseModel) []uint {
	courseIDs := make([]uint, 0, len(courseModels))
	for _, courseModel := range courseModels {
		courseIDs = append(courseIDs, courseModel.ID)
	}
	return courseIDs
}

func (h *AdminCoursesHandler) get(c *gin.Context) {
	courseID := parseUintDefault(c.Param("id"))
	if courseID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	courseModel, err := h.Store.GetCourseByID(stores.CourseByIDLookupInput{CourseID: courseID})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "course not found"})
		return
	}

	courseStructure, err := h.Store.GetCourseStructure(stores.CourseStructureInput{CourseID: courseID, IncludeUnpublished: true})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load course structure"})
		return
	}

	moduleCount := len(courseStructure.Modules)
	lessonCount := courseStructure.LessonCount
	c.JSON(http.StatusOK, gin.H{"course": adminCourseDetail(courseModel, moduleCount, lessonCount, courseStructure.Modules)})
}

func (h *AdminCoursesHandler) create(c *gin.Context) {
	request := adminCourseRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	courseModel, err := h.Store.CreateCourse(stores.CourseCreateInput{
		Slug:            request.Slug,
		Title:           request.Title,
		Description:     request.Description,
		ThumbnailURL:    request.ThumbnailURL,
		Metadata:        request.Metadata,
		BodyMarkdown:    request.BodyMarkdown,
		AccessLevel:     request.AccessLevel,
		Status:          request.Status,
		PublishedAt:     request.PublishedAt,
		MetaTitle:       request.MetaTitle,
		MetaDescription: request.MetaDescription,
		MetaImageURL:    request.MetaImageURL,
		CanonicalURL:    request.CanonicalURL,
		NoIndex:         boolValue(request.NoIndex),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create course"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"course": adminCourseDetail(courseModel, 0, 0, []stores.CourseModuleWithLessons{})})
}

func (h *AdminCoursesHandler) update(c *gin.Context) {
	courseID := parseUintDefault(c.Param("id"))
	if courseID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	request := adminCourseRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	metadataInput := request.Metadata
	courseModel, err := h.Store.UpdateCourse(stores.CourseUpdateInput{
		CourseID:        courseID,
		Title:           request.Title,
		Description:     request.Description,
		ThumbnailURL:    request.ThumbnailURL,
		Metadata:        &metadataInput,
		BodyMarkdown:    request.BodyMarkdown,
		AccessLevel:     request.AccessLevel,
		Status:          request.Status,
		PublishedAt:     request.PublishedAt,
		MetaTitle:       request.MetaTitle,
		MetaDescription: request.MetaDescription,
		MetaImageURL:    request.MetaImageURL,
		CanonicalURL:    request.CanonicalURL,
		NoIndex:         request.NoIndex,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update course"})
		return
	}

	courseStructure, err := h.Store.GetCourseStructure(stores.CourseStructureInput{CourseID: courseID, IncludeUnpublished: true})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load updated structure"})
		return
	}

	moduleCount := len(courseStructure.Modules)
	lessonCount := courseStructure.LessonCount
	c.JSON(http.StatusOK, gin.H{"course": adminCourseDetail(courseModel, moduleCount, lessonCount, courseStructure.Modules)})
}

func (h *AdminCoursesHandler) delete(c *gin.Context) {
	courseID := parseUintDefault(c.Param("id"))
	if courseID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	if err := h.Store.DeleteCourse(stores.CourseDeleteInput{CourseID: courseID}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete course"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (h *AdminCoursesHandler) createModule(c *gin.Context) {
	courseID := parseUintDefault(c.Param("id"))
	if courseID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	request := adminCourseModuleRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	modulePosition := intPointerValue(request.Position)
	moduleModel, err := h.Store.CreateCourseModule(stores.CourseModuleCreateInput{
		CourseID: courseID,
		Title:    request.Title,
		Position: modulePosition,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create module"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"module": adminCourseModuleSummary(*moduleModel)})
}

func (h *AdminCoursesHandler) updateModule(c *gin.Context) {
	courseID := parseUintDefault(c.Param("id"))
	moduleID := parseUintDefault(c.Param("module_id"))
	if courseID == 0 || moduleID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid module id"})
		return
	}

	request := adminCourseModuleRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	moduleModel, err := h.Store.UpdateCourseModule(stores.CourseModuleUpdateInput{
		CourseID: courseID,
		ModuleID: moduleID,
		Title:    request.Title,
		Position: request.Position,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update module"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"module": adminCourseModuleSummary(*moduleModel)})
}

func (h *AdminCoursesHandler) deleteModule(c *gin.Context) {
	courseID := parseUintDefault(c.Param("id"))
	moduleID := parseUintDefault(c.Param("module_id"))
	if courseID == 0 || moduleID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid module id"})
		return
	}

	if err := h.Store.DeleteCourseModule(stores.CourseModuleDeleteInput{CourseID: courseID, ModuleID: moduleID}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete module"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (h *AdminCoursesHandler) reorderModules(c *gin.Context) {
	courseID := parseUintDefault(c.Param("id"))
	if courseID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	request := adminCourseReorderModulesRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if err := h.Store.ReorderCourseModules(stores.CourseModuleReorderInput{CourseID: courseID, ModuleIDs: request.ModuleIDs}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reorder modules"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"updated": true})
}

func (h *AdminCoursesHandler) createLesson(c *gin.Context) {
	courseID := parseUintDefault(c.Param("id"))
	moduleID := parseUintDefault(c.Param("module_id"))
	if courseID == 0 || moduleID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid module id"})
		return
	}

	request := adminCourseLessonRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	lessonModel, err := h.Store.CreateCourseLesson(stores.CourseLessonCreateInput{
		CourseID:     courseID,
		ModuleID:     moduleID,
		Title:        request.Title,
		Slug:         request.Slug,
		BodyMarkdown: request.BodyMarkdown,
		VimeoURL:     request.VimeoURL,
		Position:     intPointerValue(request.Position),
		IsFree:       boolValue(request.IsFree),
		Status:       request.Status,
		PublishedAt:  request.PublishedAt,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create lesson"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"lesson": adminCourseLessonSummary(*lessonModel, false)})
}

func (h *AdminCoursesHandler) updateLesson(c *gin.Context) {
	courseID := parseUintDefault(c.Param("id"))
	moduleID := parseUintDefault(c.Param("module_id"))
	lessonID := parseUintDefault(c.Param("lesson_id"))
	if courseID == 0 || moduleID == 0 || lessonID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lesson id"})
		return
	}

	request := adminCourseLessonRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	lessonModel, err := h.Store.UpdateCourseLesson(stores.CourseLessonUpdateInput{
		CourseID:     courseID,
		ModuleID:     moduleID,
		LessonID:     lessonID,
		Title:        request.Title,
		Slug:         request.Slug,
		BodyMarkdown: request.BodyMarkdown,
		VimeoURL:     request.VimeoURL,
		Position:     request.Position,
		IsFree:       request.IsFree,
		Status:       request.Status,
		PublishedAt:  request.PublishedAt,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update lesson"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"lesson": adminCourseLessonSummary(*lessonModel, false)})
}

func (h *AdminCoursesHandler) deleteLesson(c *gin.Context) {
	courseID := parseUintDefault(c.Param("id"))
	moduleID := parseUintDefault(c.Param("module_id"))
	lessonID := parseUintDefault(c.Param("lesson_id"))
	if courseID == 0 || moduleID == 0 || lessonID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid lesson id"})
		return
	}

	if err := h.Store.DeleteCourseLesson(stores.CourseLessonDeleteInput{CourseID: courseID, ModuleID: moduleID, LessonID: lessonID}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete lesson"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (h *AdminCoursesHandler) reorderLessons(c *gin.Context) {
	courseID := parseUintDefault(c.Param("id"))
	moduleID := parseUintDefault(c.Param("module_id"))
	if courseID == 0 || moduleID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid module id"})
		return
	}

	request := adminCourseReorderLessonsRequest{}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	if err := h.Store.ReorderCourseLessons(stores.CourseLessonReorderInput{
		CourseID:  courseID,
		ModuleID:  moduleID,
		LessonIDs: request.LessonIDs,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to reorder lessons"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"updated": true})
}

func adminCourseSummary(courseModel stores.CourseModel, courseCount stores.CourseContentCount) gin.H {
	courseMetadata := stores.ParseCourseMetadata(courseModel.MetadataJSON)
	if courseCount.ModuleCount > 0 {
		courseMetadata.ModuleCount = courseCount.ModuleCount
	}
	if courseCount.LessonCount > 0 {
		courseMetadata.LessonCount = courseCount.LessonCount
	}

	return gin.H{
		"id":            courseModel.ID,
		"slug":          courseModel.Slug,
		"title":         courseModel.Title,
		"description":   resolveCourseTextDescription(courseModel),
		"thumbnail_url": courseModel.ThumbnailURL,
		"metadata":      courseMetadata,
		"access_level":  courseModel.AccessLevel,
		"status":        courseModel.Status,
		"published_at":  courseModel.PublishedAt,
		"module_count":  courseMetadata.ModuleCount,
		"lesson_count":  courseMetadata.LessonCount,
	}
}

func adminCourseDetail(courseModel *stores.CourseModel, moduleCount int, lessonCount int, modules []stores.CourseModuleWithLessons) gin.H {
	if courseModel == nil {
		return gin.H{}
	}

	courseMetadata := stores.ParseCourseMetadata(courseModel.MetadataJSON)
	if moduleCount > 0 {
		courseMetadata.ModuleCount = moduleCount
	}
	if lessonCount > 0 {
		courseMetadata.LessonCount = lessonCount
	}

	moduleItems := make([]gin.H, 0, len(modules))
	for _, moduleWithLessons := range modules {
		lessonItems := make([]gin.H, 0, len(moduleWithLessons.Lessons))
		for _, lessonModel := range moduleWithLessons.Lessons {
			lessonItems = append(lessonItems, adminCourseLessonSummary(lessonModel, false))
		}

		moduleItems = append(moduleItems, gin.H{
			"id":        moduleWithLessons.Module.ID,
			"course_id": moduleWithLessons.Module.CourseID,
			"title":     moduleWithLessons.Module.Title,
			"position":  moduleWithLessons.Module.Position,
			"lessons":   lessonItems,
		})
	}

	return gin.H{
		"id":               courseModel.ID,
		"slug":             courseModel.Slug,
		"title":            courseModel.Title,
		"description":      resolveCourseTextDescription(*courseModel),
		"thumbnail_url":    courseModel.ThumbnailURL,
		"metadata":         courseMetadata,
		"access_level":     courseModel.AccessLevel,
		"status":           courseModel.Status,
		"published_at":     courseModel.PublishedAt,
		"body_markdown":    courseModel.BodyMarkdown,
		"meta_title":       courseModel.MetaTitle,
		"meta_description": courseModel.MetaDescription,
		"meta_image_url":   courseModel.MetaImageURL,
		"canonical_url":    courseModel.CanonicalURL,
		"noindex":          courseModel.NoIndex,
		"modules":          moduleItems,
		"module_count":     courseMetadata.ModuleCount,
		"lesson_count":     courseMetadata.LessonCount,
	}
}

func adminCourseModuleSummary(moduleModel stores.CourseModuleModel) gin.H {
	return gin.H{
		"id":        moduleModel.ID,
		"course_id": moduleModel.CourseID,
		"title":     moduleModel.Title,
		"position":  moduleModel.Position,
	}
}

func adminCourseLessonSummary(lessonModel stores.CourseLessonModel, isLocked bool) gin.H {
	return gin.H{
		"id":            lessonModel.ID,
		"course_id":     lessonModel.CourseID,
		"module_id":     lessonModel.ModuleID,
		"title":         lessonModel.Title,
		"slug":          lessonModel.Slug,
		"body_markdown": lessonModel.BodyMarkdown,
		"vimeo_url":     lessonModel.VimeoURL,
		"position":      lessonModel.Position,
		"is_free":       lessonModel.IsFree,
		"is_locked":     isLocked,
		"status":        lessonModel.Status,
		"published_at":  lessonModel.PublishedAt,
	}
}

func resolveCourseTextDescription(courseModel stores.CourseModel) string {
	description := strings.TrimSpace(courseModel.Description)
	if description != "" {
		return description
	}
	return strings.TrimSpace(courseModel.Excerpt)
}

func intPointerValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}
