package tests

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"social/api/handlers"
	"social/db"
	"social/models"
	"testing"
)

func TestSendAndListDialog(t *testing.T) {
	// Инициализируем тестовую базу данных
	if err := SetupFeedTestDB(); err != nil {
		panic(err)
	}

	// Создаем записи в shard_maps для пользователей
	shardMap1 := &models.ShardMap{UserID: 1, ShardID: 1}
	shardMap2 := &models.ShardMap{UserID: 2, ShardID: 1}
	db.ORM.Create(shardMap1)
	db.ORM.Create(shardMap2)

	// Создаем таблицу messages_1 для шарда 1
	db.ORM.Exec("CREATE TABLE IF NOT EXISTS messages_1 (id INTEGER PRIMARY KEY AUTOINCREMENT, from_user_id INTEGER, to_user_id INTEGER, text TEXT, created_at DATETIME, is_read BOOLEAN)")

	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Авторизация (эмулируем user_id=1) - добавляем middleware ДО роутов
	r.Use(func(c *gin.Context) { c.Set("user_id", int64(1)); c.Next() })

	r.POST("/api/v1/dialog/:user_id/send", handlers.SendMessageHandler)
	r.GET("/api/v1/dialog/:user_id/list", handlers.ListDialogHandler)

	// Отправка сообщения
	msg := map[string]string{"text": "Hello, user 2!"}
	body, _ := json.Marshal(msg)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/dialog/2/send", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	assert.Equal(t, 200, w.Code)

	// Получение диалога
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("GET", "/api/v1/dialog/2/list", nil)
	r.ServeHTTP(w2, req2)
	assert.Equal(t, 200, w2.Code)
	var resp map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &resp)
	assert.NotEmpty(t, resp["messages"])
}
