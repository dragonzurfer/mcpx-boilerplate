package services

import (
	"strings"
	"testing"
)

func TestRenderMarkdownEmbedsVimeoURL(t *testing.T) {
	input := MarkdownRenderInput{Markdown: "Intro\n\nhttps://vimeo.com/123456\n"}
	output, err := RenderMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output.HTML, "player.vimeo.com/video/123456") {
		t.Fatalf("expected vimeo embed in HTML, got %q", output.HTML)
	}
}

func TestRenderMarkdownEmbedsVimeoShortcode(t *testing.T) {
	input := MarkdownRenderInput{Markdown: "::vimeo{url=\"https://vimeo.com/98765\"}"}
	output, err := RenderMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output.HTML, "player.vimeo.com/video/98765") {
		t.Fatalf("expected shortcode embed, got %q", output.HTML)
	}
}

func TestRenderMarkdownSanitizesIframes(t *testing.T) {
	input := MarkdownRenderInput{Markdown: "<iframe src=\"https://evil.com\"></iframe>"}
	output, err := RenderMarkdown(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(output.HTML, "iframe") {
		t.Fatalf("expected iframe removed, got %q", output.HTML)
	}
}
