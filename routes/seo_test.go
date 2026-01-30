package routes

import "testing"

func TestBuildOGSubtitleUsesExcerpt(t *testing.T) {
	got := buildOGSubtitle(" Short excerpt ", "Ignored **markdown**")
	if got != "Short excerpt" {
		t.Fatalf("expected excerpt, got %q", got)
	}
}

func TestBuildOGSubtitleFallsBackToMarkdown(t *testing.T) {
	markdown := "This is **bold** and a [link](https://example.com)."
	got := buildOGSubtitle("", markdown)
	if got != "This is bold and a link." {
		t.Fatalf("expected markdown fallback, got %q", got)
	}
}

func TestBuildOGSubtitleTruncates(t *testing.T) {
	longText := "Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua."
	got := buildOGSubtitle("", longText)
	if len(got) > ogSubtitleLimit {
		t.Fatalf("expected truncated subtitle, got length %d", len(got))
	}
}
