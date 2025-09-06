package tests

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"social/db"
	"social/models"
	"social/services"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const ApiBaseUrl = "http://localhost:8080"

var TestRedisClient *redis.Client

func SetupFeedTestDB() error {
	// Инициализируем тестовую базу данных SQLite в памяти
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		return err
	}
	// Автомиграция всех моделей включая Post, Message, ShardMap
	err = database.AutoMigrate(&models.User{}, &models.Friend{}, &models.Post{}, &models.ShardMap{}, &models.Message{})
	if err != nil {
		return err
	}
	// Устанавливаем глобальную переменную ORM
	db.ORM = database
	return nil
}

func SetupTestRedis() {
	// Настраиваем тестовый Redis клиент
	TestRedisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Используем тестовую БД
	})
	// Очищаем тестовую БД
	TestRedisClient.FlushDB(context.Background())
	// Устанавливаем глобальный клиент для тестов
	services.RedisClient = TestRedisClient

	// НЕ инициализируем QueueServiceInstance в тестах, чтобы использовать fallback путь
	services.QueueServiceInstance = nil

	// Инициализируем RabbitMQ для тестов
	os.Setenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	if err := services.InitRabbitMQ(); err != nil {
		// Если RabbitMQ недоступен, продолжаем без него
		fmt.Printf("Warning: RabbitMQ not available for tests: %v\n", err)
	} else {
		// Запускаем консьюмер в фоновом режиме
		go func() {
			ctx := context.Background()
			if err := services.StartFeedEventConsumer(ctx, "test_feed_queue"); err != nil {
				fmt.Printf("Warning: Failed to start feed consumer: %v\n", err)
			}
		}()
		// Даем консьюмеру время на запуск
		time.Sleep(100 * time.Millisecond)
	}
}

func CreateTestUser(t *testing.T, firstName, lastName string) *models.User {
	user := &models.User{
		FirstName: firstName,
		LastName:  lastName,
		Nickname:  fmt.Sprintf("user_%d", time.Now().UnixNano()),
		Birthday:  time.Now().AddDate(-25, 0, 0),
		Sex:       "male",
		City:      "Test City",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	err := db.ORM.Create(user).Error
	require.NoError(t, err)
	return user
}

func CreateFriendship(t *testing.T, userID, friendID int64) {
	friendship := &models.Friend{
		UserID:     userID,
		FriendID:   friendID,
		Status:     "approved",
		CreatedAt:  time.Now(),
		ApprovedAt: time.Now(),
	}
	err := db.ORM.Create(friendship).Error
	require.NoError(t, err)
}
