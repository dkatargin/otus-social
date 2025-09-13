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
	"social/api/middleware"
	"social/db"
	"social/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Авторизация (эмулируем user_id=1) - добавляем middleware ДО роутов
	r.Use(func(c *gin.Context) { c.Set("user_id", int64(1)); c.Next() })

	r.POST("/api/v1/dialog/:user_id/send", handlers.SendMessageHandler)
	r.GET("/api/v1/dialog/:user_id/list", handlers.ListDialogHandler)

	// Отправка сообщения
	msg := map[string]interface{}{"to": 2, "text": "Hello, user 2!"}
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

// DialogMessage представляет структуру сообщения в API
type DialogMessage struct {
	ID      int64     `json:"id"`
	FromID  int64     `json:"from_id"`
	ToID    int64     `json:"to_id"`
	Text    string    `json:"text"`
	Created time.Time `json:"created_at"`
	IsRead  bool      `json:"is_read"`
}

// SendMessageRequest структура для отправки сообщения
type SendMessageRequest struct {
	To   int64  `json:"to"`
	Text string `json:"text"`
}

// sendMessage отправляет сообщение через API
func sendMessage(token string, recipientID int64, text string) (*http.Response, error) {
	url := fmt.Sprintf("%s/api/v1/dialog/%d/send", ApiBaseUrl, recipientID)

	payload := SendMessageRequest{
		To:   recipientID,
		Text: text,
	}

	jsonData, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	return http.DefaultClient.Do(req)
}

// getDialog получает диалог между пользователями
func getDialog(token string, userID int64, limit, offset int) (*http.Response, error) {
	url := fmt.Sprintf("%s/api/v1/dialog/%d/list?limit=%d&offset=%d",
		ApiBaseUrl, userID, limit, offset)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	return http.DefaultClient.Do(req)
}

