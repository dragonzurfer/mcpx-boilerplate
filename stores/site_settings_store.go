package stores

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type SiteSettingsInput struct {
	SiteName              string
	SiteURL               string
	DefaultOgImageURL     string
	PrimaryColor          string
	TwitterSite           string
	TwitterCreator        string
	GoogleVerification    string
	BingVerification      string
	PinterestVerification string
}

func (s *Store) GetSiteSettings() (*SiteSettingsModel, error) {
	settings := SiteSettingsModel{}
	err := s.db.First(&settings).Error
	if err == nil {
		return &settings, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return nil, gorm.ErrRecordNotFound
}

func (s *Store) EnsureSiteSettings(defaults SiteSettingsInput) (*SiteSettingsModel, error) {
	settings, err := s.GetSiteSettings()
	if err == nil {
		return settings, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	settings = &SiteSettingsModel{
		SiteName:              strings.TrimSpace(defaults.SiteName),
		SiteURL:               strings.TrimSpace(defaults.SiteURL),
		DefaultOgImageURL:     strings.TrimSpace(defaults.DefaultOgImageURL),
		PrimaryColor:          strings.TrimSpace(defaults.PrimaryColor),
		TwitterSite:           strings.TrimSpace(defaults.TwitterSite),
		TwitterCreator:        strings.TrimSpace(defaults.TwitterCreator),
		GoogleVerification:    strings.TrimSpace(defaults.GoogleVerification),
		BingVerification:      strings.TrimSpace(defaults.BingVerification),
		PinterestVerification: strings.TrimSpace(defaults.PinterestVerification),
		UpdatedAt:             time.Now().UTC(),
	}

	if err := s.db.Create(settings).Error; err != nil {
		return nil, err
	}
	return settings, nil
}

func (s *Store) UpdateSiteSettings(input SiteSettingsInput) (*SiteSettingsModel, error) {
	settings, err := s.GetSiteSettings()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return s.EnsureSiteSettings(input)
		}
		return nil, err
	}

	updates := map[string]interface{}{
		"site_name":              strings.TrimSpace(input.SiteName),
		"site_url":               strings.TrimSpace(input.SiteURL),
		"default_og_image_url":   strings.TrimSpace(input.DefaultOgImageURL),
		"primary_color":          strings.TrimSpace(input.PrimaryColor),
		"twitter_site":           strings.TrimSpace(input.TwitterSite),
		"twitter_creator":        strings.TrimSpace(input.TwitterCreator),
		"google_verification":    strings.TrimSpace(input.GoogleVerification),
		"bing_verification":      strings.TrimSpace(input.BingVerification),
		"pinterest_verification": strings.TrimSpace(input.PinterestVerification),
		"updated_at":             time.Now().UTC(),
	}

	if err := s.db.Model(settings).Updates(updates).Error; err != nil {
		return nil, err
	}
	return settings, nil
}
