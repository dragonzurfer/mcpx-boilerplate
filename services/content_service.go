package services

import (
	"strings"
	"time"

	"github.com/mcpx/boilerplate/stores"
	"gorm.io/gorm"
)

type ContentService struct {
	Store *stores.Store
}

type PostAccessInput struct {
	Slug          string
	Viewer        *stores.UserModel
	Now           time.Time
	UseHTMLCache  bool
}

type PostAccessOutput struct {
	Post        *stores.PostModel
	Tags        []stores.TagModel
	HTML        string
	TeaserHTML  string
	IsLocked    bool
	GateType    string
	AccessLevel string
	EntitlementActive bool
}

const (
	GateTypeLoginRequired = "LOGIN_REQUIRED"
	GateTypePaywall       = "PAYWALL"
)

func (s *ContentService) GetPostBySlug(input PostAccessInput) (PostAccessOutput, error) {
	post, tags, err := s.Store.GetPostBySlug(stores.PostLookupInput{Slug: input.Slug, IncludeTags: true})
	if err != nil {
		return PostAccessOutput{}, err
	}

	entitlementActive := false
	if input.Viewer != nil {
		entitlementActive = hasActiveEntitlement(s.Store, input.Viewer.ID, input.Now)
	}

	gate := resolvePostGate(post, input.Viewer, entitlementActive)
	html, teaserHTML := renderPostContent(s.Store, post, input.UseHTMLCache)

	return PostAccessOutput{
		Post:             post,
		Tags:             tags,
		HTML:             html,
		TeaserHTML:       teaserHTML,
		IsLocked:         gate.IsLocked,
		GateType:         gate.GateType,
		AccessLevel:      post.AccessLevel,
		EntitlementActive: entitlementActive,
	}, nil
}

type CourseAccessInput struct {
	Slug         string
	Viewer       *stores.UserModel
	Now          time.Time
	UseHTMLCache bool
}

type CourseAccessOutput struct {
	Course            *stores.CourseModel
	HTML              string
	TeaserHTML        string
	IsLocked          bool
	GateType          string
	AccessLevel       string
	EntitlementActive bool
}

func (s *ContentService) GetCourseBySlug(input CourseAccessInput) (CourseAccessOutput, error) {
	course, err := s.Store.GetCourseBySlug(stores.CourseLookupInput{Slug: input.Slug})
	if err != nil {
		return CourseAccessOutput{}, err
	}

	entitlementActive := false
	if input.Viewer != nil {
		entitlementActive = hasActiveEntitlement(s.Store, input.Viewer.ID, input.Now)
	}

	gate := resolveCourseGate(course, input.Viewer, entitlementActive)
	html, teaser := renderCourseContent(s.Store, course, input.UseHTMLCache)

	return CourseAccessOutput{
		Course:            course,
		HTML:              html,
		TeaserHTML:        teaser,
		IsLocked:          gate.IsLocked,
		GateType:          gate.GateType,
		AccessLevel:       course.AccessLevel,
		EntitlementActive: entitlementActive,
	}, nil
}

type gateDecision struct {
	IsLocked bool
	GateType string
}

func resolvePostGate(post *stores.PostModel, viewer *stores.UserModel, entitlementActive bool) gateDecision {
	if viewer == nil {
		return gateDecision{IsLocked: true, GateType: GateTypeLoginRequired}
	}

	access := strings.ToUpper(strings.TrimSpace(post.AccessLevel))
	if access == stores.AccessLevelPaid && !entitlementActive {
		return gateDecision{IsLocked: true, GateType: GateTypePaywall}
	}
	return gateDecision{IsLocked: false, GateType: ""}
}

func resolveCourseGate(course *stores.CourseModel, viewer *stores.UserModel, entitlementActive bool) gateDecision {
	if viewer == nil {
		return gateDecision{IsLocked: true, GateType: GateTypeLoginRequired}
	}

	access := strings.ToUpper(strings.TrimSpace(course.AccessLevel))
	if access == stores.AccessLevelPaid && !entitlementActive {
		return gateDecision{IsLocked: true, GateType: GateTypePaywall}
	}
	return gateDecision{IsLocked: false, GateType: ""}
}

func renderPostContent(store *stores.Store, post *stores.PostModel, useCache bool) (string, string) {
	fullHTML := ""
	if useCache && strings.TrimSpace(post.BodyHTMLCache) != "" {
		fullHTML = post.BodyHTMLCache
	} else {
		output, err := RenderMarkdown(MarkdownRenderInput{Markdown: post.BodyMarkdown})
		if err == nil {
			fullHTML = output.HTML
			_ = store.UpdatePostHTMLCache(stores.PostHTMLCacheInput{PostID: post.ID, HTML: fullHTML})
		}
	}

	teaserMarkdown := buildTeaserMarkdown(post.BodyMarkdown)
	teaserHTML := ""
	if teaserMarkdown != "" {
		output, err := RenderMarkdown(MarkdownRenderInput{Markdown: teaserMarkdown})
		if err == nil {
			teaserHTML = output.HTML
		}
	}

	return fullHTML, teaserHTML
}

func renderCourseContent(store *stores.Store, course *stores.CourseModel, useCache bool) (string, string) {
	fullHTML := ""
	if useCache && strings.TrimSpace(course.BodyHTMLCache) != "" {
		fullHTML = course.BodyHTMLCache
	} else {
		output, err := RenderMarkdown(MarkdownRenderInput{Markdown: course.BodyMarkdown})
		if err == nil {
			fullHTML = output.HTML
			_ = store.UpdateCourseHTMLCache(course.ID, fullHTML)
		}
	}

	teaserMarkdown := buildTeaserMarkdown(course.BodyMarkdown)
	teaserHTML := ""
	if teaserMarkdown != "" {
		output, err := RenderMarkdown(MarkdownRenderInput{Markdown: teaserMarkdown})
		if err == nil {
			teaserHTML = output.HTML
		}
	}

	return fullHTML, teaserHTML
}

func buildTeaserMarkdown(markdown string) string {
	parts := strings.Split(markdown, "\n\n")
	if len(parts) == 0 {
		return ""
	}
	limit := 2
	if len(parts) < limit {
		limit = len(parts)
	}
	return strings.Join(parts[:limit], "\n\n")
}

func hasActiveEntitlement(store *stores.Store, userID uint, now time.Time) bool {
	_, err := store.GetActiveEntitlement(stores.EntitlementLookupInput{UserID: userID, Now: now})
	return err == nil
}

func (s *ContentService) ListPosts(input stores.PostListInput) (stores.PostListOutput, error) {
	return s.Store.ListPosts(input)
}

func (s *ContentService) ListCourses(input stores.CourseListInput) (stores.CourseListOutput, error) {
	return s.Store.ListCourses(input)
}

func (s *ContentService) GetPostByID(postID uint) (*stores.PostModel, error) {
	if postID == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	post := stores.PostModel{}
	if err := s.Store.DB().Where("id = ?", postID).First(&post).Error; err != nil {
		return nil, err
	}
	return &post, nil
}
