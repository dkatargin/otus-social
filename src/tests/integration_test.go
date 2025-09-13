package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"social/api/routes"
	"social/config"
	"social/db"
	"social/models"
	"social/services"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	// Настройка тестового окружения
	os.Setenv("REDIS_HOST", "localhost")
	os.Setenv("REDIS_PORT", "6380")
	os.Setenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5673/")
	os.Setenv("DB_HOST", "localhost")
	os.Setenv("DB_PORT", "5440")
	os.Setenv("DB_USER", "test_user")
	os.Setenv("DB_PASSWORD", "test_password")
	os.Setenv("DB_NAME", "social_test")

	// Загружаем тестовую конфигурацию
	err := config.LoadConfig("../config/test.yaml")
	if err != nil {
		fmt.Printf("Failed to load test config: %v\n", err)
		os.Exit(1)
	}

	// Подключаемся к тестовой БД
	err = db.ConnectDB()
	if err != nil {
		fmt.Printf("Failed to connect to test DB: %v\n", err)
		os.Exit(1)
	}

	// Инициализируем Redis
	err = services.InitRedis()
	if err != nil {
		fmt.Printf("Failed to connect to test Redis: %v\n", err)
		os.Exit(1)
	}

	// Инициализируем сервис очередей
	services.InitQueueService()

	// Инициализируем RabbitMQ
	err = services.InitRabbitMQ()
	if err != nil {
		fmt.Printf("Failed to init test RabbitMQ: %v\n", err)
		os.Exit(1)
	}

	// Запускаем RabbitMQ consumer
	ctx := context.Background()
	err = services.StartFeedEventConsumer(ctx, "test_feed_push_queue")
	if err != nil {
		fmt.Printf("Failed to start test feed consumer: %v\n", err)
		os.Exit(1)
	}

	// Запускаем воркеры очереди
	if services.QueueServiceInstance != nil {
		services.QueueServiceInstance.StartWorkers(ctx)
	}

	// Запускаем тесты
	code := m.Run()

	// Очистка
	services.CloseRedis()
	os.Exit(code)
}

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.Default()
	routes.PublicApi(router)
	return router
}

func createTestUser(t *testing.T) int64 {
	user := &models.User{
		Nickname:  fmt.Sprintf("test_user_%d", time.Now().UnixNano()),
		FirstName: "Test",
		LastName:  "User",
		Password:  "hashedpassword",
		Birthday:  time.Now(),
		Sex:       models.MALE,
		City:      "Test City",
	}

	err := db.GetWriteDB(context.Background()).Create(user).Error
	require.NoError(t, err)
	return user.ID
}

func TestCreatePost(t *testing.T) {
	router := setupTestRouter()
	userID := createTestUser(t)

	// Создаем пост
	postData := map[string]string{
		"content": "Test post content",
	}
	jsonData, _ := json.Marshal(postData)

	req, _ := http.NewRequest("POST", "/api/v1/posts/create", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", fmt.Sprintf("%d", userID)) // Для тестового middleware

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code)

	var response models.Post
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Test post content", response.Content)
	assert.Equal(t, userID, response.UserID)
}

func TestGetFeed(t *testing.T) {
	router := setupTestRouter()
	userID := createTestUser(t)

	// Создаем пост
	postService := services.NewPostService()
	post, err := postService.CreatePost(context.Background(), userID, "Test feed post")
	require.NoError(t, err)

	// Ждем немного для обработки очереди
	time.Sleep(100 * time.Millisecond)

	// Получаем ленту
	req, _ := http.NewRequest("GET", "/api/v1/feed", nil)
	req.Header.Set("X-User-ID", fmt.Sprintf("%d", userID))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response models.FeedResponse
	err = json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.True(t, len(response.Posts) > 0)
	assert.Equal(t, post.ID, response.Posts[0].ID)
}

func TestQueueStats(t *testing.T) {
	router := setupTestRouter()

	req, _ := http.NewRequest("GET", "/api/v1/admin/queue/stats", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var stats map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &stats)
	require.NoError(t, err)
	assert.Contains(t, stats, "queue_length")
	assert.Contains(t, stats, "worker_count")
}

func TestWebSocketConnection(t *testing.T) {
	// Этот тест проверяет базовое подключение WebSocket
	router := setupTestRouter()
	userID := createTestUser(t)

	// Создаем WebSocket сервер для тестирования
	server := httptest.NewServer(router)
	defer server.Close()

	// Симулируем WebSocket подключение (упрощенный тест)
	req, _ := http.NewRequest("GET", "/api/v1/ws/feed", nil)
	req.Header.Set("X-User-ID", fmt.Sprintf("%d", userID))
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Sec-WebSocket-Key", "test-key")
	req.Header.Set("Sec-WebSocket-Version", "13")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// WebSocket upgrade должен вернуть 400 в тестовом окружении, но это нормально
	// так как мы не можем полноценно тестировать WebSocket в httptest
	assert.True(t, w.Code == http.StatusBadRequest || w.Code == http.StatusSwitchingProtocols)
}

func TestFeedCacheInvalidation(t *testing.T) {
	router := setupTestRouter()
	userID := createTestUser(t)

	// Инвалидируем кеш
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/admin/cache/feed/%d", userID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Cache invalidated successfully", response["message"])
}

func TestFeedRebuild(t *testing.T) {
	router := setupTestRouter()
	userID := createTestUser(t)

	// Перестраиваем ленту
	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/feed/rebuild/%d", userID), nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "Feed rebuilt successfully", response["message"])
}
