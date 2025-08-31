package middleware

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// TestAuthMiddleware - middleware для тестовой аутентификации через X-User-ID заголовок
func TestAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDHeader := c.GetHeader("X-User-ID")
		if userIDHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "X-User-ID header is required"})
			c.Abort()
			return
		}

		userID, err := strconv.ParseInt(userIDHeader, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid X-User-ID format"})
			c.Abort()
			return
		}

		c.Set("user_id", userID)
		c.Next()
	}
}

// OptionalAuthMiddleware - middleware для опциональной аутентификации
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userIDHeader := c.GetHeader("X-User-ID")
		if userIDHeader != "" {
			if userID, err := strconv.ParseInt(userIDHeader, 10, 64); err == nil {
				c.Set("user_id", userID)
			}
		}
		c.Next()
	}
}
