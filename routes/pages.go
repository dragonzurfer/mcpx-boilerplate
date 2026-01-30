package routes

import (
	"html/template"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/services"
	"github.com/mcpx/boilerplate/stores"
)

type PageHandler struct {
	Store     *stores.Store
	Content   *services.ContentService
	Templates map[string]*template.Template
}

func LoadTemplates() (map[string]*template.Template, error) {
	layout := filepath.Join("web", "templates", "layout.html")
	nav := filepath.Join("web", "templates", "partials", "nav.html")
	adminNav := filepath.Join("web", "templates", "partials", "admin_nav.html")
	pages := map[string]string{
		"home":                 filepath.Join("web", "templates", "home.html"),
		"post":                 filepath.Join("web", "templates", "post.html"),
		"pricing":              filepath.Join("web", "templates", "pricing.html"),
		"courses":              filepath.Join("web", "templates", "courses.html"),
		"course":               filepath.Join("web", "templates", "course.html"),
		"account":              filepath.Join("web", "templates", "account.html"),
		"admin_dashboard":      filepath.Join("web", "templates", "admin_dashboard.html"),
		"admin_posts":          filepath.Join("web", "templates", "admin_posts.html"),
		"admin_post_analytics": filepath.Join("web", "templates", "admin_post_analytics.html"),
		"admin_funnel":         filepath.Join("web", "templates", "admin_funnel.html"),
		"admin_promos":         filepath.Join("web", "templates", "admin_promos.html"),
		"admin_users":          filepath.Join("web", "templates", "admin_users.html"),
		"admin_settings":       filepath.Join("web", "templates", "admin_settings.html"),
		"admin_analytics":      filepath.Join("web", "templates", "admin_analytics.html"),
	}

	templates := map[string]*template.Template{}
	for name, page := range pages {
		tmpl, err := template.ParseFiles(layout, nav, adminNav, page)
		if err != nil {
			return nil, err
		}
		templates[name] = tmpl
	}
	return templates, nil
}

func (h *PageHandler) Register(r *gin.Engine) {
	r.GET("/", h.home)
	r.GET("/post/:slug", h.post)
	r.GET("/pricing", h.pricing)
	r.GET("/courses", h.courses)
	r.GET("/course/:slug", h.course)
	r.GET("/account", h.account)
	r.GET("/admin", h.adminDashboard)
	r.GET("/admin/posts", h.adminPosts)
	r.GET("/admin/posts/:id/analytics", h.adminPostAnalytics)
	r.GET("/admin/funnel", h.adminFunnel)
	r.GET("/admin/promos", h.adminPromos)
	r.GET("/admin/users", h.adminUsers)
	r.GET("/admin/settings", h.adminSettings)
	r.GET("/admin/analytics", h.adminAnalytics)
}

type pageData struct {
	Meta        services.MetaOutput
	Post        *stores.PostModel
	Course      *stores.CourseModel
	Tags        []stores.TagModel
	ContentHTML template.HTML
	JSONLD      template.JS
	Theme       ThemeData
}

type ThemeData struct {
	PrimaryColor string
	OffWhite     string
	SiteURL      string
}

const defaultOffWhite = "#f5f1e8"

func (h *PageHandler) home(c *gin.Context) {
	settings := h.resolveSiteSettings()
	meta := services.BuildMeta(services.MetaBuildInput{PageType: services.PageTypeHome, Site: settings})
	data := pageData{Meta: meta, JSONLD: template.JS(meta.JSONLD), Theme: resolveTheme(settings)}
	h.renderTemplate(c, "home", data)
}

func (h *PageHandler) pricing(c *gin.Context) {
	settings := h.resolveSiteSettings()
	meta := services.BuildMeta(services.MetaBuildInput{PageType: services.PageTypeHome, Site: settings})
	data := pageData{Meta: meta, JSONLD: template.JS(meta.JSONLD), Theme: resolveTheme(settings)}
	h.renderTemplate(c, "pricing", data)
}

func (h *PageHandler) adminAnalytics(c *gin.Context) {
	settings := h.resolveSiteSettings()
	meta := services.BuildMeta(services.MetaBuildInput{PageType: services.PageTypeHome, Site: settings})
	meta.Title = buildAdminTitle(meta.SiteName)
	meta.Description = "Admin analytics dashboard for funnel stages and promo performance."
	meta.Robots = "noindex,nofollow"
	if settings.SiteURL != "" {
		meta.CanonicalURL = settings.SiteURL + "/admin/analytics"
	}

	data := pageData{Meta: meta, JSONLD: template.JS(meta.JSONLD), Theme: resolveTheme(settings)}
	h.renderTemplate(c, "admin_analytics", data)
}

