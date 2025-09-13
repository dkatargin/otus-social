package handlers

import (
	"net/http"
	"strconv"

	"social/services"

	"github.com/gin-gonic/gin"
)

// RedisDialogHandlers содержит обработчики для Redis-диалогов
type RedisDialogHandlers struct {
	redisService *services.RedisDialogService
}

// NewRedisDialogHandlers создает новый экземпляр обработчиков Redis-диалогов
func NewRedisDialogHandlers(redisService *services.RedisDialogService) *RedisDialogHandlers {
	return &RedisDialogHandlers{
		redisService: redisService,
	}
}

// RedisSendMessageRequest структура для запроса отправки сообщения через Redis
type RedisSendMessageRequest struct {
	Text string `json:"text" binding:"required"`
}

// SendMessageHandler - отправка сообщения через Redis с UDF
func (h *RedisDialogHandlers) SendMessageHandler(c *gin.Context) {
	fromUserID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Парсим ID получателя из URL
	toUserIDStr := c.Param("user_id")
	toUserID, err := strconv.ParseInt(toUserIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	// Парсим тело запроса
	var req RedisSendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Отправляем сообщение через Redis UDF
	message, err := h.redisService.SendMessage(fromUserID.(int64), toUserID, req.Text)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send message"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Message sent",
		"data":    message,
	})
}

// ListDialogHandler - получение сообщений диалога через Redis
func (h *RedisDialogHandlers) ListDialogHandler(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	otherUserIDStr := c.Param("user_id")
	otherUserID, err := strconv.ParseInt(otherUserIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	// Парсим параметры пагинации
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100 // Ограничиваем максимальный размер страницы
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	// Получаем сообщения через Redis UDF
	messages, err := h.redisService.GetMessages(userID.(int64), otherUserID, offset, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve messages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

// GetDialogStatsHandler - получение статистики диалога через Redis
func (h *RedisDialogHandlers) GetDialogStatsHandler(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	otherUserIDStr := c.Param("user_id")
	otherUserID, err := strconv.ParseInt(otherUserIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	// Получаем статистику через Redis UDF
	stats, err := h.redisService.GetDialogStats(userID.(int64), otherUserID, userID.(int64))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get dialog stats"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id": userID,
		"stats":   stats,
	})
}

// MarkAsReadHandler - отметка сообщений как прочитанных через Redis
func (h *RedisDialogHandlers) MarkAsReadHandler(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	otherUserIDStr := c.Param("user_id")
	otherUserID, err := strconv.ParseInt(otherUserIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	// Отмечаем сообщения как прочитанные через Redis UDF
	count, err := h.redisService.MarkAsRead(userID.(int64), otherUserID, userID.(int64))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark messages as read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "Messages marked as read",
		"updated_count": count,
	})
}
