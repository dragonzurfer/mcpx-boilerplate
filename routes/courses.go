package routes

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/middleware"
	"github.com/mcpx/boilerplate/services"
	"github.com/mcpx/boilerplate/stores"
	"gorm.io/gorm"
)

type CoursesHandler struct {
	Service *services.ContentService
}

func (h *CoursesHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/courses", h.list)

	rg.GET(
		"/courses/:slug",
		middleware.RequireCourseAuthentication(),
		middleware.CourseEntitlement(h.Service.Store),
		h.get,
	)

	rg.GET(
		"/courses/:slug/lessons/:lessonSlug",
		middleware.RequireCourseAuthentication(),
		middleware.CourseEntitlement(h.Service.Store),
		middleware.LessonAccess(h.Service.Store),
		h.getLesson,
	)
}

func (h *CoursesHandler) list(c *gin.Context) {
	accessParam := strings.TrimSpace(c.Query("access_level"))
	accessLevels := resolveAccessLevels(accessParam)
	page := parseIntDefault(c.Query("page"), 0)
	pageSize := parseIntDefault(c.Query("page_size"), 20)
	status := stores.CourseStatusPublished
	query := strings.TrimSpace(c.Query("q"))

	courseListOutput, err := h.Service.ListCourses(stores.CourseListInput{
		AccessLevels: accessLevels,
		Status:       status,
		Query:        query,
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load courses"})
		return
	}

	courseIDs := collectCourseIDs(courseListOutput.Courses)
	courseCounts, err := h.Service.Store.GetCourseContentCounts(stores.CourseContentCountsInput{
		CourseIDs:          courseIDs,
		IncludeUnpublished: false,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load course counts"})
		return
	}

	items := make([]gin.H, 0, len(courseListOutput.Courses))
	for _, courseModel := range courseListOutput.Courses {
		items = append(items, publicCourseSummary(courseModel, courseCounts[courseModel.ID]))
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "total": courseListOutput.Total})
}

func (h *CoursesHandler) get(c *gin.Context) {
	courseSlug := strings.TrimSpace(c.Param("slug"))
	if courseSlug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}

	courseModel, err := h.Service.Store.GetCourseBySlug(stores.CourseLookupInput{Slug: courseSlug})
	if err != nil {
		handleCourseLoadError(c, err)
		return
	}

	if strings.ToUpper(strings.TrimSpace(courseModel.Status)) != stores.CourseStatusPublished {
		c.JSON(http.StatusNotFound, gin.H{"error": "course not found"})
		return
	}

	courseStructure, err := h.Service.Store.GetCourseStructure(stores.CourseStructureInput{
		CourseID:           courseModel.ID,
		IncludeUnpublished: false,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load course structure"})
		return
	}

	entitlementActive := middleware.CurrentCourseEntitlement(c)
	moduleItems, firstLessonSlug := buildPublicCourseModules(courseStructure.Modules, entitlementActive)

	courseMetadata := stores.ParseCourseMetadata(courseModel.MetadataJSON)
	courseMetadata.ModuleCount = len(moduleItems)
	courseMetadata.LessonCount = courseStructure.LessonCount

	c.JSON(http.StatusOK, gin.H{
		"course": gin.H{
			"id":                   courseModel.ID,
			"slug":                 courseModel.Slug,
			"title":                courseModel.Title,
			"description":          resolveCourseTextDescription(*courseModel),
			"thumbnail_url":        courseModel.ThumbnailURL,
			"metadata":             courseMetadata,
			"modules":              moduleItems,
			"module_count":         courseMetadata.ModuleCount,
			"lesson_count":         courseMetadata.LessonCount,
			"selected_lesson_slug": firstLessonSlug,
		},
		"entitlement_active": entitlementActive,
	})
}

func (h *CoursesHandler) getLesson(c *gin.Context) {
	lessonModel, ok := middleware.CurrentCourseLesson(c)
	if !ok || lessonModel == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "lesson not found"})
		return
	}

	htmlOutput, err := renderCourseLessonHTML(h.Service.Store, lessonModel)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to render lesson"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"lesson":             adminCourseLessonSummary(*lessonModel, false),
		"html":               htmlOutput,
		"is_locked":          false,
		"entitlement_active": middleware.CurrentCourseEntitlement(c),
	})
}

func handleCourseLoadError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "course not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load course"})
}

func publicCourseSummary(courseModel stores.CourseModel, courseCount stores.CourseContentCount) gin.H {
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
		"tags":          courseMetadata.Tags,
		"difficulty":    courseMetadata.Difficulty,
		"module_count":  courseMetadata.ModuleCount,
		"lesson_count":  courseMetadata.LessonCount,
	}
}

func buildPublicCourseModules(modules []stores.CourseModuleWithLessons, entitlementActive bool) ([]gin.H, string) {
	moduleItems := make([]gin.H, 0, len(modules))
	selectedLessonSlug := ""
	for _, moduleWithLessons := range modules {
		lessonItems := make([]gin.H, 0, len(moduleWithLessons.Lessons))
		for _, lessonModel := range moduleWithLessons.Lessons {
			isLocked := !lessonModel.IsFree && !entitlementActive
			if selectedLessonSlug == "" && !isLocked {
				selectedLessonSlug = lessonModel.Slug
			}
			lessonItems = append(lessonItems, adminCourseLessonSummary(lessonModel, isLocked))
		}

		moduleItems = append(moduleItems, gin.H{
			"id":       moduleWithLessons.Module.ID,
			"title":    moduleWithLessons.Module.Title,
			"position": moduleWithLessons.Module.Position,
			"lessons":  lessonItems,
		})
	}

	return moduleItems, selectedLessonSlug
}

func renderCourseLessonHTML(store *stores.Store, lessonModel *stores.CourseLessonModel) (string, error) {
	if lessonModel == nil {
		return "", gorm.ErrRecordNotFound
	}

	cachedHTML := strings.TrimSpace(lessonModel.BodyHTMLCache)
	if cachedHTML != "" {
		return cachedHTML, nil
	}

	markdownOutput, err := services.RenderMarkdown(services.MarkdownRenderInput{Markdown: lessonMarkdownForRender(*lessonModel)})
	if err != nil {
		return "", err
	}

	renderedHTML := markdownOutput.HTML
	_ = store.UpdateCourseLessonHTMLCache(stores.CourseLessonHTMLCacheInput{LessonID: lessonModel.ID, HTML: renderedHTML})
	return renderedHTML, nil
}

func lessonMarkdownForRender(lessonModel stores.CourseLessonModel) string {
	lessonMarkdown := strings.TrimSpace(lessonModel.BodyMarkdown)
	vimeoURL := strings.TrimSpace(lessonModel.VimeoURL)
	if vimeoURL == "" {
		return lessonMarkdown
	}
	if strings.Contains(lessonMarkdown, vimeoURL) {
		return lessonMarkdown
	}
	if lessonMarkdown == "" {
		return vimeoURL
	}
	return lessonMarkdown + "\n\n" + vimeoURL
}
