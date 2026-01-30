package routes

import (
	"testing"

	"github.com/mcpx/boilerplate/stores"
)

func TestResolveSiteNameDefaultsToSubdomain(t *testing.T) {
	got := resolveSiteName(nil)
	if got != "explore" {
		t.Fatalf("expected default site name explore, got %q", got)
	}
}

func TestResolveSiteNameUsesSetting(t *testing.T) {
	settings := &stores.SiteSettingsModel{SiteName: "Custom"}
	got := resolveSiteName(settings)
	if got != "Custom" {
		t.Fatalf("expected custom site name, got %q", got)
	}
}

func TestResolveSiteNameFallsBackWhenBlank(t *testing.T) {
	settings := &stores.SiteSettingsModel{SiteName: "   "}
	got := resolveSiteName(settings)
	if got != "explore" {
		t.Fatalf("expected fallback site name explore, got %q", got)
	}
}

func TestResolveSiteURLDefaultsToExplore(t *testing.T) {
	got := resolveSiteURL(nil)
	if got != "https://explore.mcpx.in" {
		t.Fatalf("expected default site url https://explore.mcpx.in, got %q", got)
	}
}

func TestResolveSiteURLUsesSetting(t *testing.T) {
	settings := &stores.SiteSettingsModel{SiteURL: "https://example.com"}
	got := resolveSiteURL(settings)
	if got != "https://example.com" {
		t.Fatalf("expected custom site url, got %q", got)
	}
}

func TestResolveSiteURLTrimsTrailingSlash(t *testing.T) {
	settings := &stores.SiteSettingsModel{SiteURL: "https://example.com/"}
	got := resolveSiteURL(settings)
	if got != "https://example.com" {
		t.Fatalf("expected trimmed site url, got %q", got)
	}
}

func TestResolveSiteURLFallsBackWhenBlank(t *testing.T) {
	settings := &stores.SiteSettingsModel{SiteURL: "   "}
	got := resolveSiteURL(settings)
	if got != "https://explore.mcpx.in" {
		t.Fatalf("expected fallback site url, got %q", got)
	}
}

func TestResolvePrimaryColorDefaults(t *testing.T) {
	got := resolvePrimaryColor(nil)
	if got != defaultPrimaryColor {
		t.Fatalf("expected default primary color %q, got %q", defaultPrimaryColor, got)
	}
}

func TestResolvePrimaryColorUsesSetting(t *testing.T) {
	settings := &stores.SiteSettingsModel{PrimaryColor: "#ff9900"}
	got := resolvePrimaryColor(settings)
	if got != "#ff9900" {
		t.Fatalf("expected primary color, got %q", got)
	}
}

func TestResolvePrimaryColorFallsBackWhenBlank(t *testing.T) {
	settings := &stores.SiteSettingsModel{PrimaryColor: "   "}
	got := resolvePrimaryColor(settings)
	if got != defaultPrimaryColor {
		t.Fatalf("expected fallback primary color %q, got %q", defaultPrimaryColor, got)
	}
}
