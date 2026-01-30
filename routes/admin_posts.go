package routes

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/middleware"
	"github.com/mcpx/boilerplate/stores"
)

type AdminPostsHandler struct {
	Store *stores.Store
}

type adminPostRequest struct {
	Slug            string     `json:"slug"`
	Title           string     `json:"title"`
	Excerpt         string     `json:"excerpt"`
	BodyMarkdown    string     `json:"body_markdown"`
	AccessLevel     string     `json:"access_level"`
	Status          string     `json:"status"`
	PublishedAt     *time.Time `json:"published_at"`
	TopicTags       []string   `json:"topic_tags"`
	InterestTags    []string   `json:"interest_tags"`
	MetaTitle       string     `json:"meta_title"`
	MetaDescription string     `json:"meta_description"`
	MetaImageURL    string     `json:"meta_image_url"`
	CanonicalURL    string     `json:"canonical_url"`
	NoIndex         *bool      `json:"noindex"`
}

func (h *AdminPostsHandler) Register(rg *gin.RouterGroup) {
	rg.GET("/posts", h.list)
	rg.GET("/posts/:id", h.get)
	rg.POST("/posts", h.create)
	rg.PUT("/posts/:id", h.update)
}

func (h *AdminPostsHandler) list(c *gin.Context) {
	status := strings.TrimSpace(c.Query("status"))
	query := strings.TrimSpace(c.Query("q"))
	page := parseIntDefault(c.Query("page"), 0)
	pageSize := parseIntDefault(c.Query("page_size"), 20)

	output, err := h.Store.ListPosts(stores.PostListInput{
		Status:   status,
		Query:    query,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load posts"})
		return
	}

	items := make([]gin.H, 0, len(output.Posts))
	for _, post := range output.Posts {
		items = append(items, postListItem(post))
	}

	c.JSON(http.StatusOK, gin.H{"items": items, "total": output.Total})
}

func (h *AdminPostsHandler) get(c *gin.Context) {
	id := parseUintDefault(c.Param("id"))
	if id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	post, tags, err := h.Store.GetPostByID(stores.PostIDLookupInput{PostID: id, IncludeTags: true})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"post": adminPostDetail(post, tags)})
}

func (h *AdminPostsHandler) create(c *gin.Context) {
	var req adminPostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	adminUser, _ := middleware.CurrentUser(c)
	createdBy := uint(0)
	if adminUser != nil {
		createdBy = adminUser.ID
	}

	tagIDs, err := h.resolveTagIDs(req.TopicTags, req.InterestTags)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to resolve tags"})
		return
	}

	post, err := h.Store.CreatePost(stores.PostCreateInput{
		Slug:            req.Slug,
		Title:           req.Title,
		Excerpt:         req.Excerpt,
		BodyMarkdown:    req.BodyMarkdown,
		AccessLevel:     strings.ToUpper(req.AccessLevel),
		Status:          strings.ToUpper(req.Status),
		PublishedAt:     req.PublishedAt,
		CreatedBy:       createdBy,
		MetaTitle:       req.MetaTitle,
		MetaDescription: req.MetaDescription,
		MetaImageURL:    req.MetaImageURL,
		CanonicalURL:    req.CanonicalURL,
		NoIndex:         boolValue(req.NoIndex),
		TagIDs:          tagIDs,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create post"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"post": postListItem(*post)})
}

func (h *AdminPostsHandler) update(c *gin.Context) {
	idRaw := strings.TrimSpace(c.Param("id"))
	id64, err := strconv.ParseUint(idRaw, 10, 64)
	if err != nil || id64 == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var req adminPostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	tagIDs, err := h.resolveTagIDs(req.TopicTags, req.InterestTags)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to resolve tags"})
		return
	}

	post, err := h.Store.UpdatePost(stores.PostUpdateInput{
		PostID:          uint(id64),
		Title:           req.Title,
		Excerpt:         req.Excerpt,
		BodyMarkdown:    req.BodyMarkdown,
		AccessLevel:     strings.ToUpper(req.AccessLevel),
		Status:          strings.ToUpper(req.Status),
		PublishedAt:     req.PublishedAt,
		MetaTitle:       req.MetaTitle,
		MetaDescription: req.MetaDescription,
		MetaImageURL:    req.MetaImageURL,
		CanonicalURL:    req.CanonicalURL,
		NoIndex:         req.NoIndex,
		TagIDs:          tagIDs,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update post"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"post": postListItem(*post)})
}

func (h *AdminPostsHandler) resolveTagIDs(topicTags, interestTags []string) ([]uint, error) {
	tagIDs := []uint{}
	seen := map[uint]bool{}

	for _, tag := range topicTags {
		model, err := h.Store.GetOrCreateTag(tag, "TOPIC")
		if err != nil {
			return nil, err
		}
		if !seen[model.ID] {
			tagIDs = append(tagIDs, model.ID)
			seen[model.ID] = true
		}
	}

	for _, tag := range interestTags {
		model, err := h.Store.GetOrCreateTag(tag, "INTEREST")
		if err != nil {
			return nil, err
		}
		if !seen[model.ID] {
			tagIDs = append(tagIDs, model.ID)
			seen[model.ID] = true
		}
	}

	return tagIDs, nil
}

func boolValue(ptr *bool) bool {
	if ptr == nil {
		return false
	}
	return *ptr
}

func adminPostDetail(post *stores.PostModel, tags []stores.TagModel) gin.H {
	if post == nil {
		return gin.H{}
	}
	return gin.H{
		"id":               post.ID,
		"slug":             post.Slug,
		"title":            post.Title,
		"excerpt":          post.Excerpt,
		"body_markdown":    post.BodyMarkdown,
		"access_level":     post.AccessLevel,
		"status":           post.Status,
		"published_at":     post.PublishedAt,
		"topic_tags":       filterTagNames(tags, "TOPIC"),
		"interest_tags":    filterTagNames(tags, "INTEREST"),
		"meta_title":       post.MetaTitle,
		"meta_description": post.MetaDescription,
		"meta_image_url":   post.MetaImageURL,
		"canonical_url":    post.CanonicalURL,
		"noindex":          post.NoIndex,
	}
}

func filterTagNames(tags []stores.TagModel, tagType string) []string {
	out := []string{}
	for _, tag := range tags {
		if strings.EqualFold(tag.TagType, tagType) {
			out = append(out, tag.Name)
		}
	}
	return out
}
