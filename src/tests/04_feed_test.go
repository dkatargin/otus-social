package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"social/api/handlers"
	"social/db"
	"social/models"
	"social/services"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestRedisClient для тестирования
var TestRedisClient *redis.Client

func setupFeedTestDB() error {
	// Инициализируем тестовую базу данных SQLite в памяти
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		return err
	}

	// Автомиграция всех моделей включая Post
	err = database.AutoMigrate(&models.User{}, &models.Friend{}, &models.Post{})
	if err != nil {
		return err
	}

	// Устанавливаем глобальную переменную ORM
	db.ORM = database
	return nil
}

func setupTestRedis() {
	// Настраиваем тестовый Redis клиент
	TestRedisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Используем тестовую БД
	})

	// Очищаем тестовую БД
	TestRedisClient.FlushDB(context.Background())

	// Устанавливаем глобальный клиент для тестов
	services.RedisClient = TestRedisClient
	services.QueueServiceInstance = services.NewQueueService()
}

func setupFeedRouter() *gin.Engine {
	// Инициализируем тестовую базу данных и Redis
	if err := setupFeedTestDB(); err != nil {
		panic(err)
	}
	setupTestRedis()

	r := gin.Default()

	// Мидлвар для установки user_id в контекст (эмуляция аутентификации)
	r.Use(func(c *gin.Context) {
		userIDHeader := c.GetHeader("X-User-ID")
		if userIDHeader != "" {
			if userID, err := strconv.ParseInt(userIDHeader, 10, 64); err == nil {
				c.Set("user_id", userID)
			}
		}
		c.Next()
	})

	// Эндпоинты для ленты
	r.POST("/api/v1/posts/create", handlers.CreatePost)
	r.DELETE("/api/v1/posts/:post_id", handlers.DeletePost)
	r.GET("/api/v1/feed", handlers.GetFeed)
	r.DELETE("/api/v1/admin/cache/feed/:user_id", handlers.InvalidateUserFeed)
	r.POST("/api/v1/admin/feed/rebuild/:user_id", handlers.RebuildUserFeed)
	r.GET("/api/v1/admin/queue/stats", handlers.GetQueueStats)

	return r
}

