package handlers

import (
	"net/http"
	"social/services"
	"strconv"

	"github.com/gin-gonic/gin"
)

var postService = services.NewPostService()

// CreatePost создает новый пост
func CreatePost(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Получаем ID пользователя из контекста (предполагается, что он установлен middleware)
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	post, err := postService.CreatePost(c.Request.Context(), userID.(int64), req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create post"})
		return
	}

	c.JSON(http.StatusCreated, post)
}

// GetFeed получает ленту постов друзей
func GetFeed(c *gin.Context) {
	// Получаем ID пользователя из контекста
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Параметры пагинации
	lastIDStr := c.Query("last_id")
	limitStr := c.Query("limit")

	var lastID int64 = 0
	var limit int = 20

	if lastIDStr != "" {
		if parsed, err := strconv.ParseInt(lastIDStr, 10, 64); err == nil {
			lastID = parsed
		}
	}

	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	feed, err := postService.GetUserFeed(c.Request.Context(), userID.(int64), lastID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get feed"})
		return
	}

	c.JSON(http.StatusOK, feed)
}

// InvalidateUserFeed инвалидирует кеш ленты пользователя (админский эндпоинт)
func InvalidateUserFeed(c *gin.Context) {
	userIDStr := c.Param("user_id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	err = postService.InvalidateUserFeed(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to invalidate cache"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Cache invalidated successfully"})
}

// RebuildUserFeed перестраивает кеш ленты пользователя из БД (админский эндпоинт)
func RebuildUserFeed(c *gin.Context) {
	userIDStr := c.Param("user_id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	err = postService.RebuildUserFeedFromDB(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to rebuild feed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Feed rebuilt successfully"})
}

// RebuildAllFeeds перестраивает кеши всех лент из БД (админский эндпоинт)
func RebuildAllFeeds(c *gin.Context) {
	err := postService.RebuildAllFeeds(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to rebuild all feeds"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "All feeds rebuilt successfully"})
}

// DeletePost удаляет пост
func DeletePost(c *gin.Context) {
	postIDStr := c.Param("post_id")
	postID, err := strconv.ParseInt(postIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid post ID"})
		return
	}

	// Получаем ID пользователя из контекста
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	err = postService.DeletePost(c.Request.Context(), userID.(int64), postID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete post"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Post deleted successfully"})
}

// GetQueueStats возвращает статистику очереди (админский эндпоинт)
func GetQueueStats(c *gin.Context) {
	if services.QueueServiceInstance == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Queue service not available"})
		return
	}

	queueLength, err := services.QueueServiceInstance.GetQueueStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get queue stats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"queue_length": queueLength,
		"workers":      5, // QUEUE_WORKER_COUNT
	})
}
