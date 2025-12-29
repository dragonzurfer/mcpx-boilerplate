package routes

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/mcpx/boilerplate/core"
	"google.golang.org/api/idtoken"
)

type GoogleLoginRequest struct {
	GoogleToken string `json:"googleToken" binding:"required"`
}

// RegisterAuth registers the /api/auth/login route to exchange Google ID tokens for app tokens.
func RegisterAuth(rg *gin.RouterGroup, store *core.Store, logf func(string, ...interface{})) {
	audiences := parseGoogleAudiences()
	jwtSecret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	issuer := strings.TrimSpace(os.Getenv("JWT_ISSUER"))
	if issuer == "" {
		issuer = "mcpx"
	}

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

		user, err := store.GetOrCreateUser(payload.Subject, name, "google", email, payload.Claims)
		if err != nil {
			logf("error [auth/login]: persist user: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist user"})
			return
		}

		claims := jwt.MapClaims{
			"sub":      user.IdentityUID,
			"userId":   user.ID,
			"email":    user.Email,
			"exp":      time.Now().Add(365 * 24 * time.Hour).Unix(),
			"iss":      issuer,
			"provider": "google",
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, err := token.SignedString([]byte(jwtSecret))
		if err != nil {
			logf("error [auth/login]: sign token: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to issue token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"token": signed,
			"user": gin.H{
				"id":    user.ID,
				"name":  user.DisplayName,
				"email": user.Email,
			},
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
