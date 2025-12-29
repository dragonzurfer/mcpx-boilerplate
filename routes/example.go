package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func RegisterExample(rg *gin.RouterGroup) {
	rg.GET("/example/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "pong"})
	})
}
