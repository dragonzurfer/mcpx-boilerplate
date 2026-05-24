package routes

import (
	"strings"
	"testing"
)

func TestValidatePhoneFromInputIndia(t *testing.T) {
	req := CompletePhoneRequest{
		CountryCode:    "IN",
		NationalNumber: "9876543210",
	}

	phoneDetails, err := validatePhoneFromInput(req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if phoneDetails.CountryCode != "IN" {
		t.Fatalf("expected IN, got %s", phoneDetails.CountryCode)
	}
	if phoneDetails.NationalNumber != "9876543210" {
		t.Fatalf("expected normalized national number, got %s", phoneDetails.NationalNumber)
	}
	if !strings.HasPrefix(phoneDetails.E164, "+91") {
		t.Fatalf("expected +91 prefix, got %s", phoneDetails.E164)
	}
}

func TestValidatePhoneFromInputIndiaLength(t *testing.T) {
	req := CompletePhoneRequest{
		CountryCode:    "IN",
		NationalNumber: "987654321",
	}

	_, err := validatePhoneFromInput(req)
	if err == nil {
		t.Fatal("expected error for invalid Indian number length")
	}
}

func TestValidatePhoneFromInputUnsupportedCountry(t *testing.T) {
	req := CompletePhoneRequest{
		CountryCode:    "XX",
		NationalNumber: "1234567890",
	}

	_, err := validatePhoneFromInput(req)
	if err == nil {
		t.Fatal("expected error for unsupported country")
	}
}

func TestSignAndParsePhoneCompletionToken(t *testing.T) {
	secret := "test-secret"
	issuer := "test-issuer"
	provider := "google"
	subject := "google-subject-1"
	userID := uint(42)

	token, err := signPhoneCompletionToken(phoneCompletionTokenInput{
		JWTSecret:      secret,
		Issuer:         issuer,
		Provider:       provider,
		ProviderUserID: subject,
		UserID:         userID,
	})
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	claims, err := parsePhoneCompletionClaims(token, secret)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}

	if claims.UserID != userID {
		t.Fatalf("expected user id %d, got %d", userID, claims.UserID)
	}
	if claims.Subject != subject {
		t.Fatalf("expected subject %s, got %s", subject, claims.Subject)
	}
	if claims.Provider != provider {
		t.Fatalf("expected provider %s, got %s", provider, claims.Provider)
	}
	if claims.Purpose != authPhoneTokenPurpose {
		t.Fatalf("expected purpose %s, got %s", authPhoneTokenPurpose, claims.Purpose)
	}
	if claims.Issuer != issuer {
		t.Fatalf("expected issuer %s, got %s", issuer, claims.Issuer)
	}
}

func TestBuildPhoneCountriesIncludesIndia(t *testing.T) {
	countries := buildPhoneCountries()
	if len(countries) == 0 {
		t.Fatal("expected at least one supported phone country")
	}

	foundIndia := false
	for _, country := range countries {
		if country.RegionCode == "IN" && country.DialCode == "+91" {
			foundIndia = true
			break
		}
	}

	if !foundIndia {
		t.Fatal("expected IN (+91) to be available")
	}
}
