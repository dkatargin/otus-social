package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strconv"
	"sync"
	"testing"
	"time"

	"social/api/handlers"
	"social/db"
	"social/models"
	"social/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// ComparisonTestResult содержит результаты сравнительного тестирования
type ComparisonTestResult struct {
	SQLResults   DialogLoadTestResult      `json:"sql_results"`
	RedisResults RedisDialogLoadTestResult `json:"redis_results"`
	Comparison   PerformanceComparison     `json:"comparison"`
}

// PerformanceComparison содержит сравнительные метрики производительности
type PerformanceComparison struct {
	SendThroughputImprovement float64 `json:"send_throughput_improvement"`
	ReadThroughputImprovement float64 `json:"read_throughput_improvement"`
	SendLatencyImprovement    float64 `json:"send_latency_improvement"`
	ReadLatencyImprovement    float64 `json:"read_latency_improvement"`
	P95SendLatencyImprovement float64 `json:"p95_send_latency_improvement"`
	P95ReadLatencyImprovement float64 `json:"p95_read_latency_improvement"`
	OverallPerformanceGain    float64 `json:"overall_performance_gain"`
}

// TestDialogPerformanceComparison проводит сравнительное тестирование SQL vs Redis диалогов
func TestDialogPerformanceComparison(t *testing.T) {
	t.Log("=== Starting Dialog Performance Comparison Test ===")

	// Параметры тестирования
	testConfig := struct {
		numUsers             int
		numConcurrentWorkers int
		testDuration         time.Duration
		sendRatio            float64
	}{
		numUsers:             50,
		numConcurrentWorkers: 25,
		testDuration:         20 * time.Second,
		sendRatio:            0.7,
	}

	// 1. Тестируем SQL версию
	t.Log("--- Testing SQL Dialog Implementation ---")
	sqlResults := testSQLDialogs(t, testConfig)

	// 2. Тестируем Redis версию
	t.Log("--- Testing Redis Dialog Implementation ---")
	redisResults := testRedisDialogs(t, testConfig)

	// 3. Сравниваем результаты
	comparison := calculatePerformanceComparison(sqlResults, redisResults)

	// 4. Формируем итоговый отчет
	finalResults := ComparisonTestResult{
		SQLResults:   sqlResults,
		RedisResults: redisResults,
		Comparison:   comparison,
	}

	// Выводим результаты сравнения
	printComparisonResults(t, finalResults)

	// Сохраняем результаты
	saveComparisonResults(t, finalResults)

	// Проверяем базовые результаты
	require.Greater(t, sqlResults.SuccessfulSends, 0, "SQL should have successful sends")
	require.Greater(t, redisResults.SuccessfulSends, 0, "Redis should have successful sends")
}

// testSQLDialogs тестирует SQL версию диалогов
func testSQLDialogs(t *testing.T, testConfig interface{}) DialogLoadTestResult {
	// Инициализируем тестовую базу данных
	if err := SetupFeedTestDB(); err != nil {
		t.Fatalf("Failed to setup test DB: %v", err)
	}

	cfg := testConfig.(struct {
		numUsers             int
		numConcurrentWorkers int
		testDuration         time.Duration
		sendRatio            float64
	})

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Настройка middleware
	router.Use(func(c *gin.Context) {
		userIDStr := c.GetHeader("X-User-ID")
		if userIDStr != "" {
			if userID, err := strconv.ParseInt(userIDStr, 10, 64); err == nil {
				c.Set("user_id", userID)
			}
		}
		c.Next()
	})

	// SQL роуты
	api := router.Group("/api/v1")
	api.POST("/dialog/:user_id/send", handlers.SendMessageHandler)
	api.GET("/dialog/:user_id/list", handlers.ListDialogHandler)

	// Создаем тестовых пользователей
	for i := 1; i <= cfg.numUsers; i++ {
		shardMap := &models.ShardMap{UserID: int64(i), ShardID: i % 4}
		db.ORM.Create(shardMap)
	}

	return runSQLLoadTest(router, cfg)
}

