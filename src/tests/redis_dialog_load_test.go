package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"
	"time"

	"social/api/handlers"
	"social/config"
	"social/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// RedisDialogLoadTestResult содержит результаты нагрузочного тестирования Redis-диалогов
type RedisDialogLoadTestResult struct {
	TotalRequests   int     `json:"total_requests"`
	SuccessfulSends int     `json:"successful_sends"`
	SuccessfulReads int     `json:"successful_reads"`
	FailedSends     int     `json:"failed_sends"`
	FailedReads     int     `json:"failed_reads"`
	Duration        string  `json:"duration"`
	SendThroughput  float64 `json:"send_throughput_rps"`
	ReadThroughput  float64 `json:"read_throughput_rps"`
	AvgSendLatency  string  `json:"avg_send_latency"`
	AvgReadLatency  string  `json:"avg_read_latency"`
	MaxSendLatency  string  `json:"max_send_latency"`
	MaxReadLatency  string  `json:"max_read_latency"`
	P95SendLatency  string  `json:"p95_send_latency"`
	P95ReadLatency  string  `json:"p95_read_latency"`
}

func (r *RedisDialogLoadTestResult) ToJSON() *RedisDialogLoadTestResult {
	return &RedisDialogLoadTestResult{
		TotalRequests:   r.TotalRequests,
		SuccessfulSends: r.SuccessfulSends,
		SuccessfulReads: r.SuccessfulReads,
		FailedSends:     r.FailedSends,
		FailedReads:     r.FailedReads,
		Duration:        r.Duration,
		SendThroughput:  r.SendThroughput,
		ReadThroughput:  r.ReadThroughput,
		AvgSendLatency:  r.AvgSendLatency,
		AvgReadLatency:  r.AvgReadLatency,
		MaxSendLatency:  r.MaxSendLatency,
		MaxReadLatency:  r.MaxReadLatency,
		P95SendLatency:  r.P95SendLatency,
		P95ReadLatency:  r.P95ReadLatency,
	}
}