func createTestUser(t *testing.T, firstName, lastName string) *models.User {
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

func createFriendship(t *testing.T, userID, friendID int64) {
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

func createTestPost(t *testing.T, router *gin.Engine, userID int64, content string) *models.Post {
	postData := map[string]string{"content": content}
	jsonData, _ := json.Marshal(postData)

	req, _ := http.NewRequest("POST", "/api/v1/posts/create", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", strconv.FormatInt(userID, 10))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var post models.Post
	err := json.Unmarshal(w.Body.Bytes(), &post)
	require.NoError(t, err)

	return &post
}

func TestFeedDisplay(t *testing.T) {
	router := setupFeedRouter()

	// Создаем тестовых пользователей
	user1 := createTestUser(t, "John", "Doe")
	user2 := createTestUser(t, "Jane", "Smith")
	user3 := createTestUser(t, "Bob", "Johnson")

	// Устанавливаем дружбу между user1 и user2, user1 и user3
	createFriendship(t, user1.ID, user2.ID)
	createFriendship(t, user1.ID, user3.ID)

	// user2 и user3 создают посты
	createTestPost(t, router, user2.ID, "Пост от Jane")
	createTestPost(t, router, user3.ID, "Пост от Bob")
	createTestPost(t, router, user1.ID, "Пост от John")

	// Ждем немного для обработки очереди
	time.Sleep(100 * time.Millisecond)

	t.Run("GetFeedSuccess", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/feed", nil)
		req.Header.Set("X-User-ID", strconv.FormatInt(user1.ID, 10))

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response models.FeedResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Проверяем, что в ленте есть посты от друзей и самого пользователя
		assert.True(t, len(response.Posts) >= 2, "Лента должна содержать посты от друзей")

		// Проверяем, что посты отсортированы по времени создания (новые первыми)
		for i := 1; i < len(response.Posts); i++ {
			assert.True(t, response.Posts[i-1].CreatedAt.After(response.Posts[i].CreatedAt) ||
				response.Posts[i-1].CreatedAt.Equal(response.Posts[i].CreatedAt),
				"Посты должны быть отсортированы по времени создания")
		}

		// Проверяем структуру постов
		for _, post := range response.Posts {
			assert.NotZero(t, post.ID)
			assert.NotZero(t, post.UserID)
			assert.NotEmpty(t, post.UserName)
			assert.NotEmpty(t, post.Content)
			assert.False(t, post.CreatedAt.IsZero())
		}
	})

	t.Run("GetFeedWithPagination", func(t *testing.T) {
		// Создаем больше постов для тестирования пагинации
		for i := 0; i < 5; i++ {
			createTestPost(t, router, user2.ID, fmt.Sprintf("Дополнительный пост %d", i))
		}

		time.Sleep(100 * time.Millisecond)

		// Первая страница
		req, _ := http.NewRequest("GET", "/api/v1/feed?limit=3", nil)
		req.Header.Set("X-User-ID", strconv.FormatInt(user1.ID, 10))

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response1 models.FeedResponse
		err := json.Unmarshal(w.Body.Bytes(), &response1)
		require.NoError(t, err)

		assert.Equal(t, 3, len(response1.Posts), "Первая страница должна содержать 3 поста")
		assert.True(t, response1.HasMore, "Должны быть еще посты")

		lastID := response1.LastID
		assert.NotZero(t, lastID)

		// Вторая страница
		req2, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/feed?limit=3&last_id=%d", lastID), nil)
		req2.Header.Set("X-User-ID", strconv.FormatInt(user1.ID, 10))

		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusOK, w2.Code)

		var response2 models.FeedResponse
		err = json.Unmarshal(w2.Body.Bytes(), &response2)
		require.NoError(t, err)

		// Проверяем, что посты не дублируются
		for _, post1 := range response1.Posts {
			for _, post2 := range response2.Posts {
				assert.NotEqual(t, post1.ID, post2.ID, "Посты не должны дублироваться между страницами")
			}
		}
	})

	t.Run("GetFeedUnauthorized", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/feed", nil)
		// Не устанавливаем X-User-ID header

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("GetFeedNoFriends", func(t *testing.T) {
		// Создаем пользователя без друзей
		userNoFriends := createTestUser(t, "Lonely", "User")

		req, _ := http.NewRequest("GET", "/api/v1/feed", nil)
		req.Header.Set("X-User-ID", strconv.FormatInt(userNoFriends.ID, 10))

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response models.FeedResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Empty(t, response.Posts, "Лента пользователя без друзей должна быть пустой")
		assert.False(t, response.HasMore)
	})
}

func TestFeedCaching(t *testing.T) {
	router := setupFeedRouter()

	// Создаем тестовых пользователей
	user1 := createTestUser(t, "User", "One")
	user2 := createTestUser(t, "User", "Two")

	// Устанавливаем дружбу
	createFriendship(t, user1.ID, user2.ID)

	// Создаем пост
	createTestPost(t, router, user2.ID, "Тестовый пост для кеширования")
	time.Sleep(100 * time.Millisecond)

	t.Run("FeedCacheHit", func(t *testing.T) {
		// Первый запрос - создает кеш
		req1, _ := http.NewRequest("GET", "/api/v1/feed", nil)
		req1.Header.Set("X-User-ID", strconv.FormatInt(user1.ID, 10))

		w1 := httptest.NewRecorder()
		start1 := time.Now()
		router.ServeHTTP(w1, req1)
		duration1 := time.Since(start1)

		assert.Equal(t, http.StatusOK, w1.Code)

		// Второй запрос - должен использовать кеш (быть быстрее)
		req2, _ := http.NewRequest("GET", "/api/v1/feed", nil)
		req2.Header.Set("X-User-ID", strconv.FormatInt(user1.ID, 10))

		w2 := httptest.NewRecorder()
		start2 := time.Now()
		router.ServeHTTP(w2, req2)
		duration2 := time.Since(start2)

		assert.Equal(t, http.StatusOK, w2.Code)

		// Результаты должны быть идентичными
		assert.Equal(t, w1.Body.String(), w2.Body.String())

		// Второй запрос должен быть быстрее (использует кеш)
		// Это может не всегда работать в тестах, но в общем случае кеш быстрее
		t.Logf("First request: %v, Second request: %v", duration1, duration2)
	})

	t.Run("CacheInvalidation", func(t *testing.T) {
		// Инвалидируем кеш
		req, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/admin/cache/feed/%d", user1.ID), nil)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Проверяем, что кеш действительно очищен
		feedKey := fmt.Sprintf("user_feed:%d", user1.ID)
		exists := services.RedisClient.Exists(context.Background(), feedKey).Val()
		assert.Equal(t, int64(0), exists, "Кеш должен быть очищен")
	})

	t.Run("CacheRebuild", func(t *testing.T) {
		// Перестраиваем кеш из БД
		req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/admin/feed/rebuild/%d", user1.ID), nil)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Проверяем, что кеш создан
		feedKey := fmt.Sprintf("user_feed:%d", user1.ID)
		exists := services.RedisClient.Exists(context.Background(), feedKey).Val()
		assert.Equal(t, int64(1), exists, "Кеш должен быть создан")

		// Проверяем, что лента работает после перестройки
		feedReq, _ := http.NewRequest("GET", "/api/v1/feed", nil)
		feedReq.Header.Set("X-User-ID", strconv.FormatInt(user1.ID, 10))

		feedW := httptest.NewRecorder()
		router.ServeHTTP(feedW, feedReq)

		assert.Equal(t, http.StatusOK, feedW.Code)

		var response models.FeedResponse
		err := json.Unmarshal(feedW.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, len(response.Posts) > 0, "Лента должна содержать посты после перестройки")
	})
}

func TestPostDeletion(t *testing.T) {
	router := setupFeedRouter()

	// Создаем тестовых пользователей
	user1 := createTestUser(t, "User", "One")
	user2 := createTestUser(t, "User", "Two")

	// Устанавливаем дружбу
	createFriendship(t, user1.ID, user2.ID)

	// Создаем пост
	post := createTestPost(t, router, user2.ID, "Пост для удаления")
	time.Sleep(100 * time.Millisecond)

	t.Run("DeletePostSuccess", func(t *testing.T) {
		// Проверяем, что пост есть в ленте
		feedReq, _ := http.NewRequest("GET", "/api/v1/feed", nil)
		feedReq.Header.Set("X-User-ID", strconv.FormatInt(user1.ID, 10))

		feedW := httptest.NewRecorder()
		router.ServeHTTP(feedW, feedReq)

		var response models.FeedResponse
		json.Unmarshal(feedW.Body.Bytes(), &response)

		foundPost := false
		for _, p := range response.Posts {
			if p.ID == post.ID {
				foundPost = true
				break
			}
		}
		assert.True(t, foundPost, "Пост должен быть в ленте до удаления")

		// Удаляем пост
		deleteReq, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/posts/%d", post.ID), nil)
		deleteReq.Header.Set("X-User-ID", strconv.FormatInt(user2.ID, 10))

		deleteW := httptest.NewRecorder()
		router.ServeHTTP(deleteW, deleteReq)

		assert.Equal(t, http.StatusOK, deleteW.Code)

		// Ждем обработки очереди удаления
		time.Sleep(200 * time.Millisecond)

		// Проверяем, что поста нет в ленте
		feedReq2, _ := http.NewRequest("GET", "/api/v1/feed", nil)
		feedReq2.Header.Set("X-User-ID", strconv.FormatInt(user1.ID, 10))

		feedW2 := httptest.NewRecorder()
		router.ServeHTTP(feedW2, feedReq2)

		var response2 models.FeedResponse
		json.Unmarshal(feedW2.Body.Bytes(), &response2)

		foundPostAfterDelete := false
		for _, p := range response2.Posts {
			if p.ID == post.ID {
				foundPostAfterDelete = true
				break
			}
		}
		assert.False(t, foundPostAfterDelete, "Пост не должен быть в ленте после удаления")
	})

	t.Run("DeletePostUnauthorized", func(t *testing.T) {
		// Создаем еще один пост
		post2 := createTestPost(t, router, user2.ID, "Еще один пост")

		// Пытаемся удалить пост другого пользователя
		deleteReq, _ := http.NewRequest("DELETE", fmt.Sprintf("/api/v1/posts/%d", post2.ID), nil)
		deleteReq.Header.Set("X-User-ID", strconv.FormatInt(user1.ID, 10)) // user1 пытается удалить пост user2

		deleteW := httptest.NewRecorder()
		router.ServeHTTP(deleteW, deleteReq)

		assert.Equal(t, http.StatusInternalServerError, deleteW.Code)
	})
}

func TestQueueStats(t *testing.T) {
	router := setupFeedRouter()

	t.Run("GetQueueStats", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/admin/queue/stats", nil)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var stats map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &stats)
		require.NoError(t, err)

		assert.Contains(t, stats, "queue_length")
		assert.Contains(t, stats, "workers")
		assert.Equal(t, float64(5), stats["workers"]) // QUEUE_WORKER_COUNT
	})
}

func TestFeedLimits(t *testing.T) {
	router := setupFeedRouter()

	user1 := createTestUser(t, "User", "One")
	user2 := createTestUser(t, "User", "Two")
	createFriendship(t, user1.ID, user2.ID)

	t.Run("FeedLimitValidation", func(t *testing.T) {
		// Тест с невалидным лимитом
		req, _ := http.NewRequest("GET", "/api/v1/feed?limit=0", nil)
		req.Header.Set("X-User-ID", strconv.FormatInt(user1.ID, 10))

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Тест с лимитом больше максимального
		req2, _ := http.NewRequest("GET", "/api/v1/feed?limit=200", nil)
		req2.Header.Set("X-User-ID", strconv.FormatInt(user1.ID, 10))

		w2 := httptest.NewRecorder()
		router.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusOK, w2.Code)

		var response models.FeedResponse
		json.Unmarshal(w2.Body.Bytes(), &response)
		// Система должна ограничить количество постов
		assert.True(t, len(response.Posts) <= 100, "Система должна ограничивать количество постов")
	})
}

func TestFeedWithoutRedis(t *testing.T) {
	// Тест работы без Redis (fallback режим)
	router := setupFeedRouter()

	// Отключаем Redis
	services.RedisClient = nil
	defer func() {
		setupTestRedis() // Восстанавливаем для других тестов
	}()

	user1 := createTestUser(t, "User", "One")
	user2 := createTestUser(t, "User", "Two")
	createFriendship(t, user1.ID, user2.ID)

	post := createTestPost(t, router, user2.ID, "Пост без Redis")

	t.Run("FeedWorksWithoutRedis", func(t *testing.T) {
		req, _ := http.NewRequest("GET", "/api/v1/feed", nil)
		req.Header.Set("X-User-ID", strconv.FormatInt(user1.ID, 10))

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response models.FeedResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		// Система должна работать без Redis, получая данные из БД
		foundPost := false
		for _, p := range response.Posts {
			if p.ID == post.ID {
				foundPost = true
				break
			}
		}
		assert.True(t, foundPost, "Лента должна работать без Redis")
	})
}