// testRedisDialogs тестирует Redis версию диалогов
func testRedisDialogs(t *testing.T, testConfig interface{}) RedisDialogLoadTestResult {
	cfg := testConfig.(struct {
		numUsers             int
		numConcurrentWorkers int
		testDuration         time.Duration
		sendRatio            float64
	})

	// Инициализируем Redis сервис
	redisService := services.NewRedisDialogService("localhost:6380", "", 0)
	defer func() {
		if err := redisService.Close(); err != nil {
			t.Logf("Warning: failed to close Redis service: %v", err)
		}
	}()

	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Настройка middleware
	router.Use(func(c *gin.Context) {
		userIDStr := c.GetHeader("X-User-ID")
		if userIDStr != "" {
			if userID, err := strconv.ParseInt(userIDStr, 10, 64); err == nil {
				c.Set("user_id", userID)
			}
		}
		c.Next()
	})

	// Redis роуты
	redisHandlers := handlers.NewRedisDialogHandlers(redisService)
	api := router.Group("/api/v1")
	redisDialogs := api.Group("/redis/dialog")
	{
		redisDialogs.POST("/:user_id/send", redisHandlers.SendMessageHandler)
		redisDialogs.GET("/:user_id/list", redisHandlers.ListDialogHandler)
	}

	return runRedisLoadTest(router, cfg)
}

// runSQLLoadTest выполняет нагрузочный тест для SQL
func runSQLLoadTest(router *gin.Engine, cfg interface{}) DialogLoadTestResult {
	testCfg := cfg.(struct {
		numUsers             int
		numConcurrentWorkers int
		testDuration         time.Duration
		sendRatio            float64
	})

	results := DialogLoadTestResult{}
	var wg sync.WaitGroup
	var mu sync.Mutex

	sendLatencies := make([]time.Duration, 0)
	readLatencies := make([]time.Duration, 0)

	ctx, cancel := context.WithTimeout(context.Background(), testCfg.testDuration)
	defer cancel()

	startTime := time.Now()

	// Запускаем воркеров
	for i := 0; i < testCfg.numConcurrentWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			localRand := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))

			for {
				select {
				case <-ctx.Done():
					return
				default:
					fromUser := int64(localRand.Intn(testCfg.numUsers) + 1)
					toUser := int64(localRand.Intn(testCfg.numUsers) + 1)

					if fromUser == toUser {
						continue
					}

					if localRand.Float64() < testCfg.sendRatio {
						// Отправка сообщения
						start := time.Now()
						success := sendSQLMessage(router, fromUser, toUser, "Test message")
						latency := time.Since(start)

						mu.Lock()
						sendLatencies = append(sendLatencies, latency)
						if success {
							results.SuccessfulSends++
						} else {
							results.FailedSends++
						}
						mu.Unlock()
					} else {
						// Чтение сообщений
						start := time.Now()
						success := readSQLMessages(router, fromUser, toUser)
						latency := time.Since(start)

						mu.Lock()
						readLatencies = append(readLatencies, latency)
						if success {
							results.SuccessfulReads++
						} else {
							results.FailedReads++
						}
						mu.Unlock()
					}
				}
			}
		}(i)
	}

	wg.Wait()

	// Вычисляем статистики
	results.Duration = time.Since(startTime).String()
	results.TotalRequests = results.SuccessfulSends + results.FailedSends + results.SuccessfulReads + results.FailedReads

	durationSeconds := time.Since(startTime).Seconds()
	results.SendThroughput = float64(results.SuccessfulSends) / durationSeconds
	results.ReadThroughput = float64(results.SuccessfulReads) / durationSeconds

	// Статистики задержек
	if len(sendLatencies) > 0 {
		sort.Slice(sendLatencies, func(i, j int) bool { return sendLatencies[i] < sendLatencies[j] })
		results.AvgSendLatency = calculateAvgLatency(sendLatencies).String()
		results.MaxSendLatency = sendLatencies[len(sendLatencies)-1].String()
		results.P95SendLatency = sendLatencies[int(float64(len(sendLatencies))*0.95)].String()
	}

	if len(readLatencies) > 0 {
		sort.Slice(readLatencies, func(i, j int) bool { return readLatencies[i] < readLatencies[j] })
		results.AvgReadLatency = calculateAvgLatency(readLatencies).String()
		results.MaxReadLatency = readLatencies[len(readLatencies)-1].String()
		results.P95ReadLatency = readLatencies[int(float64(len(readLatencies))*0.95)].String()
	}

	return results
}