func (h *PageHandler) courses(c *gin.Context) {
	settings := h.resolveSiteSettings()
	meta := services.BuildMeta(services.MetaBuildInput{PageType: services.PageTypeHome, Site: settings})
	data := pageData{Meta: meta, JSONLD: template.JS(meta.JSONLD), Theme: resolveTheme(settings)}
	h.renderTemplate(c, "courses", data)
}

func (h *PageHandler) course(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}

	output, err := h.Content.GetCourseBySlug(services.CourseAccessInput{
		Slug:         slug,
		Viewer:       nil,
		Now:          time.Now().UTC(),
		UseHTMLCache: true,
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "course not found"})
		return
	}

	settings := h.resolveSiteSettings()
	meta := services.BuildMeta(services.MetaBuildInput{PageType: services.PageTypeCourse, Course: output.Course, Site: settings})
	content := template.HTML(output.TeaserHTML)
	if !output.IsLocked {
		content = template.HTML(output.HTML)
	}

	data := pageData{Meta: meta, Course: output.Course, ContentHTML: content}
	data.JSONLD = template.JS(meta.JSONLD)
	data.Theme = resolveTheme(settings)
	h.renderTemplate(c, "course", data)
}

func (h *PageHandler) post(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}

	output, err := h.Content.GetPostBySlug(services.PostAccessInput{
		Slug:         slug,
		Viewer:       nil,
		Now:          time.Now().UTC(),
		UseHTMLCache: true,
	})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
		return
	}

	settings := h.resolveSiteSettings()
	topicTags := filterTagsByType(output.Tags, "TOPIC")
	meta := services.BuildMeta(services.MetaBuildInput{PageType: services.PageTypePost, Post: output.Post, Site: settings, TopicTags: topicTags})

	content := template.HTML(output.TeaserHTML)
	if !output.IsLocked {
		content = template.HTML(output.HTML)
	}

	data := pageData{
		Meta:        meta,
		Post:        output.Post,
		Tags:        output.Tags,
		ContentHTML: content,
	}
	data.JSONLD = template.JS(meta.JSONLD)
	data.Theme = resolveTheme(settings)
	h.renderTemplate(c, "post", data)
}

func (h *PageHandler) account(c *gin.Context) {
	settings := h.resolveSiteSettings()
	meta := services.BuildMeta(services.MetaBuildInput{PageType: services.PageTypeHome, Site: settings})
	meta.Title = "Account | " + meta.SiteName
	meta.Robots = "noindex,nofollow"
	data := pageData{Meta: meta, JSONLD: template.JS(meta.JSONLD), Theme: resolveTheme(settings)}
	h.renderTemplate(c, "account", data)
}

func (h *PageHandler) adminDashboard(c *gin.Context) {
	settings := h.resolveSiteSettings()
	meta := services.BuildMeta(services.MetaBuildInput{PageType: services.PageTypeHome, Site: settings})
	meta.Title = buildAdminTitle(meta.SiteName)
	meta.Description = "Admin workspace for managing content and growth settings."
	meta.Robots = "noindex,nofollow"
	data := pageData{Meta: meta, JSONLD: template.JS(meta.JSONLD), Theme: resolveTheme(settings)}
	h.renderTemplate(c, "admin_dashboard", data)
}

func (h *PageHandler) adminPosts(c *gin.Context) {
	settings := h.resolveSiteSettings()
	meta := services.BuildMeta(services.MetaBuildInput{PageType: services.PageTypeHome, Site: settings})
	meta.Title = "Admin Posts | " + meta.SiteName
	meta.Robots = "noindex,nofollow"
	data := pageData{Meta: meta, JSONLD: template.JS(meta.JSONLD), Theme: resolveTheme(settings)}
	h.renderTemplate(c, "admin_posts", data)
}

func (h *PageHandler) adminPostAnalytics(c *gin.Context) {
	settings := h.resolveSiteSettings()
	meta := services.BuildMeta(services.MetaBuildInput{PageType: services.PageTypeHome, Site: settings})
	meta.Title = "Post Analytics | " + meta.SiteName
	meta.Robots = "noindex,nofollow"

	postID, err := strconv.ParseUint(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil || postID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid post id"})
		return
	}

	post := stores.PostModel{}
	_ = h.Store.DB().First(&post, postID).Error
	if post.ID != 0 && post.Title != "" {
		meta.Title = post.Title + " · Analytics | " + meta.SiteName
	}

	data := pageData{Meta: meta, JSONLD: template.JS(meta.JSONLD), Theme: resolveTheme(settings), Post: &post}
	h.renderTemplate(c, "admin_post_analytics", data)
}

