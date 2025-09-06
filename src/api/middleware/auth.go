package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// TestAuthMiddleware - middleware для тестовой аутентификации
// Поддерживает два варианта:
// 1. X-User-ID заголовок (для простых тестов)
// 2. Authorization: Bearer test_token_N (для интеграционных тестов)
func TestAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Сначала проверяем X-User-ID заголовок
		userIDHeader := c.GetHeader("X-User-ID")
		if userIDHeader != "" {
			userID, err := strconv.ParseInt(userIDHeader, 10, 64)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid X-User-ID format"})
				c.Abort()
				return
			}
			c.Set("user_id", userID)
			c.Next()
			return
		}

		// Затем проверяем Authorization Bearer token
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")

			// Парсим тестовые токены вида test_token_N
			if strings.HasPrefix(token, "test_token_") {
				userIDStr := strings.TrimPrefix(token, "test_token_")
				userID, err := strconv.ParseInt(userIDStr, 10, 64)
				if err != nil {
					c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid test token format"})
					c.Abort()
					return
				}
				c.Set("user_id", userID)
				c.Next()
				return
			}
		}

		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required: provide X-User-ID header or Authorization Bearer token"})
		c.Abort()
	}
}

// OptionalAuthMiddleware - middleware для опциональной аутентификации
func OptionalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Проверяем X-User-ID заголовок
		userIDHeader := c.GetHeader("X-User-ID")
		if userIDHeader != "" {
			if userID, err := strconv.ParseInt(userIDHeader, 10, 64); err == nil {
				c.Set("user_id", userID)
				c.Next()
				return
			}
		}

		// Проверяем Authorization Bearer token
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			token := strings.TrimPrefix(authHeader, "Bearer ")
			if strings.HasPrefix(token, "test_token_") {
				userIDStr := strings.TrimPrefix(token, "test_token_")
				if userID, err := strconv.ParseInt(userIDStr, 10, 64); err == nil {
					c.Set("user_id", userID)
				}
			}
		}

		c.Next()
	}
}
