package tests

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"social/db"
	"social/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// getShardID возвращает номер шарда для пользователя (копия из handlers)
func getShardIDForTest(userID1, userID2 int64) int {
	minID := userID1
	maxID := userID2
	if userID1 > userID2 {
		minID = userID2
		maxID = userID1
	}

	// Проверяем, есть ли явное маппирование в shard_map
	var shardMap models.ShardMap
	if err := db.ORM.Where("user_id = ?", minID).First(&shardMap).Error; err == nil {
		return shardMap.ShardID
	}

	// Улучшенный алгоритм хеширования для лучшего распределения
	hash := uint64(minID)*2654435761 + uint64(maxID)*2654435789
	hash = hash ^ (hash >> 16)
	hash = hash * 2654435761
	hash = hash ^ (hash >> 16)

	return int(hash % uint64(4)) // 4 шарда
}

// TestShardDistribution проверяет равномерное распределение сообщений по шардам
func TestShardDistribution(t *testing.T) {
	// Инициализируем тестовую базу данных
	if err := SetupFeedTestDB(); err != nil {
		panic(err)
	}

	// Очищаем таблицу shard_map для честного тестирования алгоритма хеширования
	db.ORM.Exec("DELETE FROM shard_map")

	// Очищаем все таблицы сообщений
	for i := 0; i < 4; i++ {
		db.ORM.Exec(fmt.Sprintf("DELETE FROM messages_%d", i))
	}

	// Создаем большое количество пользователей для тестирования
	numUsers := 50
	users := make([]struct {
		token string
		id    int64
	}, numUsers)

	for i := 0; i < numUsers; i++ {
		timestamp := time.Now().UnixNano()
		id, token := CreateTestUser(t, fmt.Sprintf("dist_user_%d_%d", i, timestamp), "Distribution")
		users[i] = struct {
			token string
			id    int64
		}{token, id}
	}

	t.Logf("Created %d users for distribution testing", numUsers)

	// Создаем множество уникальных пар пользователей
	type messagePair struct {
		fromIndex int
		toIndex   int
		shard     int
	}

	var pairs []messagePair
	shardExpected := make(map[int]int)

	// Генерируем пары и предсказываем их распределение по шардам
	for i := 0; i < numUsers; i++ {
		for j := i + 1; j < numUsers; j++ {
			if len(pairs) >= 200 { // Ограничиваем количество пар для разумного времени тестирования
				break
			}

			expectedShard := getShardIDForTest(users[i].id, users[j].id)
			pairs = append(pairs, messagePair{
				fromIndex: i,
				toIndex:   j,
				shard:     expectedShard,
			})
			shardExpected[expectedShard]++
		}
		if len(pairs) >= 200 {
			break
		}
	}

	t.Logf("Generated %d unique pairs", len(pairs))
	t.Logf("Expected distribution:")
	for shard := 0; shard < 4; shard++ {
		t.Logf("  Shard %d: %d pairs", shard, shardExpected[shard])
	}

	// Отправляем сообщения для каждой пары
	successCount := 0
	for i, pair := range pairs {
		from := users[pair.fromIndex]
		to := users[pair.toIndex]

		text := fmt.Sprintf("Distribution test message %d", i)
		resp, err := sendMessage(from.token, to.id, text)

		if err != nil {
			t.Logf("Error sending message %d: %v", i, err)
			continue
		}

		if resp.StatusCode == 200 {
			successCount++
		} else {
			t.Logf("Message %d failed with status %d", i, resp.StatusCode)
		}
		resp.Body.Close()

		// Небольшая пауза чтобы не перегружать систему
		if i%20 == 0 && i > 0 {
			time.Sleep(10 * time.Millisecond)
		}
	}

	t.Logf("Successfully sent %d out of %d messages", successCount, len(pairs))
	require.Greater(t, successCount, len(pairs)/2, "At least half of messages should be sent successfully")

	// Проверяем фактическое распределение по шардам
	shardActual := make(map[int]int)
	totalMessages := 0

	for shardID := 0; shardID < 4; shardID++ {
		tableName := fmt.Sprintf("messages_%d", shardID)
		var count int64
		db.ORM.Table(tableName).Count(&count)
		shardActual[shardID] = int(count)
		totalMessages += int(count)
	}

	t.Logf("Actual distribution:")
	for shard := 0; shard < 4; shard++ {
		percentage := float64(shardActual[shard]) / float64(totalMessages) * 100
		t.Logf("  Shard %d: %d messages (%.1f%%)", shard, shardActual[shard], percentage)
	}

	// Проверяем, что используется несколько шардов
	usedShards := 0
	for shard := 0; shard < 4; shard++ {
		if shardActual[shard] > 0 {
			usedShards++
		}
	}

	assert.GreaterOrEqual(t, usedShards, 3, "Should use at least 3 shards for good distribution")
	assert.Greater(t, totalMessages, 50, "Should have a reasonable number of messages")

	// Проверяем относительную равномерность распределения
	if totalMessages > 0 {
		avgMessagesPerShard := float64(totalMessages) / float64(usedShards)

		for shard := 0; shard < 4; shard++ {
			if shardActual[shard] > 0 {
				deviation := float64(shardActual[shard]) - avgMessagesPerShard
				relativeDeviation := deviation / avgMessagesPerShard

				// Отклонение не должно быть больше 100% (достаточно мягкое ограничение)
				assert.LessOrEqual(t, relativeDeviation, 1.0,
					"Shard %d has too much deviation from average (%.1f%% vs average %.1f)",
					shard, float64(shardActual[shard])/float64(totalMessages)*100, avgMessagesPerShard/float64(totalMessages)*100)
			}
		}
	}
}