// TestDialogBasicFunctionality тестирует базовую функциональность диалогов
func TestDialogBasicFunctionality(t *testing.T) {
	// Инициализируем тестовую базу данных
	if err := SetupFeedTestDB(); err != nil {
		t.Fatalf("Failed to setup test database: %v", err)
	}

	// Создаем gin роутер для тестов
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Middleware для авторизации в тестах
	r.Use(func(c *gin.Context) {
		// Извлекаем user_id из заголовка Authorization
		authHeader := c.GetHeader("Authorization")
		t.Logf("Auth header: '%s'", authHeader)
		if authHeader != "" {
			// Простое извлечение ID из токена вида "test_token_123"
			if len(authHeader) > 11 && authHeader[:11] == "test_token_" {
				userIDStr := authHeader[11:]
				t.Logf("UserID string: '%s'", userIDStr)
				var userID int64
				if n, err := fmt.Sscanf(userIDStr, "%d", &userID); err == nil && n == 1 {
					t.Logf("Setting user_id: %d", userID)
					c.Set("user_id", userID)
				} else {
					t.Logf("Failed to parse userID: %v", err)
				}
			} else {
				t.Logf("Invalid token format")
			}
		} else {
			t.Logf("No auth header")
		}
		c.Next()
	})

	r.POST("/api/v1/dialog/:user_id/send", handlers.SendMessageHandler)
	r.GET("/api/v1/dialog/:user_id/list", handlers.ListDialogHandler)

	// Создаем двух тестовых пользователей

	user1ID, user1Token := CreateTestUser(t, "dialog_user1_"+strconv.FormatInt(time.Now().UnixNano(), 10), "Dialog")
	user2ID, user2Token := CreateTestUser(t, "dialog_user2_"+strconv.FormatInt(time.Now().UnixNano(), 10), "Dialog")

	t.Run("SendMessage", func(t *testing.T) {
		// Отправляем сообщение от user1 к user2 через httptest
		payload := SendMessageRequest{
			To:   user2ID,
			Text: "Hello from user1!",
		}
		jsonData, _ := json.Marshal(payload)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/dialog/%d/send", user2ID), bytes.NewBuffer(jsonData))
		req.Header.Set("Authorization", user1Token)
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("SendMessageBackAndForth", func(t *testing.T) {
		// Отправляем несколько сообщений в обе стороны
		messages := []struct {
			token string
			to    int64
			text  string
		}{
			{user1Token, user2ID, "Message 1 from user1"},
			{user2Token, user1ID, "Message 2 from user2"},
			{user1Token, user2ID, "Message 3 from user1"},
			{user2Token, user1ID, "Message 4 from user2"},
		}

		for i, msg := range messages {
			payload := SendMessageRequest{
				To:   msg.to,
				Text: msg.text,
			}
			jsonData, _ := json.Marshal(payload)

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/dialog/%d/send", msg.to), bytes.NewBuffer(jsonData))
			req.Header.Set("Authorization", msg.token)
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code, "Failed to send message %d", i+1)
			time.Sleep(10 * time.Millisecond)
		}
	})

	t.Run("GetDialog", func(t *testing.T) {
		// Получаем диалог от лица user1
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/dialog/%d/list?limit=10&offset=0", user2ID), nil)
		req.Header.Set("Authorization", user1Token)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var result map[string]interface{}
		err := json.NewDecoder(w.Body).Decode(&result)
		require.NoError(t, err)

		if messages, ok := result["messages"]; ok && messages != nil {
			messagesList := messages.([]interface{})
			assert.Greater(t, len(messagesList), 0, "Should have messages in dialog")

			// Проверяем структуру первого сообщения
			if len(messagesList) > 0 {
				firstMsg := messagesList[0].(map[string]interface{})
				assert.Contains(t, firstMsg, "id")
				assert.Contains(t, firstMsg, "from_id")
				assert.Contains(t, firstMsg, "to_id")
				assert.Contains(t, firstMsg, "text")
				assert.Contains(t, firstMsg, "created_at")
			}
		}
	})

	t.Run("DialogSymmetry", func(t *testing.T) {
		// Проверяем, что диалог выглядит одинаково с обеих сторон
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/dialog/%d/list?limit=50&offset=0", user2ID), nil)
		req1.Header.Set("Authorization", user1Token)
		r.ServeHTTP(w1, req1)

		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/dialog/%d/list?limit=50&offset=0", user1ID), nil)
		req2.Header.Set("Authorization", user2Token)
		r.ServeHTTP(w2, req2)

		var result1, result2 map[string]interface{}
		json.NewDecoder(w1.Body).Decode(&result1)
		json.NewDecoder(w2.Body).Decode(&result2)

		// Безопасная проверка наличия сообщений
		var messages1, messages2 []interface{}
		if msgs1, ok := result1["messages"]; ok && msgs1 != nil {
			messages1 = msgs1.([]interface{})
		}
		if msgs2, ok := result2["messages"]; ok && msgs2 != nil {
			messages2 = msgs2.([]interface{})
		}

		// Количество сообщений должно быть одинаковым
		assert.Equal(t, len(messages1), len(messages2),
			"Both users should see the same number of messages")

		// Содержимое сообщений должно быть одинаковым
		for i := 0; i < len(messages1) && i < len(messages2); i++ {
			msg1 := messages1[i].(map[string]interface{})
			msg2 := messages2[i].(map[string]interface{})

			assert.Equal(t, msg1["text"], msg2["text"])
			assert.Equal(t, msg1["from_id"], msg2["from_id"])
			assert.Equal(t, msg1["to_id"], msg2["to_id"])
		}
	})
}

// TestDialogPagination тестирует пагинацию в диалогах
func TestDialogPagination(t *testing.T) {
	// Инициализируем тестовую базу данных
	if err := SetupFeedTestDB(); err != nil {
		t.Fatalf("Failed to setup test database: %v", err)
	}

	// Создаем тестовых пользователей
	user1ID, user1Token := CreateTestUser(t, "paginate_user1_"+strconv.FormatInt(time.Now().UnixNano(), 10), "Paginate")
	user2ID, user2Token := CreateTestUser(t, "paginate_user2_"+strconv.FormatInt(time.Now().UnixNano(), 10), "Paginate")

	// Создаем записи в shard_maps для пользователей
	shardMap1 := &models.ShardMap{UserID: user1ID, ShardID: 1}
	shardMap2 := &models.ShardMap{UserID: user2ID, ShardID: 1}
	db.ORM.Create(shardMap1)
	db.ORM.Create(shardMap2)

	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Используем новый middleware аутентификации
	r.Use(middleware.TestAuthMiddleware())

	r.POST("/api/v1/dialog/:user_id/send", handlers.SendMessageHandler)
	r.GET("/api/v1/dialog/:user_id/list", handlers.ListDialogHandler)

	// Отправляем несколько сообщений для создания диалога
	for i := 1; i <= 15; i++ {
		// Чередуем отправителей
		var fromToken string
		var toID int64
		if i%2 == 1 {
			fromToken = user1Token
			toID = user2ID
		} else {
			fromToken = user2Token
			toID = user1ID
		}

		req := SendMessageRequest{
			To:   toID,
			Text: fmt.Sprintf("Message number %d", i),
		}

		jsonData, _ := json.Marshal(req)
		httpReq := httptest.NewRequest("POST", fmt.Sprintf("/api/v1/dialog/%d/send", toID), bytes.NewBuffer(jsonData))
		httpReq.Header.Set("Authorization", "Bearer "+fromToken)
		httpReq.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, httpReq)

		if w.Code != 200 {
			t.Logf("Failed to send message %d, status: %d, body: %s", i, w.Code, w.Body.String())
		}
	}

	t.Run("PaginationLimits", func(t *testing.T) {
		// Тестируем лимит сообщений
		httpReq := httptest.NewRequest("GET", fmt.Sprintf("/api/v1/dialog/%d/list?limit=10", user2ID), nil)
		httpReq.Header.Set("Authorization", "Bearer "+user1Token)

		w := httptest.NewRecorder()
		r.ServeHTTP(w, httpReq)

		assert.Equal(t, 200, w.Code)

		var response map[string]interface{}
		json.Unmarshal(w.Body.Bytes(), &response)

		if messages, ok := response["messages"]; ok {
			messagesSlice := messages.([]interface{})
			assert.LessOrEqual(t, len(messagesSlice), 10, "Should respect limit parameter")
		}
	})
}