// runRedisLoadTest выполняет нагрузочный тест для Redis
func runRedisLoadTest(router *gin.Engine, cfg interface{}) RedisDialogLoadTestResult {
	testCfg := cfg.(struct {
		numUsers             int
		numConcurrentWorkers int
		testDuration         time.Duration
		sendRatio            float64
	})

	results := RedisDialogLoadTestResult{}
	var wg sync.WaitGroup
	var mu sync.Mutex

	sendLatencies := make([]time.Duration, 0)
	readLatencies := make([]time.Duration, 0)

	ctx, cancel := context.WithTimeout(context.Background(), testCfg.testDuration)
	defer cancel()

	startTime := time.Now()

	// Запускаем воркеров
	for i := 0; i < testCfg.numConcurrentWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			localRand := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))

			for {
				select {
				case <-ctx.Done():
					return
				default:
					fromUser := int64(localRand.Intn(testCfg.numUsers) + 1)
					toUser := int64(localRand.Intn(testCfg.numUsers) + 1)

					if fromUser == toUser {
						continue
					}

					if localRand.Float64() < testCfg.sendRatio {
						// Отправка сообщения
						start := time.Now()
						success := sendRedisMessageTest(router, fromUser, toUser, "Test message")
						latency := time.Since(start)

						mu.Lock()
						sendLatencies = append(sendLatencies, latency)
						if success {
							results.SuccessfulSends++
						} else {
							results.FailedSends++
						}
						mu.Unlock()
					} else {
						// Чтение сообщений
						start := time.Now()
						success := readRedisMessagesTest(router, fromUser, toUser)
						latency := time.Since(start)

						mu.Lock()
						readLatencies = append(readLatencies, latency)
						if success {
							results.SuccessfulReads++
						} else {
							results.FailedReads++
						}
						mu.Unlock()
					}
				}
			}
		}(i)
	}

	wg.Wait()

	// Вычисляем статистики (аналогично SQL версии)
	results.Duration = time.Since(startTime).String()
	results.TotalRequests = results.SuccessfulSends + results.FailedSends + results.SuccessfulReads + results.FailedReads

	durationSeconds := time.Since(startTime).Seconds()
	results.SendThroughput = float64(results.SuccessfulSends) / durationSeconds
	results.ReadThroughput = float64(results.SuccessfulReads) / durationSeconds

	if len(sendLatencies) > 0 {
		sort.Slice(sendLatencies, func(i, j int) bool { return sendLatencies[i] < sendLatencies[j] })
		results.AvgSendLatency = calculateAvgLatency(sendLatencies).String()
		results.MaxSendLatency = sendLatencies[len(sendLatencies)-1].String()
		results.P95SendLatency = sendLatencies[int(float64(len(sendLatencies))*0.95)].String()
	}

	if len(readLatencies) > 0 {
		sort.Slice(readLatencies, func(i, j int) bool { return readLatencies[i] < readLatencies[j] })
		results.AvgReadLatency = calculateAvgLatency(readLatencies).String()
		results.MaxReadLatency = readLatencies[len(readLatencies)-1].String()
		results.P95ReadLatency = readLatencies[int(float64(len(readLatencies))*0.95)].String()
	}

	return results
}

// Вспомогательные функции для тестирования
func sendSQLMessage(router *gin.Engine, fromUser, toUser int64, text string) bool {
	reqBody := map[string]interface{}{"to": toUser, "text": text}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/dialog/%d/send", toUser), bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", strconv.FormatInt(fromUser, 10))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	return w.Code == http.StatusOK
}

func readSQLMessages(router *gin.Engine, user1, user2 int64) bool {
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/dialog/%d/list?limit=10", user2), nil)
	req.Header.Set("X-User-ID", strconv.FormatInt(user1, 10))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	return w.Code == http.StatusOK
}

func sendRedisMessageTest(router *gin.Engine, fromUser, toUser int64, text string) bool {
	reqBody := map[string]interface{}{"text": text}
	jsonBody, _ := json.Marshal(reqBody)

	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/redis/dialog/%d/send", toUser), bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", strconv.FormatInt(fromUser, 10))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	return w.Code == http.StatusOK
}

func readRedisMessagesTest(router *gin.Engine, user1, user2 int64) bool {
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/redis/dialog/%d/list?limit=10", user2), nil)
	req.Header.Set("X-User-ID", strconv.FormatInt(user1, 10))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	return w.Code == http.StatusOK
}

func calculateAvgLatency(latencies []time.Duration) time.Duration {
	var total time.Duration
	for _, latency := range latencies {
		total += latency
	}
	return total / time.Duration(len(latencies))
}

