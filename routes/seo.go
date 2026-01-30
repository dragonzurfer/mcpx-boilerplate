package routes

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/stores"
)

type SEOHandler struct {
	Store *stores.Store
}

func (h *SEOHandler) Register(r *gin.Engine) {
	r.GET("/robots.txt", h.robots)
	r.GET("/sitemap.xml", h.sitemap)
	r.GET("/rss.xml", h.rss)
	r.GET("/og/post/:slug", h.ogPost)
	r.GET("/og/course/:slug", h.ogCourse)
	r.GET("/og/home", h.ogHome)
}

func (h *SEOHandler) robots(c *gin.Context) {
	siteURL := resolveSiteURLFromStore(h.Store)
	lines := []string{
		"User-agent: *",
		"Allow: /",
		"Disallow: /admin",
		"Disallow: /api",
	}
	if siteURL != "" {
		lines = append(lines, fmt.Sprintf("Sitemap: %s/sitemap.xml", siteURL))
	}
	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(strings.Join(lines, "\n")))
}

func (h *SEOHandler) sitemap(c *gin.Context) {
	siteURL := resolveSiteURLFromStore(h.Store)
	posts, err := h.Store.ListPublishedPosts(stores.PublishedPostsInput{AccessLevel: stores.AccessLevelPublic})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build sitemap"})
		return
	}
	courses, err := h.Store.ListCourses(stores.CourseListInput{AccessLevels: []string{stores.AccessLevelPublic}, Status: stores.CourseStatusPublished, Page: 0, PageSize: 1000})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build sitemap"})
		return
	}

	urls := []sitemapURL{}
	for _, post := range posts {
		if post.Slug == "" {
			continue
		}
		loc := fmt.Sprintf("%s/post/%s", siteURL, post.Slug)
		urls = append(urls, sitemapURL{Loc: loc, LastMod: formatSitemapTime(post.UpdatedAt)})
	}
	for _, course := range courses.Courses {
		if course.Slug == "" {
			continue
		}
		loc := fmt.Sprintf("%s/course/%s", siteURL, course.Slug)
		urls = append(urls, sitemapURL{Loc: loc, LastMod: formatSitemapTime(course.UpdatedAt)})
	}

	payload := sitemapURLSet{Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: urls}
	data, _ := xml.MarshalIndent(payload, "", "  ")
	c.Data(http.StatusOK, "application/xml; charset=utf-8", append([]byte(xml.Header), data...))
}

func (h *SEOHandler) rss(c *gin.Context) {
	siteURL := resolveSiteURLFromStore(h.Store)
	siteName := resolveSiteNameFromStore(h.Store)
	posts, err := h.Store.ListPublishedPosts(stores.PublishedPostsInput{AccessLevel: stores.AccessLevelPublic})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to build rss"})
		return
	}

	items := make([]rssItem, 0, len(posts))
	for _, post := range posts {
		if post.Slug == "" {
			continue
		}
		link := fmt.Sprintf("%s/post/%s", siteURL, post.Slug)
		item := rssItem{
			Title:       post.Title,
			Link:        link,
			Description: post.Excerpt,
			PubDate:     formatRSSTime(post.PublishedAt, post.CreatedAt),
		}
		items = append(items, item)
	}

	feed := rssFeed{
		Version: "2.0",
		Channel: rssChannel{
			Title:       siteName,
			Link:        siteURL,
			Description: siteName,
			Items:       items,
		},
	}

	data, _ := xml.MarshalIndent(feed, "", "  ")
	c.Data(http.StatusOK, "application/xml; charset=utf-8", append([]byte(xml.Header), data...))
}

func (h *SEOHandler) ogPost(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}

	post, _, err := h.Store.GetPostBySlug(stores.PostLookupInput{Slug: slug, IncludeTags: false})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "post not found"})
		return
	}

	title := post.Title
	subtitle := buildOGSubtitle(post.Excerpt, post.BodyMarkdown)
	if subtitle == "" {
		subtitle = "Explore insights at explore"
	}

	svg := buildOGSVG(title, subtitle)
	c.Data(http.StatusOK, "image/svg+xml; charset=utf-8", []byte(svg))
}

