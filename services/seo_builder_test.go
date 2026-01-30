package services

import (
	"strings"
	"testing"
	"time"

	"github.com/mcpx/boilerplate/stores"
)

func TestBuildMetaUsesExplicitOverrides(t *testing.T) {
	publishedAt := time.Date(2024, 10, 24, 12, 0, 0, 0, time.UTC)
	post := stores.PostModel{
		ID:             10,
		Slug:           "deep-dive",
		Title:          "Deep Dive",
		Excerpt:        "",
		BodyMarkdown:   "Hello **world**",
		AccessLevel:    stores.AccessLevelPublic,
		Status:         stores.PostStatusPublished,
		PublishedAt:    &publishedAt,
		MetaTitle:      "Custom Title",
		MetaDescription: "Custom description",
		MetaImageURL:   "https://cdn.example.com/og.png",
		CanonicalURL:   "https://explore.mcpx.in/post/deep-dive",
	}

	site := SiteSettings{
		SiteName: "Explore",
		SiteURL:  "https://explore.mcpx.in",
	}

	input := MetaBuildInput{
		PageType:  PageTypePost,
		Post:      &post,
		Site:      site,
		TopicTags: []string{"AI", "Design"},
	}

	output := BuildMeta(input)

	if output.Title != "Custom Title | Explore" {
		t.Fatalf("expected title to use override, got %q", output.Title)
	}
	if output.Description != "Custom description" {
		t.Fatalf("expected description override, got %q", output.Description)
	}
	if output.ImageURL != "https://cdn.example.com/og.png" {
		t.Fatalf("expected explicit image, got %q", output.ImageURL)
	}
	if output.CanonicalURL != "https://explore.mcpx.in/post/deep-dive" {
		t.Fatalf("expected canonical override, got %q", output.CanonicalURL)
	}
	if output.Robots != "index,follow" {
		t.Fatalf("expected indexable robots, got %q", output.Robots)
	}
	if output.OgType != "article" {
		t.Fatalf("expected article og:type, got %q", output.OgType)
	}
	if len(output.ArticleTags) != 2 {
		t.Fatalf("expected topic tags in og metadata")
	}
	if !strings.Contains(output.JSONLD, "BlogPosting") {
		t.Fatalf("expected BlogPosting JSON-LD")
	}
}

func TestBuildMetaFallbacks(t *testing.T) {
	post := stores.PostModel{
		ID:           11,
		Slug:         "fallback-post",
		Title:        "Fallback",
		Excerpt:      "Short summary",
		BodyMarkdown: "# Heading\n\nSome **content** here.",
		AccessLevel:  stores.AccessLevelPublic,
		Status:       stores.PostStatusPublished,
	}

	site := SiteSettings{
		SiteName: "Explore",
		SiteURL:  "https://explore.mcpx.in",
	}

	output := BuildMeta(MetaBuildInput{PageType: PageTypePost, Post: &post, Site: site})

	if output.Description != "Short summary" {
		t.Fatalf("expected excerpt fallback, got %q", output.Description)
	}
	if output.ImageURL != "https://explore.mcpx.in/og/post/fallback-post" {
		t.Fatalf("expected default og image url, got %q", output.ImageURL)
	}
}

func TestBuildMetaNoIndex(t *testing.T) {
	post := stores.PostModel{
		ID:          12,
		Slug:        "draft-post",
		Title:       "Draft",
		AccessLevel: stores.AccessLevelPublic,
		Status:      stores.PostStatusDraft,
		NoIndex:     true,
	}

	site := SiteSettings{SiteName: "Explore", SiteURL: "https://explore.mcpx.in"}
	output := BuildMeta(MetaBuildInput{PageType: PageTypePost, Post: &post, Site: site})

	if output.Robots != "noindex,nofollow" {
		t.Fatalf("expected noindex robots, got %q", output.Robots)
	}
}

func TestBuildMetaMarkdownFallbackDescription(t *testing.T) {
	post := stores.PostModel{
		ID:           13,
		Slug:         "markdown-post",
		Title:        "Markdown",
		BodyMarkdown: "**Bold** example with a [link](https://example.com).",
		AccessLevel:  stores.AccessLevelPublic,
		Status:       stores.PostStatusPublished,
	}

	site := SiteSettings{SiteName: "Explore", SiteURL: "https://explore.mcpx.in"}
	output := BuildMeta(MetaBuildInput{PageType: PageTypePost, Post: &post, Site: site})

	if output.Description == "" {
		t.Fatal("expected markdown-derived description")
	}
}
