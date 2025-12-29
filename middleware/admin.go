package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

func RequireAdminToken() gin.HandlerFunc {
	secret := strings.TrimSpace(os.Getenv("ADMIN_TOKEN"))
	return func(c *gin.Context) {
		if secret == "" {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "admin token not configured"})
			return
		}
		incoming := strings.TrimSpace(c.GetHeader("X-Admin-Token"))
		if incoming == "" || incoming != secret {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid admin token"})
			return
		}
		c.Next()
	}
}
