package tests

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"social/api/handlers"
	"testing"
)

func TestSendAndListDialog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v1/dialog/:user_id/send", handlers.SendMessageHandler)
	r.GET("/api/v1/dialog/:user_id/list", handlers.ListDialogHandler)

	// Авторизация (эмулируем user_id=1)
	r.Use(func(c *gin.Context) { c.Set("user_id", int64(1)) })

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
