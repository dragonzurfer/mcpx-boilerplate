package middleware

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/stores"
	"gorm.io/gorm"
)

const (
	ContextCourseEntitlementKey = "courseEntitlementActive"
	ContextCourseKey            = "currentCourse"
	ContextCourseLessonKey      = "currentCourseLesson"
)

func RequireCourseAuthentication() gin.HandlerFunc {
	return func(c *gin.Context) {
		if _, ok := CurrentUser(c); !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
		c.Next()
	}
}

func CourseEntitlement(store *stores.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		userModel, ok := CurrentUser(c)
		if !ok || userModel == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		entitlementActive, err := lookupEntitlementStatus(store, userModel.ID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to check entitlement"})
			return
		}

		c.Set(ContextCourseEntitlementKey, entitlementActive)
		c.Next()
	}
}

func lookupEntitlementStatus(store *stores.Store, userID uint) (bool, error) {
	_, err := store.GetActiveEntitlement(stores.EntitlementLookupInput{UserID: userID, Now: time.Now().UTC()})
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return false, err
}

func LessonAccess(store *stores.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		courseModel, err := resolveCourseFromRequest(store, c)
		if err != nil {
			handleCourseLookupError(c, err)
			return
		}

		lessonSlug := strings.TrimSpace(c.Param("lessonSlug"))
		if lessonSlug == "" {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "lesson slug required"})
			return
		}

		lessonModel, err := store.GetCourseLessonBySlug(stores.CourseLessonSlugLookupInput{
			CourseID:   courseModel.ID,
			LessonSlug: lessonSlug,
		})
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "lesson not found"})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to load lesson"})
			return
		}

		if strings.ToUpper(strings.TrimSpace(lessonModel.Status)) != stores.CourseStatusPublished {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "lesson not found"})
			return
		}

		entitlementActive := CurrentCourseEntitlement(c)
		isLessonLocked := !lessonModel.IsFree && !entitlementActive
		if isLessonLocked {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{
				"lesson": gin.H{
					"id":        lessonModel.ID,
					"slug":      lessonModel.Slug,
					"title":     lessonModel.Title,
					"is_free":   lessonModel.IsFree,
					"is_locked": true,
				},
				"is_locked": true,
				"gate": gin.H{
					"type": "PAYWALL",
				},
			})
			return
		}

		c.Set(ContextCourseKey, courseModel)
		c.Set(ContextCourseLessonKey, lessonModel)
		c.Next()
	}
}

func resolveCourseFromRequest(store *stores.Store, c *gin.Context) (*stores.CourseModel, error) {
	courseSlug := strings.TrimSpace(c.Param("slug"))
	if courseSlug == "" {
		return nil, gorm.ErrRecordNotFound
	}

	courseModel, err := store.GetCourseBySlug(stores.CourseLookupInput{Slug: courseSlug})
	if err != nil {
		return nil, err
	}

	if strings.ToUpper(strings.TrimSpace(courseModel.Status)) != stores.CourseStatusPublished {
		return nil, gorm.ErrRecordNotFound
	}

	return courseModel, nil
}

func handleCourseLookupError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "course not found"})
		return
	}
	c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to load course"})
}

func CurrentCourseEntitlement(c *gin.Context) bool {
	value, exists := c.Get(ContextCourseEntitlementKey)
	if !exists {
		return false
	}

	entitlementActive, isBool := value.(bool)
	if !isBool {
		return false
	}

	return entitlementActive
}

func CurrentCourse(c *gin.Context) (*stores.CourseModel, bool) {
	value, exists := c.Get(ContextCourseKey)
	if !exists {
		return nil, false
	}

	courseModel, ok := value.(*stores.CourseModel)
	return courseModel, ok
}

func CurrentCourseLesson(c *gin.Context) (*stores.CourseLessonModel, bool) {
	value, exists := c.Get(ContextCourseLessonKey)
	if !exists {
		return nil, false
	}

	lessonModel, ok := value.(*stores.CourseLessonModel)
	return lessonModel, ok
}
