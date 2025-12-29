package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	ContextAPIKeyHash  = "apiKeyHash"
	ContextAPIKeyLabel = "apiKeyLabel"
)

func parseAPIKeys() map[string]string {
	raw := strings.TrimSpace(os.Getenv("API_KEYS"))
	if raw == "" {
		return map[string]string{}
	}
	out := map[string]string{}
	parts := strings.Split(raw, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		label := "default"
		key := part
		if strings.Contains(part, ":") {
			items := strings.SplitN(part, ":", 2)
			if strings.TrimSpace(items[0]) != "" {
				label = strings.TrimSpace(items[0])
			}
			key = strings.TrimSpace(items[1])
		}
		if key == "" {
			continue
		}
		hash := sha256.Sum256([]byte(key))
		out[hex.EncodeToString(hash[:])] = label
	}
	return out
}

func APIKey(required bool) gin.HandlerFunc {
	keys := parseAPIKeys()
	return func(c *gin.Context) {
		if len(keys) == 0 {
			if required {
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "api key auth not configured"})
			}
			return
		}

		key := strings.TrimSpace(c.GetHeader("X-API-Key"))
		if key == "" {
			if required {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing api key"})
			}
			return
		}
		hash := sha256.Sum256([]byte(key))
		hashHex := hex.EncodeToString(hash[:])
		label, ok := keys[hashHex]
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid api key"})
			return
		}
		c.Set(ContextAPIKeyHash, hashHex)
		c.Set(ContextAPIKeyLabel, label)
		c.Next()
	}
}

func RequireAPIKey() gin.HandlerFunc {
	return APIKey(true)
}

func OptionalAPIKey() gin.HandlerFunc {
	return APIKey(false)
}

func CurrentAPIKey(c *gin.Context) (hash string, label string, ok bool) {
	h, ok := c.Get(ContextAPIKeyHash)
	if !ok {
		return "", "", false
	}
	l, _ := c.Get(ContextAPIKeyLabel)
	hash, _ = h.(string)
	label, _ = l.(string)
	return hash, label, hash != ""
}
