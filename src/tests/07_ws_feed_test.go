package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"social/api/handlers"
)

func setupWSServer() *gin.Engine {
	if err := SetupFeedTestDB(); err != nil {
		panic(err)
	}
	SetupTestRedis()

	r := gin.Default()
	// Мидлвар для user_id
	r.Use(func(c *gin.Context) {
		userIDHeader := c.GetHeader("X-User-ID")
		if userIDHeader != "" {
			if userID, err := strconv.ParseInt(userIDHeader, 10, 64); err == nil {
				c.Set("user_id", userID)
			}
		}
		c.Next()
	})
	// WebSocket endpoint
	r.GET("/api/v1/ws/feed", handlers.WSFeedHandler)
	// REST для создания поста
	r.POST("/api/v1/posts/create", handlers.CreatePost)
	return r
}

type wsFeedEvent struct {
	Event     string    `json:"event"`
	Message   string    `json:"message"`
	UserID    int64     `json:"user_id,omitempty"`
	PostID    int64     `json:"post_id,omitempty"`
	AuthorID  int64     `json:"author_id,omitempty"`
	Content   string    `json:"content,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

func TestWebSocketFeedPush(t *testing.T) {
	router := setupWSServer()
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Создаем пользователей и дружбу
	user1 := CreateTestUser(t, "Alice", "Test")
	user2 := CreateTestUser(t, "Bob", "Test")
	CreateFriendship(t, user1.ID, user2.ID)

	// Открываем WebSocket соединение от имени user1
	wsURL := "ws" + ts.URL[4:] + "/api/v1/ws/feed"
	dialer := websocket.Dialer{}
	headers := make(map[string][]string)
	headers["X-User-ID"] = []string{strconv.FormatInt(user1.ID, 10)}
	conn, resp, err := dialer.Dial(wsURL, headers)
	require.NoError(t, err, "WebSocket dial failed, resp: %+v", resp)
	defer conn.Close()

	// Читаем приветственное сообщение
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err)
	var hello wsFeedEvent
	_ = json.Unmarshal(msg, &hello)
	assert.Equal(t, "connected", hello.Event)

	// Создаем пост от имени user2 (друг user1)
	postData := map[string]string{"content": "Hello from Bob!"}
	jsonData, _ := json.Marshal(postData)
	client := ts.Client()
	req, _ := http.NewRequest("POST", ts.URL+"/api/v1/posts/create", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", strconv.FormatInt(user2.ID, 10))
	resp2, err := client.Do(req)
	require.NoError(t, err)
	assert.Equal(t, 201, resp2.StatusCode)

	// Ждем push события через WebSocket
	pushReceived := make(chan wsFeedEvent, 1)
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var evt wsFeedEvent
			if err := json.Unmarshal(msg, &evt); err == nil && evt.Event == "feed_posted" {
				pushReceived <- evt
				return
			}
		}
	}()

	select {
	case evt := <-pushReceived:
		assert.Equal(t, "feed_posted", evt.Event)
		assert.Equal(t, user1.ID, evt.UserID)
		assert.Equal(t, user2.ID, evt.AuthorID)
		assert.Equal(t, "Hello from Bob!", evt.Content)
		assert.NotZero(t, evt.PostID)
		assert.False(t, evt.CreatedAt.IsZero())
	case <-time.After(5 * time.Second):
		t.Fatal("Did not receive push event via WebSocket")
	}
}
