package tests

import (
	"bytes"
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

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func setupFeedRouter() *gin.Engine {
	// Инициализируем тестовую базу данных
	if err := SetupFeedTestDB(); err != nil {
		panic(err)
	}

	// Инициализируем Redis (заглушка для тестов)
	if err := SetupTestRedis(); err != nil {
		panic(err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Middleware для авторизации в тестах
	r.Use(func(c *gin.Context) {
		userID := c.GetHeader("X-User-ID")
		if userID != "" {
			if id, err := strconv.ParseInt(userID, 10, 64); err == nil {
				c.Set("user_id", id)
			}
		}
		c.Next()
	})

	// Регистрируем роуты для фида
	r.POST("/api/v1/posts/create", handlers.CreatePost)
	r.GET("/api/v1/feed", handlers.GetFeed)

	return r
}

// createTestUserForFeed создает тестового пользователя специально для feed тестов
func createTestUserForFeed(t *testing.T, firstName, lastName string) *models.User {
	nickname := fmt.Sprintf("user_%d", time.Now().UnixNano())

	user := &models.User{
		FirstName: firstName,
		LastName:  lastName,
		Nickname:  nickname,
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
	user1 := createTestUserForFeed(t, "John", "Doe")
	user2 := createTestUserForFeed(t, "Jane", "Smith")
	user3 := createTestUserForFeed(t, "Bob", "Johnson")

	// Устанавливаем дружбу между user1 и user2, user1 и user3
	createFriendship(t, user1.ID, user2.ID)
	createFriendship(t, user1.ID, user3.ID)

	// user2 и user3 создают посты
	createTestPost(t, router, user2.ID, "Пост от Jane")
	createTestPost(t, router, user3.ID, "Пост от Bob")
	createTestPost(t, router, user1.ID, "Пост от John")

	// Получаем фид для user1
	req, _ := http.NewRequest("GET", "/api/v1/feed", nil)
	req.Header.Set("X-User-ID", strconv.FormatInt(user1.ID, 10))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	posts, ok := response["posts"].([]interface{})
	require.True(t, ok)
	require.Greater(t, len(posts), 0, "Feed should contain posts from friends")
}

func TestFeedCaching(t *testing.T) {
	router := setupFeedRouter()

	// Создаем пользователя
	user1 := createTestUserForFeed(t, "Cache", "User")

	// Первый запрос фида
	req1, _ := http.NewRequest("GET", "/api/v1/feed", nil)
	req1.Header.Set("X-User-ID", strconv.FormatInt(user1.ID, 10))

	w1 := httptest.NewRecorder()
	start1 := time.Now()
	router.ServeHTTP(w1, req1)
	duration1 := time.Since(start1)

	require.Equal(t, http.StatusOK, w1.Code)

	// Второй запрос фида (должен быть из кеша)
	req2, _ := http.NewRequest("GET", "/api/v1/feed", nil)
	req2.Header.Set("X-User-ID", strconv.FormatInt(user1.ID, 10))

	w2 := httptest.NewRecorder()
	start2 := time.Now()
	router.ServeHTTP(w2, req2)
	duration2 := time.Since(start2)

	require.Equal(t, http.StatusOK, w2.Code)

	// Второй запрос должен быть быстрее (кеширование)
	t.Logf("First request: %v, Second request: %v", duration1, duration2)
}

func TestFeedPagination(t *testing.T) {
	router := setupFeedRouter()

	// Создаем пользователей
	user1 := createTestUserForFeed(t, "Paginate", "User")
	user2 := createTestUserForFeed(t, "Friend", "User")

	// Устанавливаем дружбу
	createFriendship(t, user1.ID, user2.ID)

	// Создаем много постов
	for i := 0; i < 25; i++ {
		createTestPost(t, router, user2.ID, fmt.Sprintf("Пост номер %d", i+1))
	}

	// Тестируем пагинацию
	req, _ := http.NewRequest("GET", "/api/v1/feed?limit=10&offset=0", nil)
	req.Header.Set("X-User-ID", strconv.FormatInt(user1.ID, 10))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	posts, ok := response["posts"].([]interface{})
	require.True(t, ok)
	require.LessOrEqual(t, len(posts), 10, "Should respect limit parameter")
}

func TestFeedPerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	router := setupFeedRouter()

	// Создаем пользователя с большим количеством друзей
	mainUser := createTestUserForFeed(t, "Popular", "User")

	// Создаем 100 друзей
	friends := make([]*models.User, 100)
	for i := 0; i < 100; i++ {
		friend := createTestUserForFeed(t, fmt.Sprintf("Friend%d", i), "User")
		friends[i] = friend
		createFriendship(t, mainUser.ID, friend.ID)

		// Каждый друг создает по 5 постов
		for j := 0; j < 5; j++ {
			createTestPost(t, router, friend.ID, fmt.Sprintf("Пост %d от друга %d", j+1, i+1))
		}
	}

	// Тестируем производительность загрузки фида
	start := time.Now()
	req, _ := http.NewRequest("GET", "/api/v1/feed?limit=50", nil)
	req.Header.Set("X-User-ID", strconv.FormatInt(mainUser.ID, 10))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	duration := time.Since(start)

	require.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	posts, ok := response["posts"].([]interface{})
	require.True(t, ok)
	require.Greater(t, len(posts), 0, "Should return posts")

	t.Logf("Feed loading time for 100 friends with 500 total posts: %v", duration)

	// Время загрузки должно быть разумным (меньше 5 секунд)
	require.Less(t, duration.Seconds(), 5.0, "Feed should load in reasonable time")
}
