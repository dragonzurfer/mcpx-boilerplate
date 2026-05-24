package routes

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/mcpx/boilerplate/stores"
	"github.com/nyaruka/phonenumbers"
	"google.golang.org/api/idtoken"
	"gorm.io/gorm"
)

type GoogleLoginRequest struct {
	GoogleToken string `json:"googleToken" binding:"required"`
	AnonID      string `json:"anon_id"`
}

type CompletePhoneRequest struct {
	PhoneToken     string `json:"phone_token" binding:"required"`
	CountryCode    string `json:"country_code" binding:"required"`
	NationalNumber string `json:"national_number" binding:"required"`
}

type PhoneCountryResponseItem struct {
	RegionCode string `json:"region_code"`
	DialCode   string `json:"dial_code"`
}

type phoneCompletionClaims struct {
	UserID   uint   `json:"userId,omitempty"`
	Provider string `json:"provider,omitempty"`
	Purpose  string `json:"purpose,omitempty"`
	jwt.RegisteredClaims
}

type validatedPhone struct {
	CountryCode    string
	NationalNumber string
	E164           string
}

const (
	authPhoneTokenPurpose = "complete_phone"
	authPhoneTokenTTL     = 15 * time.Minute
	authAppTokenTTL       = 365 * 24 * time.Hour
)

// RegisterAuth registers the /api/auth/login route to exchange Google ID tokens for app tokens.
func RegisterAuth(rg *gin.RouterGroup, store *stores.Store, logf func(string, ...interface{})) {
	audiences := parseGoogleAudiences()
	jwtSecret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	issuer := strings.TrimSpace(os.Getenv("JWT_ISSUER"))
	if issuer == "" {
		issuer = "mcpx"
	}
	phoneCountries := buildPhoneCountries()

	rg.GET("/auth/phone-countries", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"items": phoneCountries})
	})

	rg.POST("/auth/login", func(c *gin.Context) {
		if len(audiences) == 0 {
			logf("error [auth/login]: GOOGLE_CLIENT_ID(S) missing")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "server misconfigured"})
			return
		}
		if jwtSecret == "" {
			logf("error [auth/login]: JWT_SECRET missing")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "server misconfigured"})
			return
		}

		var req GoogleLoginRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "googleToken required"})
			return
		}

		payload, err := validateGoogleIDToken(c.Request.Context(), req.GoogleToken, audiences)
		if err != nil || payload == nil {
			logf("warn [auth/login]: google token invalid: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid google token"})
			return
		}

		name := ""
		if v, ok := payload.Claims["name"].(string); ok {
			name = v
		}
		email := ""
		if v, ok := payload.Claims["email"].(string); ok {
			email = v
		}

		avatar := ""
		if v, ok := payload.Claims["picture"].(string); ok {
			avatar = v
		}

		user, err := store.GetOrCreateUserWithIdentity(stores.UserIdentityInput{
			Provider:       "google",
			ProviderUserID: payload.Subject,
			Email:          email,
			Name:           name,
			AvatarURL:      avatar,
		})
		if err != nil {
			logf("error [auth/login]: persist user: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist user"})
			return
		}

		if strings.TrimSpace(req.AnonID) != "" {
			_ = store.MergeAnonEvents(req.AnonID, user.ID)
			_ = store.MergeAnonPostImpressions(req.AnonID, user.ID)
		}

		if userNeedsPhoneCompletion(user) {
			phoneToken, err := signPhoneCompletionToken(phoneCompletionTokenInput{
				JWTSecret:      jwtSecret,
				Issuer:         issuer,
				Provider:       "google",
				ProviderUserID: payload.Subject,
				UserID:         user.ID,
			})
			if err != nil {
				logf("error [auth/login]: sign phone token: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue phone token"})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"phone_required": true,
				"phone_token":    phoneToken,
				"user":           buildAuthUserPayload(user),
			})
			return
		}

		signed, err := signAppAuthToken(appAuthTokenInput{
			JWTSecret:      jwtSecret,
			Issuer:         issuer,
			Provider:       "google",
			ProviderUserID: payload.Subject,
			UserID:         user.ID,
			Email:          user.Email,
		})
		if err != nil {
			logf("error [auth/login]: sign token: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"phone_required": false,
			"token":          signed,
			"user":           buildAuthUserPayload(user),
		})
	})

	rg.POST("/auth/complete-phone", func(c *gin.Context) {
		if jwtSecret == "" {
			logf("error [auth/complete-phone]: JWT_SECRET missing")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "server misconfigured"})
			return
		}

		var req CompletePhoneRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "phone_token, country_code, and national_number are required"})
			return
		}

		claims, err := parsePhoneCompletionClaims(req.PhoneToken, jwtSecret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired phone token"})
			return
		}

		if claims.Issuer != issuer {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired phone token"})
			return
		}

		phoneDetails, err := validatePhoneFromInput(req)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		user, err := store.GetUserByID(claims.UserID)
		if err != nil {
			if errorsIsRecordNotFound(err) {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
				return
			}
			logf("error [auth/complete-phone]: load user: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load user"})
			return
		}

		if userNeedsPhoneCompletion(user) {
			user, err = store.UpdateUserPhone(stores.UserPhoneUpdateInput{
				UserID:              user.ID,
				PhoneCountryCode:    phoneDetails.CountryCode,
				PhoneNationalNumber: phoneDetails.NationalNumber,
				PhoneE164:           phoneDetails.E164,
			})
			if err != nil {
				logf("error [auth/complete-phone]: update phone: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save phone number"})
				return
			}
		}

		signed, err := signAppAuthToken(appAuthTokenInput{
			JWTSecret:      jwtSecret,
			Issuer:         issuer,
			Provider:       claims.Provider,
			ProviderUserID: claims.Subject,
			UserID:         user.ID,
			Email:          user.Email,
		})
		if err != nil {
			logf("error [auth/complete-phone]: sign token: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"token": signed,
			"user":  buildAuthUserPayload(user),
		})
	})
}