// TestRedisDialogLoad проводит нагрузочное тестирование Redis-диалогов с UDF
func TestRedisDialogLoad(t *testing.T) {
	// Загружаем тестовую конфигурацию
	err := config.LoadConfig("../config/test.yaml")
	require.NoError(t, err)

	// Настройка Redis для диалогов
	redisService := services.NewRedisDialogService(
		fmt.Sprintf("%s:%d", "localhost", 6380), // Отдельный Redis для диалогов
		"",
		0,
	)
	defer redisService.Close()

	// Настройка gin в тестовом режиме
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Создаем обработчики Redis-диалогов
	redisHandlers := handlers.NewRedisDialogHandlers(redisService)

	// Настраиваем роуты
	api := router.Group("/api/v1")
	api.Use(func(c *gin.Context) {
		// Мок аутентификации для тестов
		c.Set("user_id", int64(1))
		c.Next()
	})

	redisDialogs := api.Group("/redis/dialog")
	{
		redisDialogs.POST("/:user_id/send", redisHandlers.SendMessageHandler)
		redisDialogs.GET("/:user_id/list", redisHandlers.ListDialogHandler)
		redisDialogs.GET("/:user_id/stats", redisHandlers.GetDialogStatsHandler)
		redisDialogs.POST("/:user_id/read", redisHandlers.MarkAsReadHandler)
	}

	// Параметры нагрузочного тестирования
	numUsers := 2
	numConcurrentWorkers := 1
	testDuration := 30 * time.Second
	sendRatio := 0.7 // 70% запросов на отправку, 30% на чтение

	// Подготавливаем пользователей и их соединения
	userIDs := make([]int64, numUsers)
	for i := 0; i < numUsers; i++ {
		userIDs[i] = int64(i + 1)
	}

	// Каналы для координации
	ctx, cancel := context.WithTimeout(context.Background(), testDuration)
	defer cancel()

	sendLatencies := make([]time.Duration, 0)
	readLatencies := make([]time.Duration, 0)
	var latencyMutex sync.Mutex

	results := &RedisDialogLoadTestResult{}
	var wg sync.WaitGroup

	startTime := time.Now()

	// Запускаем воркеров
	for i := 0; i < numConcurrentWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			localRand := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))

			for {
				select {
				case <-ctx.Done():
					return
				default:
					// Выбираем случайных пользователей
					fromUser := userIDs[localRand.Intn(len(userIDs))]
					toUser := userIDs[localRand.Intn(len(userIDs))]

					// Избегаем отправки сообщений самому себе
					if fromUser == toUser {
						continue
					}

					// Решаем, отправлять сообщение или читать
					if localRand.Float64() < sendRatio {
						// Отправка сообщения
						sendStart := time.Now()
						success := sendRedisMessage(router, fromUser, toUser, fmt.Sprintf("Test message from %d", fromUser))
						sendLatency := time.Since(sendStart)

						latencyMutex.Lock()
						sendLatencies = append(sendLatencies, sendLatency)
						if success {
							results.SuccessfulSends++
						} else {
							results.FailedSends++
						}
						latencyMutex.Unlock()
					} else {
						// Чтение сообщений
						readStart := time.Now()
						success := readRedisMessages(router, fromUser, toUser)
						readLatency := time.Since(readStart)

						latencyMutex.Lock()
						readLatencies = append(readLatencies, readLatency)
						if success {
							results.SuccessfulReads++
						} else {
							results.FailedReads++
						}
						latencyMutex.Unlock()
					}
				}
			}
		}(i)
	}

	// Ждем завершения всех воркеров
	wg.Wait()

	endTime := time.Now()
	results.Duration = endTime.Sub(startTime).String()
	results.TotalRequests = results.SuccessfulSends + results.FailedSends + results.SuccessfulReads + results.FailedReads

	// Вычисляем пропускную способность
	durationSeconds := endTime.Sub(startTime).Seconds()
	results.SendThroughput = float64(results.SuccessfulSends) / durationSeconds
	results.ReadThroughput = float64(results.SuccessfulReads) / durationSeconds

	// Вычисляем статистику по задержкам
	if len(sendLatencies) > 0 {
		sort.Slice(sendLatencies, func(i, j int) bool {
			return sendLatencies[i] < sendLatencies[j]
		})

		var totalSendLatency time.Duration
		for _, latency := range sendLatencies {
			totalSendLatency += latency
		}
		results.AvgSendLatency = (totalSendLatency / time.Duration(len(sendLatencies))).String()
		results.MaxSendLatency = sendLatencies[len(sendLatencies)-1].String()
		results.P95SendLatency = sendLatencies[int(float64(len(sendLatencies))*0.95)].String()
	}

	if len(readLatencies) > 0 {
		sort.Slice(readLatencies, func(i, j int) bool {
			return readLatencies[i] < readLatencies[j]
		})

		var totalReadLatency time.Duration
		for _, latency := range readLatencies {
			totalReadLatency += latency
		}
		results.AvgReadLatency = (totalReadLatency / time.Duration(len(readLatencies))).String()
		results.MaxReadLatency = readLatencies[len(readLatencies)-1].String()
		results.P95ReadLatency = readLatencies[int(float64(len(readLatencies))*0.95)].String()
	}

	// Выводим результаты
	t.Logf("=== Redis Dialog Load Test Results ===")
	t.Logf("Duration: %v", results.Duration)
	t.Logf("Total Requests: %d", results.TotalRequests)
	t.Logf("Successful Sends: %d", results.SuccessfulSends)
	t.Logf("Failed Sends: %d", results.FailedSends)
	t.Logf("Successful Reads: %d", results.SuccessfulReads)
	t.Logf("Failed Reads: %d", results.FailedReads)
	t.Logf("Send Throughput: %.2f RPS", results.SendThroughput)
	t.Logf("Read Throughput: %.2f RPS", results.ReadThroughput)
	t.Logf("Avg Send Latency: %v", results.AvgSendLatency)
	t.Logf("Max Send Latency: %v", results.MaxSendLatency)
	t.Logf("P95 Send Latency: %v", results.P95SendLatency)
	t.Logf("Avg Read Latency: %v", results.AvgReadLatency)
	t.Logf("Max Read Latency: %v", results.MaxReadLatency)
	t.Logf("P95 Read Latency: %v", results.P95ReadLatency)

	// Сохраняем результаты в JSON
	saveRedisLoadTestResults(t, results)

	// Проверяем базовые требования
	require.Greater(t, results.SuccessfulSends, 0, "Should have successful sends")
	require.Greater(t, results.SuccessfulReads, 0, "Should have successful reads")
	require.Less(t, float64(results.FailedSends)/float64(results.TotalRequests), 0.05, "Failed sends should be less than 5%")
	require.Less(t, float64(results.FailedReads)/float64(results.TotalRequests), 0.05, "Failed reads should be less than 5%")
}

// sendRedisMessage отправляет сообщение через Redis API
func sendRedisMessage(router *gin.Engine, fromUser, toUser int64, text string) bool {
	reqBody := map[string]interface{}{
		"text": text,
	}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/redis/dialog/%d/send", toUser), bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	// Устанавливаем пользователя в контекст
	req = req.WithContext(context.WithValue(req.Context(), "user_id", fromUser))

	w := httptest.NewRecorder()

	// Создаем gin context с пользователем
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("user_id", fromUser)

	router.ServeHTTP(w, req)

	return w.Code == http.StatusOK
}

// readRedisMessages читает сообщения через Redis API
func readRedisMessages(router *gin.Engine, user1, user2 int64) bool {
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/redis/dialog/%d/list?limit=10", user2), nil)
	req = req.WithContext(context.WithValue(req.Context(), "user_id", user1))

	w := httptest.NewRecorder()

	// Создаем gin context с пользователем
	c, _ := gin.CreateTestContext(w)
	c.Request = req
	c.Set("user_id", user1)

	router.ServeHTTP(w, req)

	return w.Code == http.StatusOK
}

// saveRedisLoadTestResults сохраняет результаты тестирования
func saveRedisLoadTestResults(t *testing.T, results *RedisDialogLoadTestResult) {
	resultsJSON, err := json.MarshalIndent(results.ToJSON(), "", "  ")
	require.NoError(t, err)

	filename := fmt.Sprintf("redis_dialog_load_test_results_%d.json", time.Now().Unix())
	t.Logf("Saving results to: %s", filename)
	t.Logf("Results JSON:\n%s", string(resultsJSON))
}
