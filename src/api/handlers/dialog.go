package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"social/db"
	"social/models"
)

const NumShards = 4 // Можно вынести в конфиг

// getShardID возвращает номер шарда для пользователя
func getShardID(userID int64) int {
	var shardMap models.ShardMap
	if err := db.ORM.Where("user_id = ?", userID).First(&shardMap).Error; err == nil {
		return shardMap.ShardID
	}
	// По умолчанию - hash
	return int(userID % NumShards)
}

// SendMessageHandler - отправка сообщения пользователю
func SendMessageHandler(c *gin.Context) {
	fromUserID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	toUserIDStr := c.Param("user_id")
	toUserID, err := strconv.ParseInt(toUserIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}
	var req struct {
		Text string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}
	shardID := getShardID(toUserID)
	tableName := "messages_" + strconv.Itoa(shardID)
	msg := models.Message{
		FromUserID: fromUserID.(int64),
		ToUserID:   toUserID,
		Text:       req.Text,
		CreatedAt:  time.Now(),
		IsRead:     false,
	}
	if err := db.ORM.Table(tableName).Create(&msg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send message"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Message sent"})
}

// ListDialogHandler - получение сообщений между пользователями (диалога)
func ListDialogHandler(c *gin.Context) {
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
	// Получаем shard для обоих пользователей (ищем в обоих шардах)
	shardIDs := []int{getShardID(userID.(int64)), getShardID(otherUserID)}
	var messages []models.Message
	for _, shardID := range shardIDs {
		tableName := "messages_" + strconv.Itoa(shardID)
		var part []models.Message
		db.ORM.Table(tableName).
			Where("(from_user_id = ? AND to_user_id = ?) OR (from_user_id = ? AND to_user_id = ?)", userID, otherUserID, otherUserID, userID).
			Order("created_at ASC").
			Find(&part)
		messages = append(messages, part...)
	}
	// Сортируем по времени (если сообщения из разных шардов)
	// (можно оптимизировать, если гарантировать хранение всех сообщений пары в одном шарде)
	// ...
	c.JSON(http.StatusOK, gin.H{"messages": messages})
}