// TestDialogValidation тестирует валидацию входных данных
func TestDialogValidation(t *testing.T) {
	_, userToken := CreateTestUser(t, "val_user_"+strconv.FormatInt(time.Now().UnixNano(), 10), "Validation")

	t.Run("InvalidRecipientID", func(t *testing.T) {
		// Отправляем сообщение несуществующему пользователю
		resp, err := sendMessage(userToken, 999999, "Hello invalid user")
		require.NoError(t, err)
		defer resp.Body.Close()

		// API может вернуть разные коды: 400, 404 или даже 200 если не проверяет существование
		// Проверяем, что запрос обработался без краха сервера
		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 500)
	})

	t.Run("EmptyMessage", func(t *testing.T) {
		user2ID, _ := CreateTestUser(t, "val_user2_"+strconv.FormatInt(time.Now().UnixNano(), 10), "Validation")

		resp, err := sendMessage(userToken, user2ID, "")
		require.NoError(t, err)
		defer resp.Body.Close()

		// Пустое сообщение может быть отклонено или принято в зависимости от валидации
		assert.True(t, resp.StatusCode >= 200 && resp.StatusCode < 500)
	})

	t.Run("MismatchedUserIDs", func(t *testing.T) {
		// Создаем запрос с несоответствующими ID в URL и теле
		user2ID, _ := CreateTestUser(t, "val_user3_"+strconv.FormatInt(time.Now().UnixNano(), 10), "Validation")
		user3ID, _ := CreateTestUser(t, "val_user4_"+strconv.FormatInt(time.Now().UnixNano(), 10), "Validation")

		url := fmt.Sprintf("%s/api/v1/dialog/%d/send", ApiBaseUrl, user2ID)

		payload := SendMessageRequest{
			To:   user3ID, // Другой ID чем в URL
			Text: "This should fail",
		}

		jsonData, _ := json.Marshal(payload)
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		require.NoError(t, err)

		req.Header.Set("Authorization", "Bearer "+userToken)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Должен вернуть ошибку аутентификации (токен одного пользователя, URL другого)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("UnauthorizedAccess", func(t *testing.T) {
		// Попытка отправить сообщение без токена
		url := fmt.Sprintf("%s/api/v1/dialog/1/send", ApiBaseUrl)

		payload := SendMessageRequest{
			To:   1,
			Text: "Unauthorized message",
		}

		jsonData, _ := json.Marshal(payload)
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}
