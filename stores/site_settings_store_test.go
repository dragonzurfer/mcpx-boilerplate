package stores

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUpdateSiteSettingsPersistsPrimaryColor(t *testing.T) {
	store := newTestStore(t)

	settings, err := store.UpdateSiteSettings(SiteSettingsInput{
		SiteName:     "Explore",
		SiteURL:      "https://explore.mcpx.in",
		PrimaryColor: "#ff6600",
	})
	if err != nil {
		t.Fatalf("expected update to succeed, got %v", err)
	}
	if settings.PrimaryColor != "#ff6600" {
		t.Fatalf("expected primary color to persist, got %q", settings.PrimaryColor)
	}

	loaded, err := store.GetSiteSettings()
	if err != nil {
		t.Fatalf("expected settings to load, got %v", err)
	}
	if loaded.PrimaryColor != "#ff6600" {
		t.Fatalf("expected primary color to persist, got %q", loaded.PrimaryColor)
	}
}

func TestEnsureSiteSettingsKeepsPrimaryColor(t *testing.T) {
	store := newTestStore(t)
	_, err := store.EnsureSiteSettings(SiteSettingsInput{
		SiteName:     "Explore",
		SiteURL:      "https://explore.mcpx.in",
		PrimaryColor: "#123456",
	})
	if err != nil {
		t.Fatalf("expected ensure to succeed, got %v", err)
	}

	settings, err := store.GetSiteSettings()
	if err != nil {
		t.Fatalf("expected settings to load, got %v", err)
	}
	if settings.PrimaryColor != "#123456" {
		t.Fatalf("expected primary color to persist, got %q", settings.PrimaryColor)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&SiteSettingsModel{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return &Store{db: db}
}