func parseGoogleAudiences() []string {
	raw := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_IDS"))
	var out []string
	if raw != "" {
		for _, part := range strings.Split(raw, ",") {
			if v := strings.TrimSpace(part); v != "" {
				out = append(out, v)
			}
		}
	}
	if len(out) == 0 {
		if v := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func validateGoogleIDToken(ctx context.Context, token string, audiences []string) (*idtoken.Payload, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("token required")
	}
	var lastErr error
	for _, aud := range audiences {
		aud = strings.TrimSpace(aud)
		if aud == "" {
			continue
		}
		payload, err := idtoken.Validate(ctx, token, aud)
		if err == nil {
			return payload, nil
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no audiences configured")
	}
	return nil, lastErr
}

type appAuthTokenInput struct {
	JWTSecret      string
	Issuer         string
	Provider       string
	ProviderUserID string
	UserID         uint
	Email          string
}

func signAppAuthToken(input appAuthTokenInput) (string, error) {
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"sub":      strings.TrimSpace(input.ProviderUserID),
		"userId":   input.UserID,
		"email":    strings.TrimSpace(input.Email),
		"exp":      now.Add(authAppTokenTTL).Unix(),
		"iss":      strings.TrimSpace(input.Issuer),
		"provider": strings.TrimSpace(input.Provider),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(strings.TrimSpace(input.JWTSecret)))
}

type phoneCompletionTokenInput struct {
	JWTSecret      string
	Issuer         string
	Provider       string
	ProviderUserID string
	UserID         uint
}

func signPhoneCompletionToken(input phoneCompletionTokenInput) (string, error) {
	now := time.Now().UTC()
	claims := phoneCompletionClaims{
		UserID:   input.UserID,
		Provider: strings.TrimSpace(input.Provider),
		Purpose:  authPhoneTokenPurpose,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strings.TrimSpace(input.ProviderUserID),
			Issuer:    strings.TrimSpace(input.Issuer),
			ExpiresAt: jwt.NewNumericDate(now.Add(authPhoneTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			ID:        uuid.NewString(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(strings.TrimSpace(input.JWTSecret)))
}

func parsePhoneCompletionClaims(tokenString string, jwtSecret string) (*phoneCompletionClaims, error) {
	tokenString = strings.TrimSpace(tokenString)
	if tokenString == "" {
		return nil, fmt.Errorf("missing phone token")
	}

	claims := &phoneCompletionClaims{}
	secretBytes := []byte(strings.TrimSpace(jwtSecret))
	token, err := jwt.ParseWithClaims(tokenString, claims, func(parsedToken *jwt.Token) (interface{}, error) {
		if _, ok := parsedToken.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return secretBytes, nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	if claims.Purpose != authPhoneTokenPurpose {
		return nil, fmt.Errorf("invalid token purpose")
	}
	if claims.UserID == 0 {
		return nil, fmt.Errorf("invalid user")
	}
	if strings.TrimSpace(claims.Subject) == "" {
		return nil, fmt.Errorf("invalid subject")
	}
	if strings.TrimSpace(claims.Provider) == "" {
		return nil, fmt.Errorf("invalid provider")
	}

	return claims, nil
}

func buildPhoneCountries() []PhoneCountryResponseItem {
	phoneCountries := make([]PhoneCountryResponseItem, 0, 256)
	for regionCode := range phonenumbers.GetSupportedRegions() {
		if regionCode == "001" {
			continue
		}

		callingCode := phonenumbers.GetCountryCodeForRegion(regionCode)
		if callingCode <= 0 {
			continue
		}

		phoneCountries = append(phoneCountries, PhoneCountryResponseItem{
			RegionCode: regionCode,
			DialCode:   fmt.Sprintf("+%d", callingCode),
		})
	}

	sort.Slice(phoneCountries, func(i int, j int) bool {
		if phoneCountries[i].DialCode == phoneCountries[j].DialCode {
			return phoneCountries[i].RegionCode < phoneCountries[j].RegionCode
		}
		return phoneCountries[i].DialCode < phoneCountries[j].DialCode
	})
	return phoneCountries
}

func validatePhoneFromInput(req CompletePhoneRequest) (validatedPhone, error) {
	countryCode := strings.ToUpper(strings.TrimSpace(req.CountryCode))
	nationalNumber := normalizePhoneNationalNumber(req.NationalNumber)
	if countryCode == "" {
		return validatedPhone{}, fmt.Errorf("country code is required")
	}
	if nationalNumber == "" {
		return validatedPhone{}, fmt.Errorf("phone number is required")
	}
	if !phonenumbers.GetSupportedRegions()[countryCode] {
		return validatedPhone{}, fmt.Errorf("country not supported")
	}
	if countryCode == "IN" && len(nationalNumber) != 10 {
		return validatedPhone{}, fmt.Errorf("indian phone numbers must be exactly 10 digits")
	}

	parsedNumber, err := phonenumbers.Parse(nationalNumber, countryCode)
	if err != nil {
		return validatedPhone{}, fmt.Errorf("invalid phone number")
	}
	if !phonenumbers.IsValidNumberForRegion(parsedNumber, countryCode) {
		return validatedPhone{}, fmt.Errorf("invalid phone number")
	}

	parsedRegion := phonenumbers.GetRegionCodeForNumber(parsedNumber)
	if strings.ToUpper(strings.TrimSpace(parsedRegion)) != countryCode {
		return validatedPhone{}, fmt.Errorf("phone number does not match selected country")
	}

	return validatedPhone{
		CountryCode:    countryCode,
		NationalNumber: nationalNumber,
		E164:           phonenumbers.Format(parsedNumber, phonenumbers.E164),
	}, nil
}

func normalizePhoneNationalNumber(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}

	builder := strings.Builder{}
	for _, r := range trimmed {
		if r >= '0' && r <= '9' {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

func buildAuthUserPayload(user *stores.UserModel) gin.H {
	if user == nil {
		return gin.H{}
	}
	return gin.H{
		"id":                 user.ID,
		"name":               user.Name,
		"email":              user.Email,
		"phone_country_code": user.PhoneCountryCode,
		"phone_e164":         user.PhoneE164,
	}
}

func userNeedsPhoneCompletion(user *stores.UserModel) bool {
	if user == nil {
		return true
	}
	return strings.TrimSpace(user.PhoneE164) == ""
}

func errorsIsRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
