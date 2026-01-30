package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mcpx/boilerplate/stores"
)

func RequireAdminRole() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := CurrentUser(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		role := strings.ToUpper(strings.TrimSpace(user.Role))
		if role != stores.UserRoleAdmin && role != stores.UserRoleSuperAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "admin required"})
			return
		}

		c.Next()
	}
}
