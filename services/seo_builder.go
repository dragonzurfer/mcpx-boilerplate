package services

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mcpx/boilerplate/stores"
)

type PageType string

const (
	PageTypePost   PageType = "post"
	PageTypeCourse PageType = "course"
	PageTypeHome   PageType = "home"
)

type SiteSettings struct {
	SiteName           string
	SiteURL            string
	DefaultOgImageURL  string
	PrimaryColor       string
	TwitterSite        string
	TwitterCreator     string
	GoogleVerification string
	BingVerification   string
	PinterestVerify    string
}

type MetaBuildInput struct {
	PageType  PageType
	Post      *stores.PostModel
	Course    *stores.CourseModel
	Site      SiteSettings
	TopicTags []string
}

type MetaOutput struct {
	Title                 string
	Description           string
	ImageURL              string
	CanonicalURL          string
	Robots                string
	OgType                string
	SiteName              string
	ArticleTags           []string
	PublishedTime         string
	ModifiedTime          string
	JSONLD                string
	TwitterSite           string
	TwitterCreator        string
	GoogleVerification    string
	BingVerification      string
	PinterestVerification string
}

const metaDescriptionLimit = 160

func BuildMeta(input MetaBuildInput) MetaOutput {
	siteURL := normalizeSiteURL(input.Site.SiteURL)
	page := input.PageType

	defaults := metaDefaults(page, siteURL)

	title := resolveTitle(input, defaults)
	description := resolveDescription(input)
	imageURL := resolveImageURL(input, defaults)
	canonical := resolveCanonicalURL(input, defaults)
	robots := resolveRobots(input)
	ogType := resolveOgType(page)
	publishedTime, modifiedTime := resolveTimes(input)
	jsonld := buildJSONLD(input, title, description, imageURL, canonical, publishedTime, modifiedTime)

	return MetaOutput{
		Title:                 title,
		Description:           description,
		ImageURL:              imageURL,
		CanonicalURL:          canonical,
		Robots:                robots,
		OgType:                ogType,
		SiteName:              strings.TrimSpace(input.Site.SiteName),
		ArticleTags:           append([]string{}, input.TopicTags...),
		PublishedTime:         publishedTime,
		ModifiedTime:          modifiedTime,
		JSONLD:                jsonld,
		TwitterSite:           strings.TrimSpace(input.Site.TwitterSite),
		TwitterCreator:        strings.TrimSpace(input.Site.TwitterCreator),
		GoogleVerification:    strings.TrimSpace(input.Site.GoogleVerification),
		BingVerification:      strings.TrimSpace(input.Site.BingVerification),
		PinterestVerification: strings.TrimSpace(input.Site.PinterestVerify),
	}
}

func metaDefaults(page PageType, siteURL string) metaDefaultsOutput {
	return metaDefaultsOutput{
		SiteURL: siteURL,
		Page:    page,
	}
}

type metaDefaultsOutput struct {
	SiteURL string
	Page    PageType
}

func normalizeSiteURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimRight(trimmed, "/")
	return trimmed
}

func resolveTitle(input MetaBuildInput, defaults metaDefaultsOutput) string {
	siteName := strings.TrimSpace(input.Site.SiteName)
	resolved := ""

	switch input.PageType {
	case PageTypePost:
		if input.Post != nil {
			resolved = strings.TrimSpace(input.Post.MetaTitle)
			if resolved == "" {
				resolved = strings.TrimSpace(input.Post.Title)
			}
		}
	case PageTypeCourse:
		if input.Course != nil {
			resolved = strings.TrimSpace(input.Course.MetaTitle)
			if resolved == "" {
				resolved = strings.TrimSpace(input.Course.Title)
			}
		}
	case PageTypeHome:
		resolved = siteName
	}

	if resolved == "" {
		resolved = siteName
	}

	if siteName == "" || resolved == siteName {
		return resolved
	}
	return fmt.Sprintf("%s | %s", resolved, siteName)
}

func resolveDescription(input MetaBuildInput) string {
	switch input.PageType {
	case PageTypePost:
		if input.Post == nil {
			return ""
		}
		return buildDescription(input.Post.MetaDescription, input.Post.Excerpt, input.Post.BodyMarkdown)
	case PageTypeCourse:
		if input.Course == nil {
			return ""
		}
		return buildDescription(input.Course.MetaDescription, input.Course.Excerpt, input.Course.BodyMarkdown)
	case PageTypeHome:
		return strings.TrimSpace(input.Site.SiteName)
	default:
		return ""
	}
}

func buildDescription(metaDescription, excerpt, markdown string) string {
	desc := strings.TrimSpace(metaDescription)
	if desc != "" {
		return desc
	}
	desc = strings.TrimSpace(excerpt)
	if desc != "" {
		return desc
	}
	plain := markdownToPlainText(markdown)
	return truncateText(plain, metaDescriptionLimit)
}

func resolveImageURL(input MetaBuildInput, defaults metaDefaultsOutput) string {
	custom := resolveEntityImage(input)
	if custom == "" {
		custom = strings.TrimSpace(input.Site.DefaultOgImageURL)
	}
	if custom == "" {
		custom = defaultOgImageURL(input, defaults)
	}
	custom = strings.TrimSpace(custom)
	if custom == "" {
		return ""
	}
	if strings.HasPrefix(custom, "http://") || strings.HasPrefix(custom, "https://") {
		return custom
	}
	if defaults.SiteURL == "" {
		return custom
	}
	return defaults.SiteURL + "/" + strings.TrimLeft(custom, "/")
}