// calculatePerformanceComparison вычисляет сравнительные метрики
func calculatePerformanceComparison(sql DialogLoadTestResult, redis RedisDialogLoadTestResult) PerformanceComparison {
	comparison := PerformanceComparison{}

	// Улучшение пропускной способности (в %)
	if sql.SendThroughput > 0 {
		comparison.SendThroughputImprovement = ((redis.SendThroughput - sql.SendThroughput) / sql.SendThroughput) * 100
	}
	if sql.ReadThroughput > 0 {
		comparison.ReadThroughputImprovement = ((redis.ReadThroughput - sql.ReadThroughput) / sql.ReadThroughput) * 100
	}

	// Улучшение задержки (в %) - уменьшение задержки считается улучшением
	if sql.AvgSendLatency != "" {
		sqlSendLatency, _ := time.ParseDuration(sql.AvgSendLatency)
		redisSendLatency, _ := time.ParseDuration(redis.AvgSendLatency)
		if sqlSendLatency > 0 {
			comparison.SendLatencyImprovement = ((sqlSendLatency - redisSendLatency).Seconds() / sqlSendLatency.Seconds()) * 100
		}
	}
	if sql.AvgReadLatency != "" {
		sqlReadLatency, _ := time.ParseDuration(sql.AvgReadLatency)
		redisReadLatency, _ := time.ParseDuration(redis.AvgReadLatency)
		if sqlReadLatency > 0 {
			comparison.ReadLatencyImprovement = ((sqlReadLatency - redisReadLatency).Seconds() / sqlReadLatency.Seconds()) * 100
		}
	}

	// P95 задержки
	if sql.P95SendLatency != "" {
		sqlP95SendLatency, _ := time.ParseDuration(sql.P95SendLatency)
		redisP95SendLatency, _ := time.ParseDuration(redis.P95SendLatency)
		if sqlP95SendLatency > 0 {
			comparison.P95SendLatencyImprovement = ((sqlP95SendLatency - redisP95SendLatency).Seconds() / sqlP95SendLatency.Seconds()) * 100
		}
	}
	if sql.P95ReadLatency != "" {
		sqlP95ReadLatency, _ := time.ParseDuration(sql.P95ReadLatency)
		redisP95ReadLatency, _ := time.ParseDuration(redis.P95ReadLatency)
		if sqlP95ReadLatency > 0 {
			comparison.P95ReadLatencyImprovement = ((sqlP95ReadLatency - redisP95ReadLatency).Seconds() / sqlP95ReadLatency.Seconds()) * 100
		}
	}

	// Общая производительность
	comparison.OverallPerformanceGain = (comparison.SendThroughputImprovement + comparison.ReadThroughputImprovement +
		comparison.SendLatencyImprovement + comparison.ReadLatencyImprovement) / 4

	return comparison
}

// printComparisonResults выводит результаты сравнения
func printComparisonResults(t *testing.T, results ComparisonTestResult) {
	t.Log("\n=== PERFORMANCE COMPARISON RESULTS ===")
	t.Logf("SQL Throughput - Send: %.2f RPS, Read: %.2f RPS", results.SQLResults.SendThroughput, results.SQLResults.ReadThroughput)
	t.Logf("Redis Throughput - Send: %.2f RPS, Read: %.2f RPS", results.RedisResults.SendThroughput, results.RedisResults.ReadThroughput)
	t.Logf("Send Throughput Improvement: %.2f%%", results.Comparison.SendThroughputImprovement)
	t.Logf("Read Throughput Improvement: %.2f%%", results.Comparison.ReadThroughputImprovement)
	t.Logf("Send Latency Improvement: %.2f%%", results.Comparison.SendLatencyImprovement)
	t.Logf("Read Latency Improvement: %.2f%%", results.Comparison.ReadLatencyImprovement)
	t.Logf("Overall Performance Gain: %.2f%%", results.Comparison.OverallPerformanceGain)
}

// saveComparisonResults сохраняет результаты сравнения
func saveComparisonResults(t *testing.T, results ComparisonTestResult) {
	resultsJSON, err := json.MarshalIndent(results, "", "  ")
	require.NoError(t, err)

	filename := fmt.Sprintf("dialog_performance_comparison_%d.json", time.Now().Unix())
	err = os.WriteFile(filename, resultsJSON, 0644)
	require.NoError(t, err)

	t.Logf("Comparison results saved to: %s", filename)
}
