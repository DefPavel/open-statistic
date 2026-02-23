package api

import (
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders добавляет стандартные security headers
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Next()
	}
}

// ValidatePath проверяет, что path внутри allowedDir (защита от path traversal)
func ValidatePath(path, allowedDir string) bool {
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	allowed, err := filepath.Abs(allowedDir)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(allowed, abs)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