func resolveEntityImage(input MetaBuildInput) string {
	switch input.PageType {
	case PageTypePost:
		if input.Post == nil {
			return ""
		}
		return strings.TrimSpace(input.Post.MetaImageURL)
	case PageTypeCourse:
		if input.Course == nil {
			return ""
		}
		return strings.TrimSpace(input.Course.MetaImageURL)
	default:
		return ""
	}
}

func defaultOgImageURL(input MetaBuildInput, defaults metaDefaultsOutput) string {
	switch input.PageType {
	case PageTypePost:
		if input.Post == nil {
			return ""
		}
		return fmt.Sprintf("%s/og/post/%s", defaults.SiteURL, input.Post.Slug)
	case PageTypeCourse:
		if input.Course == nil {
			return ""
		}
		return fmt.Sprintf("%s/og/course/%s", defaults.SiteURL, input.Course.Slug)
	case PageTypeHome:
		return fmt.Sprintf("%s/og/home", defaults.SiteURL)
	default:
		return ""
	}
}

func resolveCanonicalURL(input MetaBuildInput, defaults metaDefaultsOutput) string {
	if input.PageType == PageTypePost && input.Post != nil {
		canonical := strings.TrimSpace(input.Post.CanonicalURL)
		if canonical != "" {
			return canonical
		}
		if defaults.SiteURL != "" {
			return fmt.Sprintf("%s/post/%s", defaults.SiteURL, input.Post.Slug)
		}
	}

	if input.PageType == PageTypeCourse && input.Course != nil {
		canonical := strings.TrimSpace(input.Course.CanonicalURL)
		if canonical != "" {
			return canonical
		}
		if defaults.SiteURL != "" {
			return fmt.Sprintf("%s/course/%s", defaults.SiteURL, input.Course.Slug)
		}
	}

	if input.PageType == PageTypeHome && defaults.SiteURL != "" {
		return defaults.SiteURL + "/"
	}

	return ""
}

func resolveRobots(input MetaBuildInput) string {
	if input.PageType == PageTypePost && input.Post != nil {
		if input.Post.NoIndex || input.Post.Status != stores.PostStatusPublished {
			return "noindex,nofollow"
		}
		return "index,follow"
	}
	if input.PageType == PageTypeCourse && input.Course != nil {
		if input.Course.NoIndex || input.Course.Status != stores.CourseStatusPublished {
			return "noindex,nofollow"
		}
		return "index,follow"
	}
	return "index,follow"
}

func resolveOgType(page PageType) string {
	if page == PageTypeHome {
		return "website"
	}
	return "article"
}

func resolveTimes(input MetaBuildInput) (string, string) {
	if input.PageType == PageTypePost && input.Post != nil {
		return timeString(input.Post.PublishedAt), timeString(timePointer(input.Post.UpdatedAt))
	}
	if input.PageType == PageTypeCourse && input.Course != nil {
		return timeString(input.Course.PublishedAt), timeString(timePointer(input.Course.UpdatedAt))
	}
	return "", ""
}

func timePointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func timeString(value *time.Time) string {
	if value == nil {
		return ""
	}
	return value.Format("2006-01-02T15:04:05Z07:00")
}

func buildJSONLD(input MetaBuildInput, title, description, imageURL, canonical, publishedTime, modifiedTime string) string {
	if input.PageType != PageTypePost && input.PageType != PageTypeCourse {
		return ""
	}

	parts := []string{
		"{",
		"\"@context\":\"https://schema.org\"",
	}

	if input.PageType == PageTypeCourse {
		parts = append(parts, ",\"@type\":\"Course\"")
	} else {
		parts = append(parts, ",\"@type\":\"BlogPosting\"")
	}

	parts = append(parts, fmt.Sprintf(",\"headline\":%s", jsonString(title)))
	if description != "" {
		parts = append(parts, fmt.Sprintf(",\"description\":%s", jsonString(description)))
	}
	if canonical != "" {
		parts = append(parts, fmt.Sprintf(",\"url\":%s", jsonString(canonical)))
	}
	if imageURL != "" {
		parts = append(parts, fmt.Sprintf(",\"image\":[%s]", jsonString(imageURL)))
	}
	if publishedTime != "" {
		parts = append(parts, fmt.Sprintf(",\"datePublished\":%s", jsonString(publishedTime)))
	}
	if modifiedTime != "" {
		parts = append(parts, fmt.Sprintf(",\"dateModified\":%s", jsonString(modifiedTime)))
	}

	parts = append(parts, "}")
	return strings.Join(parts, "")
}

func jsonString(value string) string {
	escaped := strings.ReplaceAll(value, "\\", "\\\\")
	escaped = strings.ReplaceAll(escaped, "\"", "\\\"")
	return fmt.Sprintf("\"%s\"", escaped)
}

var markdownStripper = regexp.MustCompile(`\s+`)

func markdownToPlainText(markdown string) string {
	text := strings.TrimSpace(markdown)
	if text == "" {
		return ""
	}

	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\n", " ")

	linkRegex := regexp.MustCompile(`\[([^\]]+)\]\([^\)]+\)`)
	text = linkRegex.ReplaceAllString(text, "$1")

	imageRegex := regexp.MustCompile(`!\[([^\]]*)\]\([^\)]+\)`)
	text = imageRegex.ReplaceAllString(text, "$1")

	stripper := strings.NewReplacer("**", "", "__", "", "*", "", "`", "", "#", "", "_", "", ">", "")
	text = stripper.Replace(text)

	text = markdownStripper.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func truncateText(text string, limit int) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	if limit <= 0 {
		return trimmed
	}
	if len(trimmed) <= limit {
		return trimmed
	}
	return strings.TrimSpace(trimmed[:limit])
}
