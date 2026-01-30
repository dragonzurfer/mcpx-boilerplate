package routes

import (
	"errors"
	"net/http"
	"strings"
	"time"

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
	rg.GET("/courses/:slug", h.get)
}

func (h *CoursesHandler) list(c *gin.Context) {
	accessParam := strings.TrimSpace(c.Query("access_level"))
	accessLevels := resolveAccessLevels(accessParam)
	page := parseIntDefault(c.Query("page"), 0)
	pageSize := parseIntDefault(c.Query("page_size"), 20)
	status := stores.CourseStatusPublished
	query := strings.TrimSpace(c.Query("q"))

	output, err := h.Service.ListCourses(stores.CourseListInput{
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

	items := make([]gin.H, 0, len(output.Courses))
	for _, course := range output.Courses {
		items = append(items, gin.H{
			"id":           course.ID,
			"slug":         course.Slug,
			"title":        course.Title,
			"excerpt":      course.Excerpt,
			"access_level": course.AccessLevel,
			"status":       course.Status,
			"published_at": course.PublishedAt,
		})
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "total": output.Total})
}

func (h *CoursesHandler) get(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}

	viewer, _ := middleware.CurrentUser(c)
	output, err := h.Service.GetCourseBySlug(services.CourseAccessInput{
		Slug:         slug,
		Viewer:       viewer,
		Now:          time.Now().UTC(),
		UseHTMLCache: true,
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "course not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load course"})
		return
	}

	payload := gin.H{
		"course": gin.H{
			"id":           output.Course.ID,
			"slug":         output.Course.Slug,
			"title":        output.Course.Title,
			"excerpt":      output.Course.Excerpt,
			"access_level": output.Course.AccessLevel,
			"status":       output.Course.Status,
			"published_at": output.Course.PublishedAt,
		},
		"access_level": output.AccessLevel,
		"is_locked":    output.IsLocked,
		"gate":         gatePayload(services.PostAccessOutput{IsLocked: output.IsLocked, GateType: output.GateType}),
	}

	if output.IsLocked {
		payload["html"] = output.TeaserHTML
		payload["teaser_html"] = output.TeaserHTML
	} else {
		payload["html"] = output.HTML
		payload["teaser_html"] = output.TeaserHTML
	}

	c.JSON(http.StatusOK, payload)
}
