package services

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
)

type MarkdownRenderInput struct {
	Markdown string
}

type MarkdownRenderOutput struct {
	HTML string
}

var vimeoURLRegex = regexp.MustCompile(`^https?://vimeo\.com/(\d+)$`)
var vimeoShortcodeRegex = regexp.MustCompile(`::vimeo\{url=\"(https?://vimeo\.com/\d+)\"\}`)

func RenderMarkdown(input MarkdownRenderInput) (MarkdownRenderOutput, error) {
	content := strings.TrimSpace(input.Markdown)
	if content == "" {
		return MarkdownRenderOutput{HTML: ""}, nil
	}

	expanded := expandVimeoEmbeds(content)
	parser := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithRendererOptions(html.WithUnsafe()),
	)

	buffer := bytes.Buffer{}
	if err := parser.Convert([]byte(expanded), &buffer); err != nil {
		return MarkdownRenderOutput{}, err
	}

	policy := markdownPolicy()
	sanitized := policy.Sanitize(buffer.String())

	return MarkdownRenderOutput{HTML: sanitized}, nil
}

func expandVimeoEmbeds(markdown string) string {
	lines := strings.Split(markdown, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		if id := matchVimeoURL(trimmed); id != "" {
			lines[i] = vimeoIframeHTML(id)
			continue
		}

		if url := matchVimeoShortcode(trimmed); url != "" {
			id := matchVimeoURL(url)
			if id != "" {
				lines[i] = vimeoIframeHTML(id)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func matchVimeoURL(line string) string {
	matches := vimeoURLRegex.FindStringSubmatch(line)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func matchVimeoShortcode(line string) string {
	matches := vimeoShortcodeRegex.FindStringSubmatch(line)
	if len(matches) < 2 {
		return ""
	}
	return matches[1]
}

func vimeoIframeHTML(videoID string) string {
	return fmt.Sprintf(`<div class="vimeo-embed"><iframe src="https://player.vimeo.com/video/%s" frameborder="0" allow="autoplay; fullscreen; picture-in-picture" allowfullscreen></iframe></div>`, videoID)
}

func markdownPolicy() *bluemonday.Policy {
	policy := bluemonday.UGCPolicy()
	policy.AllowAttrs("class").OnElements("div")
	policy.AllowElements("iframe")
	policy.AllowAttrs("allow", "allowfullscreen", "frameborder").OnElements("iframe")
	policy.AllowDataURIImages()
	policy.AllowAttrs("src").Matching(regexp.MustCompile(`^https://player\.vimeo\.com/video/\d+$`)).OnElements("iframe")
	return policy
}
