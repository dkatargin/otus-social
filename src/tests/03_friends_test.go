package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"social/api/handlers"
	"social/db"
	"social/models"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB() error {
	// Инициализируем тестовую базу данных SQLite в памяти
	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		return err
	}

	// Автомиграция моделей
	err = database.AutoMigrate(&models.User{}, &models.Friend{})
	if err != nil {
		return err
	}

	// Создаем тестовых пользователей
	testUsers := []models.User{
		{ID: 1, Nickname: "user1", FirstName: "Test", LastName: "User1"},
		{ID: 2, Nickname: "user2", FirstName: "Test", LastName: "User2"},
	}

	for _, user := range testUsers {
		err = database.Create(&user).Error
		if err != nil {
			return err
		}
	}

	// Устанавливаем глобальную переменную ORM
	db.ORM = database
	return nil
}

func setupRouter() *gin.Engine {
	// Инициализируем тестовую базу данных
	if err := setupTestDB(); err != nil {
		panic(err)
	}

	r := gin.Default()
	// Мидлвар для тестов: всегда устанавливаем user_id=1
	r.Use(func(c *gin.Context) {
		c.Set("user_id", int64(1))
		c.Next()
	})

	r.POST("/api/v1/friends/add", handlers.AddFriend)
	r.POST("/api/v1/friends/approve", handlers.ApproveFriend)
	r.POST("/api/v1/friends/delete", handlers.DeleteFriend)
	r.GET("/api/v1/friends/list", handlers.GetFriends)
	r.GET("/api/v1/friends/requests", handlers.GetPendingRequests)
	return r
}

func TestAddFriend(t *testing.T) {
	r := setupRouter()

	body := map[string]int64{"user_id": 1, "friend_id": 2}
	jsonBody, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/friends/add", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestAddFriendSelf(t *testing.T) {
	r := setupRouter()

	body := map[string]int64{"user_id": 1, "friend_id": 1}
	jsonBody, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/friends/add", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Errorf("expected error for self-friend request, got 200")
	}
}

func TestAddFriendInvalidID(t *testing.T) {
	r := setupRouter()

	// Тестируем случай с несуществующим friend_id
	body := map[string]int64{"friend_id": 999}
	jsonBody, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/friends/add", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code == http.StatusOK {
		t.Errorf("expected error for non-existent friend ID, got 200")
	}
}

func TestAddFriendDuplicate(t *testing.T) {
	r := setupRouter()

	body := map[string]int64{"user_id": 1, "friend_id": 2}
	jsonBody, _ := json.Marshal(body)

	// Первый запрос
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("POST", "/api/v1/friends/add", bytes.NewBuffer(jsonBody))
	req1.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w1, req1)

	// Второй запрос (дубликат)
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/api/v1/friends/add", bytes.NewBuffer(jsonBody))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)

	if w2.Code == http.StatusOK {
		t.Errorf("expected error for duplicate request, got 200")
	}
}

func TestApproveFriend(t *testing.T) {
	// Инициализируем базу данных один раз
	if err := setupTestDB(); err != nil {
		panic(err)
	}

	// Создаем роутер для пользователя 1 (отправителя заявки)
	r1 := gin.Default()
	r1.Use(func(c *gin.Context) {
		c.Set("user_id", int64(1))
		c.Next()
	})
	r1.POST("/api/v1/friends/add", handlers.AddFriend)

	// Создаем заявку от пользователя 1 к пользователю 2
	body := map[string]int64{"friend_id": 2}
	jsonBody, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/friends/add", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r1.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("failed to create friend request, got %d", w.Code)
		return
	}

	// Создаем роутер для пользователя 2 (получателя заявки)
	r2 := gin.Default()
	r2.Use(func(c *gin.Context) {
		c.Set("user_id", int64(2)) // Устанавливаем user_id=2
		c.Next()
	})
	r2.POST("/api/v1/friends/approve", handlers.ApproveFriend)

	// Подтверждаем дружбу от имени пользователя 2
	approveBody := map[string]int64{"friend_id": 1}
	jsonApprove, _ := json.Marshal(approveBody)
	w2 := httptest.NewRecorder()
	approveReq, _ := http.NewRequest("POST", "/api/v1/friends/approve", bytes.NewBuffer(jsonApprove))
	approveReq.Header.Set("Content-Type", "application/json")
	r2.ServeHTTP(w2, approveReq)

	if w2.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w2.Code)
	}
}

func TestDeleteFriend(t *testing.T) {
	r := setupRouter()

	// Сначала создаём заявку
	body := map[string]int64{"user_id": 1, "friend_id": 2}
	jsonBody, _ := json.Marshal(body)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/friends/add", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// Удаляем дружбу/заявку
	deleteBody := map[string]int64{"user_id": 1, "friend_id": 2}
	jsonDelete, _ := json.Marshal(deleteBody)
	w2 := httptest.NewRecorder()
	deleteReq, _ := http.NewRequest("POST", "/api/v1/friends/delete", bytes.NewBuffer(jsonDelete))
	deleteReq.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, deleteReq)

	if w2.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w2.Code)
	}
}

func TestAddFriendInvalidRequest(t *testing.T) {
	r := setupRouter()

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/friends/add", bytes.NewBuffer([]byte(`{"friend_id": "invalid_id"}`)))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}
