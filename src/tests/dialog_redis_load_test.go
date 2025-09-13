package tests

import (
	"fmt"
	"social/services"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestRedisDialogBasic проводит базовое тестирование Redis диалогов
func TestRedisDialogBasic(t *testing.T) {
	// Создаем Redis сервис для диалогов
	redisService := services.NewRedisDialogService("localhost:6380", "", 0)
	defer redisService.Close()

	require.NoError(t, nil)

	// Тестируем отправку сообщения
	message, err := redisService.SendMessage(1, 2, "Hello from Redis!")
	require.NoError(t, err)
	require.NotNil(t, message)

	// Тестируем получение сообщений
	messages, err := redisService.GetMessages(1, 2, 0, 10)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Equal(t, "Hello from Redis!", messages[0].Text)
	require.Equal(t, int64(1), messages[0].FromUserID)
	require.Equal(t, int64(2), messages[0].ToUserID)

	// Тестируем статистику диалога
	stats, err := redisService.GetDialogStats(1, 2, 2)
	require.NoError(t, err)
	require.Equal(t, int64(1), stats.TotalMessages)
	require.Equal(t, int64(1), stats.UnreadCount)

	// Отправляем еще одно сообщение
	message2, err := redisService.SendMessage(2, 1, "Reply from Redis!")
	require.NoError(t, err)
	require.NotNil(t, message2)

	// Проверяем обновленную статистику
	stats, err = redisService.GetDialogStats(1, 2, 1)
	require.NoError(t, err)
	require.Equal(t, int64(2), stats.TotalMessages)
	require.Equal(t, int64(1), stats.UnreadCount) // Только одно непрочитанное для пользователя 1

	// Помечаем сообщения как прочитанные
	updatedCount, err := redisService.MarkAsRead(1, 2, 1)
	require.NoError(t, err)
	require.Equal(t, 1, updatedCount)

	// Проверяем статистику после прочтения
	stats, err = redisService.GetDialogStats(1, 2, 1)
	require.NoError(t, err)
	require.Equal(t, int64(2), stats.TotalMessages)
	require.Equal(t, int64(0), stats.UnreadCount)

	fmt.Printf("Redis dialog functionality test passed!\n")
}
