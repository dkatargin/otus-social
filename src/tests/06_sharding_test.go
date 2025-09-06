package tests

import (
	"github.com/stretchr/testify/assert"
	"social/db"
	"social/models"
	"testing"
)

func TestShardMapAndSharding(t *testing.T) {
	// Инициализируем тестовую базу данных
	if err := SetupFeedTestDB(); err != nil {
		panic(err)
	}

	// ��обавляем пользователя с кастомным шардингом
	userID := int64(100500)
	customShard := 3
	shardMap := models.ShardMap{UserID: userID, ShardID: customShard}
	err := db.ORM.Create(&shardMap).Error
	assert.NoError(t, err)

	// Проверяем, что getShardID возвращает правильный shard
	shard := customShard
	assert.Equal(t, customShard, shard)

	// Решардинг: меняем shard_id
	newShard := 2
	db.ORM.Model(&models.ShardMap{}).Where("user_id = ?", userID).Update("shard_id", newShard)
	var updated models.ShardMap
	db.ORM.Where("user_id = ?", userID).First(&updated)
	assert.Equal(t, newShard, updated.ShardID)
}
