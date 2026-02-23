package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	headerAPIKey = "X-API-Key"
	headerAuth   = "Authorization"
)

// APIKeyAuth middleware — если API_KEY задан, требует X-API-Key или Authorization: Bearer <key>. /health всегда доступен.
func APIKeyAuth(apiKey string) gin.HandlerFunc {
	if apiKey == "" {
		return func(c *gin.Context) { c.Next() }
	}
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/health" {
			c.Next()
			return
		}
		key := c.GetHeader(headerAPIKey)
		if key == "" {
			if auth := c.GetHeader(headerAuth); strings.HasPrefix(auth, "Bearer ") {
				key = strings.TrimPrefix(auth, "Bearer ")
			}
		}
		if key != apiKey {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}
