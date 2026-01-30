package routes

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/middleware"
	"github.com/mcpx/boilerplate/services"
	"github.com/mcpx/boilerplate/stores"
	"gorm.io/gorm"
)

type PostsHandler struct {
	Service *services.ContentService
}

func (h *PostsHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/posts", h.listPosts)
	rg.GET("/posts/:slug", h.getPost)
}

func (h *PostsHandler) listPosts(c *gin.Context) {
	accessParam := strings.TrimSpace(c.Query("access_level"))
	accessLevels := resolveAccessLevels(accessParam)

	page := parseIntDefault(c.Query("page"), 0)
	pageSize := parseIntDefault(c.Query("page_size"), 20)
	status := stores.PostStatusPublished
	query := strings.TrimSpace(c.Query("q"))
	tagID := parseUintDefault(c.Query("tag_id"))

	output, err := h.Service.ListPosts(stores.PostListInput{
		AccessLevels: accessLevels,
		TagID:        tagID,
		Query:        query,
		Status:       status,
		Page:         page,
		PageSize:     pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load posts"})
		return
	}

	items := make([]gin.H, 0, len(output.Posts))
	for _, post := range output.Posts {
		items = append(items, postListItem(post))
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": output.Total,
		"page":  page,
	})
}

func (h *PostsHandler) getPost(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}

	viewer, _ := middleware.CurrentUser(c)
	output, err := h.Service.GetPostBySlug(services.PostAccessInput{
		Slug:         slug,
		Viewer:       viewer,
		Now:          time.Now().UTC(),
		UseHTMLCache: true,
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load post"})
		return
	}

	payload := gin.H{
		"post":         postDetail(output.Post, output.Tags),
		"access_level": output.AccessLevel,
		"is_locked":    output.IsLocked,
		"gate":         gatePayload(output),
		"entitlement_active": output.EntitlementActive,
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

func postListItem(post stores.PostModel) gin.H {
	return gin.H{
		"id":           post.ID,
		"slug":         post.Slug,
		"title":        post.Title,
		"excerpt":      post.Excerpt,
		"access_level": post.AccessLevel,
		"status":       post.Status,
		"published_at": post.PublishedAt,
	}
}

func postDetail(post *stores.PostModel, tags []stores.TagModel) gin.H {
	if post == nil {
		return gin.H{}
	}
	return gin.H{
		"id":           post.ID,
		"slug":         post.Slug,
		"title":        post.Title,
		"excerpt":      post.Excerpt,
		"access_level": post.AccessLevel,
		"status":       post.Status,
		"published_at": post.PublishedAt,
		"tags":         tags,
	}
}

func gatePayload(output services.PostAccessOutput) gin.H {
	if !output.IsLocked {
		return nil
	}
	return gin.H{
		"type": output.GateType,
	}
}

func resolveAccessLevels(input string) []string {
	if input == "" {
		return []string{stores.AccessLevelPublic}
	}
	parts := strings.Split(input, ",")
	levels := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		levels = append(levels, strings.ToUpper(value))
	}
	if len(levels) == 0 {
		return []string{stores.AccessLevelPublic}
	}
	return levels
}

func parseIntDefault(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseUintDefault(value string) uint {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0
	}
	return uint(parsed)
}
