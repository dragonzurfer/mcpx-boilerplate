package middleware

import (
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/mcpx/boilerplate/core"
	"gorm.io/gorm"
)

const (
	ContextUserKey  = "currentUser"
	ContextTokenKey = "bearerToken"
)

type Claims struct {
	UserID   uint   `json:"userId,omitempty"`
	Email    string `json:"email,omitempty"`
	Provider string `json:"provider,omitempty"`
	jwt.RegisteredClaims
}

// Auth verifies our signed JWT and attaches the user to context.
func Auth(store *core.Store, logger *log.Logger) gin.HandlerFunc {
	secret := strings.TrimSpace(os.Getenv("JWT_SECRET"))
	if strings.TrimSpace(secret) == "" {
		logger.Printf("error [auth]: JWT_SECRET missing")
		return func(c *gin.Context) {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "server misconfigured"})
		}
	}
	secretBytes := []byte(secret)
	issuer := strings.TrimSpace(os.Getenv("JWT_ISSUER"))
	if issuer == "" {
		issuer = "mcpx"
	}

	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing Authorization header"})
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid Authorization header"})
			return
		}
		tokenString := strings.TrimSpace(parts[1])
		if tokenString == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "empty token"})
			return
		}

		claims := Claims{}
		token, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, errors.New("unexpected signing method")
			}
			return secretBytes, nil
		})
		if err != nil {
			logger.Printf("warn [auth]: token verification failed: %v", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}
		if !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		if strings.TrimSpace(claims.Subject) == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		if claims.Issuer != "" && claims.Issuer != issuer {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		user, err := store.GetUserByIdentity(claims.Subject)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "user not found"})
				return
			}
			logger.Printf("error [auth]: failed to load user: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "failed to load user"})
			return
		}

		c.Set(ContextUserKey, user)
		c.Set(ContextTokenKey, tokenString)
		c.Next()
	}
}

// CurrentUser fetches the authenticated user from context.
func CurrentUser(c *gin.Context) (*core.UserModel, bool) {
	val, ok := c.Get(ContextUserKey)
	if !ok {
		return nil, false
	}
	user, ok := val.(*core.UserModel)
	return user, ok
}
