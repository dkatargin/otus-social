package handlers

import (
	"net/http"
	"strconv"
	"time"

	"social/db"
	"social/models"

	"github.com/gin-gonic/gin"
)

const NumShards = 4 // Можно вынести в конфиг

// SendMessageRequest структура для запроса отправки сообщения
type SendMessageRequest struct {
	To   int64  `json:"to" binding:"required"`
	Text string `json:"text" binding:"required"`
}

// getShardID возвращает номер шарда для пользователя
// Использует детерминированную схему шардирования для обеспечения
// того, что все сообщения пары пользователей находятся в одном шарде
func getShardID(userID1, userID2 int64) int {
	// Определяем меньший и больший ID для детерминированности
	minID := userID1
	maxID := userID2
	if userID1 > userID2 {
		minID = userID2
		maxID = userID1
	}

	// Проверяем, есть ли явное маппирование в shard_map для меньшего ID
	var shardMap models.ShardMap
	if err := db.ORM.Where("user_id = ?", minID).First(&shardMap).Error; err == nil {
		return shardMap.ShardID
	}

	// Также проверяем большего пользователя для случаев решардинга
	if err := db.ORM.Where("user_id = ?", maxID).First(&shardMap).Error; err == nil {
		return shardMap.ShardID
	}

	// Улучшенный алгоритм хеширования для лучшего распределения
	// Используем простую хеш-функцию с лучшим распределением
	hash := uint64(minID)*2654435761 + uint64(maxID)*2654435789
	hash = hash ^ (hash >> 16)
	hash = hash * 2654435761
	hash = hash ^ (hash >> 16)

	return int(hash % uint64(NumShards))
}

// reassignUserToShard перемещает пользователя в указанный шард
// Это поддерживает решардинг для "эффекта Леди Гаги"
func reassignUserToShard(userID int64, newShardID int) error {
	shardMap := models.ShardMap{
		UserID:  userID,
		ShardID: newShardID,
	}

	return db.ORM.Save(&shardMap).Error
}

// SendMessageHandler - отправка сообщения пользователю
func SendMessageHandler(c *gin.Context) {
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
	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	// Проверяем соответствие ID в URL и теле запроса
	if req.To != toUserID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Recipient ID mismatch"})
		return
	}

	// Определяем шард на основе пары пользователей
	shardID := getShardID(fromUserID.(int64), toUserID)
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

	// Определяем шард для пары пользователей
	shardID := getShardID(userID.(int64), otherUserID)
	tableName := "messages_" + strconv.Itoa(shardID)

	var messages []models.Message

	// Получаем сообщения из соответствующего шарда
	result := db.ORM.Table(tableName).
		Where("(from_user_id = ? AND to_user_id = ?) OR (from_user_id = ? AND to_user_id = ?)",
			userID, otherUserID, otherUserID, userID).
		Order("created_at ASC").
		Limit(limit).
		Offset(offset).
		Find(&messages)

	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve messages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

// GetUserMessageStatsHandler - получение статистики сообщений пользователя
// Может использоваться для выявления "эффекта Леди Гаги"
func GetUserMessageStatsHandler(c *gin.Context) {
	userIDStr := c.Param("user_id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	type ShardStats struct {
		ShardID      int   `json:"shard_id"`
		MessageCount int64 `json:"message_count"`
	}

	var stats []ShardStats

	// Проверяем все шарды для подсчета сообщений пользователя
	for i := 0; i < NumShards; i++ {
		tableName := "messages_" + strconv.Itoa(i)
		var count int64

		db.ORM.Table(tableName).
			Where("from_user_id = ? OR to_user_id = ?", userID, userID).
			Count(&count)

		if count > 0 {
			stats = append(stats, ShardStats{
				ShardID:      i,
				MessageCount: count,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"user_id": userID,
		"stats":   stats,
	})
}

// ReshardUserHandler - перемещение пользователя в другой шард
// Используется для борьбы с "эффектом Леди Гаги"
func ReshardUserHandler(c *gin.Context) {
	userIDStr := c.Param("user_id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user_id"})
		return
	}

	type ReshardRequest struct {
		NewShardID int `json:"new_shard_id" binding:"required,min=0"`
	}

	var req ReshardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if req.NewShardID >= NumShards {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid shard_id"})
		return
	}

	if err := reassignUserToShard(userID, req.NewShardID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reassign user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User reassigned to new shard"})
}