func (h *SEOHandler) ogCourse(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}

	course, err := h.Store.GetCourseBySlug(stores.CourseLookupInput{Slug: slug})
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "course not found"})
		return
	}

	title := course.Title
	subtitle := buildOGSubtitle(course.Excerpt, course.BodyMarkdown)
	if subtitle == "" {
		subtitle = "Explore courses at explore"
	}

	svg := buildOGSVG(title, subtitle)
	c.Data(http.StatusOK, "image/svg+xml; charset=utf-8", []byte(svg))
}

func (h *SEOHandler) ogHome(c *gin.Context) {
	svg := buildOGSVG("Explore", "Curated newsletters and courses")
	c.Data(http.StatusOK, "image/svg+xml; charset=utf-8", []byte(svg))
}

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
}

func formatSitemapTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02")
}

func formatRSSTime(publishedAt *time.Time, createdAt time.Time) string {
	if publishedAt != nil {
		return publishedAt.Format(time.RFC1123Z)
	}
	return createdAt.Format(time.RFC1123Z)
}

func resolveSiteURLFromStore(store *stores.Store) string {
	settings, _ := store.GetSiteSettings()
	return resolveSiteURL(settings)
}

func resolveSiteNameFromStore(store *stores.Store) string {
	settings, _ := store.GetSiteSettings()
	return resolveSiteName(settings)
}

const ogSubtitleLimit = 120

var (
	ogLinkRegex  = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`)
	ogImageRegex = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`)
	ogSpaceRegex = regexp.MustCompile(`\s+`)
)

func buildOGSubtitle(excerpt, markdown string) string {
	trimmedExcerpt := strings.TrimSpace(excerpt)
	if trimmedExcerpt != "" {
		return trimmedExcerpt
	}
	plain := markdownToPlainTextForOG(markdown)
	return truncateOGText(plain, ogSubtitleLimit)
}

func markdownToPlainTextForOG(markdown string) string {
	text := strings.TrimSpace(markdown)
	if text == "" {
		return ""
	}

	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\n", " ")

	text = ogLinkRegex.ReplaceAllString(text, "$1")

	text = ogImageRegex.ReplaceAllString(text, "$1")

	stripper := strings.NewReplacer("**", "", "__", "", "*", "", "`", "", "#", "", "_", "", ">", "")
	text = stripper.Replace(text)

	text = ogSpaceRegex.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func truncateOGText(text string, limit int) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	if limit <= 0 || len(trimmed) <= limit {
		return trimmed
	}
	return strings.TrimSpace(trimmed[:limit])
}

func buildOGSVG(title, subtitle string) string {
	escapedTitle := xmlEscape(title)
	escapedSubtitle := xmlEscape(subtitle)

	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<svg width="1200" height="630" viewBox="0 0 1200 630" fill="none" xmlns="http://www.w3.org/2000/svg">
  <defs>
    <linearGradient id="bg" x1="0" y1="0" x2="1200" y2="630" gradientUnits="userSpaceOnUse">
      <stop stop-color="#0B0F1A"/>
      <stop offset="1" stop-color="#1E2A44"/>
    </linearGradient>
  </defs>
  <rect width="1200" height="630" fill="url(#bg)"/>
  <rect x="72" y="72" width="1056" height="486" rx="32" fill="#0F172A" opacity="0.85"/>
  <text x="120" y="240" fill="#F8FAFC" font-size="64" font-family="'Space Grotesk', sans-serif" font-weight="700">%s</text>
  <text x="120" y="330" fill="#94A3B8" font-size="32" font-family="'Space Grotesk', sans-serif">%s</text>
  <text x="120" y="520" fill="#38BDF8" font-size="24" font-family="'Space Grotesk', sans-serif">explore.mcpx.in</text>
</svg>`, escapedTitle, escapedSubtitle)
}

func xmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}