// TestShardConsistency проверяет детерминированность шардирования
func TestShardConsistency(t *testing.T) {
	// Инициализируем тестовую базу данных
	if err := SetupFeedTestDB(); err != nil {
		panic(err)
	}

	// Создаем пользователей
	user1ID, user1Token := CreateTestUser(t, "consistency_user1_"+strconv.FormatInt(time.Now().UnixNano(), 10), "Consistency")
	user2ID, user2Token := CreateTestUser(t, "consistency_user2_"+strconv.FormatInt(time.Now().UnixNano(), 10), "Consistency")

	// Отправляем несколько сообщений между одной парой пользователей
	for i := 0; i < 10; i++ {
		// Сообщение от user1 к user2
		text1 := fmt.Sprintf("Consistency message %d from user1", i)
		resp1, err := sendMessage(user1Token, user2ID, text1)
		require.NoError(t, err)
		defer resp1.Body.Close()
		assert.Equal(t, 200, resp1.StatusCode)

		// Сообщение от user2 к user1
		text2 := fmt.Sprintf("Consistency message %d from user2", i)
		resp2, err := sendMessage(user2Token, user1ID, text2)
		require.NoError(t, err)
		defer resp2.Body.Close()
		assert.Equal(t, 200, resp2.StatusCode)
	}

	// Проверяем, что все сообщения находятся в одном шарде
	shardCounts := make(map[int]int)
	for shardID := 0; shardID < 4; shardID++ {
		tableName := fmt.Sprintf("messages_%d", shardID)
		var count int64

		db.ORM.Table(tableName).
			Where("(from_user_id = ? AND to_user_id = ?) OR (from_user_id = ? AND to_user_id = ?)",
				user1ID, user2ID, user2ID, user1ID).
			Count(&count)

		if count > 0 {
			shardCounts[shardID] = int(count)
			t.Logf("Shard %d has %d messages for this pair", shardID, count)
		}
	}

	// Все сообщения должны быть в одном шарде
	assert.Equal(t, 1, len(shardCounts), "All messages between two users should be in one shard")

	// Проверяем, что количество сообщений корректное
	for _, count := range shardCounts {
		assert.Equal(t, 20, count, "Should have 20 messages (10 in each direction)")
	}
}

// TestShardPerformanceWithWorkingAuth тестирует производительность с рабочей аутентификацией
func TestShardPerformanceWithWorkingAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	// Инициализируем тестовую базу данных
	if err := SetupFeedTestDB(); err != nil {
		panic(err)
	}

	// Очищаем таблицу shard_map
	db.ORM.Exec("DELETE FROM shard_map")

	// Создаем пользователей для тестирования производительности
	numUsers := 20
	users := make([]struct {
		token string
		id    int64
	}, numUsers)

	for i := 0; i < numUsers; i++ {
		timestamp := time.Now().UnixNano()
		id, token := CreateTestUser(t, fmt.Sprintf("perf_user_%d_%d", i, timestamp), "Performance")
		users[i] = struct {
			token string
			id    int64
		}{token, id}
	}

	start := time.Now()
	messageCount := 100
	successCount := 0

	// Отправляем сообщения последовательно для стабильности
	for i := 0; i < messageCount; i++ {
		fromIndex := i % len(users)
		toIndex := (i + 7) % len(users) // Используем смещение для разнообразия
		if toIndex == fromIndex {
			toIndex = (toIndex + 1) % len(users)
		}

		from := users[fromIndex]
		to := users[toIndex]

		text := fmt.Sprintf("Performance test message %d", i)
		resp, err := sendMessage(from.token, to.id, text)

		if err == nil && resp.StatusCode == 200 {
			successCount++
		}

		if resp != nil {
			resp.Body.Close()
		}

		// Небольшая пауза для стабильности
		if i%10 == 0 && i > 0 {
			time.Sleep(5 * time.Millisecond)
		}
	}

	duration := time.Since(start)
	messagesPerSecond := float64(successCount) / duration.Seconds()

	t.Logf("Successfully sent %d out of %d messages", successCount, messageCount)
	t.Logf("Performance: %.2f msg/sec", messagesPerSecond)

	// Проверяем распределение по шардам
	shardCounts := make(map[int]int)
	totalMessages := 0

	for shardID := 0; shardID < 4; shardID++ {
		tableName := fmt.Sprintf("messages_%d", shardID)
		var count int64
		db.ORM.Table(tableName).Count(&count)
		shardCounts[shardID] = int(count)
		totalMessages += int(count)
	}

	t.Logf("Final shard utilization:")
	usedShards := 0
	for shardID := 0; shardID < 4; shardID++ {
		if shardCounts[shardID] > 0 {
			usedShards++
			percentage := float64(shardCounts[shardID]) / float64(totalMessages) * 100
			t.Logf("  Shard %d: %d messages (%.1f%%)", shardID, shardCounts[shardID], percentage)
		}
	}

	// Требования к успешности
	assert.Greater(t, successCount, messageCount/2, "At least half of messages should be sent successfully")
	assert.GreaterOrEqual(t, usedShards, 2, "Should use at least 2 shards")
	assert.Greater(t, messagesPerSecond, 10.0, "Should handle at least 10 messages per second")
}