func (h *PageHandler) adminFunnel(c *gin.Context) {
	settings := h.resolveSiteSettings()
	meta := services.BuildMeta(services.MetaBuildInput{PageType: services.PageTypeHome, Site: settings})
	meta.Title = "Admin Funnel | " + meta.SiteName
	meta.Robots = "noindex,nofollow"
	data := pageData{Meta: meta, JSONLD: template.JS(meta.JSONLD), Theme: resolveTheme(settings)}
	h.renderTemplate(c, "admin_funnel", data)
}

func (h *PageHandler) adminPromos(c *gin.Context) {
	settings := h.resolveSiteSettings()
	meta := services.BuildMeta(services.MetaBuildInput{PageType: services.PageTypeHome, Site: settings})
	meta.Title = "Admin Promos | " + meta.SiteName
	meta.Robots = "noindex,nofollow"
	data := pageData{Meta: meta, JSONLD: template.JS(meta.JSONLD), Theme: resolveTheme(settings)}
	h.renderTemplate(c, "admin_promos", data)
}

func (h *PageHandler) adminUsers(c *gin.Context) {
	settings := h.resolveSiteSettings()
	meta := services.BuildMeta(services.MetaBuildInput{PageType: services.PageTypeHome, Site: settings})
	meta.Title = "Admin Users | " + meta.SiteName
	meta.Robots = "noindex,nofollow"
	data := pageData{Meta: meta, JSONLD: template.JS(meta.JSONLD), Theme: resolveTheme(settings)}
	h.renderTemplate(c, "admin_users", data)
}

func (h *PageHandler) adminSettings(c *gin.Context) {
	settings := h.resolveSiteSettings()
	meta := services.BuildMeta(services.MetaBuildInput{PageType: services.PageTypeHome, Site: settings})
	meta.Title = "Admin Settings | " + meta.SiteName
	meta.Robots = "noindex,nofollow"
	data := pageData{Meta: meta, JSONLD: template.JS(meta.JSONLD), Theme: resolveTheme(settings)}
	h.renderTemplate(c, "admin_settings", data)
}

func (h *PageHandler) resolveSiteSettings() services.SiteSettings {
	defaults := stores.SiteSettingsInput{SiteName: defaultSiteName, SiteURL: defaultSiteURL, PrimaryColor: defaultPrimaryColor}
	settings, _ := h.Store.EnsureSiteSettings(defaults)
	if settings == nil {
		return services.SiteSettings{SiteName: defaultSiteName, SiteURL: defaultSiteURL, PrimaryColor: defaultPrimaryColor}
	}
	return services.SiteSettings{
		SiteName:           resolveSiteName(settings),
		SiteURL:            resolveSiteURL(settings),
		DefaultOgImageURL:  settings.DefaultOgImageURL,
		PrimaryColor:       settings.PrimaryColor,
		TwitterSite:        settings.TwitterSite,
		TwitterCreator:     settings.TwitterCreator,
		GoogleVerification: settings.GoogleVerification,
		BingVerification:   settings.BingVerification,
		PinterestVerify:    settings.PinterestVerification,
	}
}

func filterTagsByType(tags []stores.TagModel, tagType string) []string {
	out := []string{}
	for _, tag := range tags {
		if strings.EqualFold(tag.TagType, tagType) {
			out = append(out, tag.Name)
		}
	}
	return out
}

func (h *PageHandler) renderTemplate(c *gin.Context, name string, data pageData) {
	tmpl, ok := h.Templates[name]
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "template missing"})
		return
	}
	if err := tmpl.ExecuteTemplate(c.Writer, "layout", data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to render"})
	}
}

func resolveTheme(settings services.SiteSettings) ThemeData {
	primary := strings.TrimSpace(settings.PrimaryColor)
	if primary == "" {
		primary = defaultPrimaryColor
	}
	return ThemeData{
		PrimaryColor: primary,
		OffWhite:     defaultOffWhite,
		SiteURL:      settings.SiteURL,
	}
}

func buildAdminTitle(siteName string) string {
	trimmed := strings.TrimSpace(siteName)
	if trimmed == "" {
		return "Admin Analytics"
	}
	return "Admin Analytics | " + trimmed
}
