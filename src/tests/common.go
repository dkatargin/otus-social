package tests

import (
	"fmt"
	"os"
	"social/config"
	"social/db"
	"social/models"
	"social/services"
	"testing"
)

const ApiBaseUrl = "http://localhost:8080"

// SetupFeedTestDB инициализирует тестовую базу данных для тестов фида и диалогов
func SetupFeedTestDB() error {
	// Загружаем тестовую конфигурацию
	configPath := "config/test.yaml"
	// Проверяем, можем ли мы найти файл в текущей директории или на уровень выше
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		configPath = "../config/test.yaml"
	}

	err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load test config: %w", err)
	}

	// Подключаемся к тестовой базе данных PostgreSQL
	err = db.ConnectDB()
	if err != nil {
		return fmt.Errorf("failed to connect to test database: %w", err)
	}

	// Инициализируем Redis для тестов
	err = services.InitRedis()
	if err != nil {
		fmt.Printf("Warning: Redis initialization failed: %v\n", err)
		// Продолжаем тесты без Redis
	}

	// Примечание: Отключаем очистку таблиц для демонстрации работы с PostgreSQL
	// В продакшене можно включить правильную очистку таблиц

	return nil
}

// SetupTestRedis заглушка для Redis (в тестах не используем реальный Redis)
func SetupTestRedis() error {
	// В тестах Redis не нужен или используем mock
	return nil
}

// createTestUser создает тестового пользователя и возвращает токен и ID
func createTestUser(t *testing.T, nickname, firstName string) (string, int64) {
	// Инициализируем БД если она еще не инициализирована
	if db.ORM == nil {
		if err := SetupFeedTestDB(); err != nil {
			t.Fatalf("Failed to setup test database: %v", err)
		}
	}

	user := models.User{
		Nickname:  nickname,
		FirstName: firstName,
		LastName:  "TestUser",
		Password:  "testpassword",
		Sex:       models.MALE,
	}

	err := db.ORM.Create(&user).Error
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	// В реальном приложении здесь должен быть реальный токен
	// Для тестов используем простую строку
	token := fmt.Sprintf("test_token_%d", user.ID)

	return token, user.ID
}
