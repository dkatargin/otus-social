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

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// DialogLoadTestResult содержит результаты нагрузочного тестирования
type DialogLoadTestResult struct {
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

// TestDialogLoadBaseline проводит базовое нагрузочное тестирование модуля диалогов
func TestDialogLoadBaseline(t *testing.T) {
	// Инициализируем тестовую базу данных
	//if err := SetupFeedTestDB(); err != nil {
	//	panic(err)
	//}

	// Настройка gin в тестовом режиме
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Middleware для авторизации (эмулируем разных пользователей)
	r.Use(func(c *gin.Context) {
		userID := c.GetHeader("X-User-ID")
		if userID != "" {
			c.Set("user_id", parseInt64(userID))
		}
		c.Next()
	})

	r.POST("/api/v1/dialog/:user_id/send", handlers.SendMessageHandler)
	r.GET("/api/v1/dialog/:user_id/list", handlers.ListDialogHandler)

	// Создаем тестовых пользователей и их шарды
	numUsers := int64(100)
	for i := int64(1); i <= numUsers; i++ {
		shardMap := &models.ShardMap{UserID: i, ShardID: int(i % 4)}
		db.ORM.Create(shardMap)
	}

	// Параметры нагрузочного теста
	duration := 30 * time.Second
	sendWorkers := 20
	readWorkers := 10
	sendInterval := 50 * time.Millisecond
	readInterval := 100 * time.Millisecond

	result := runDialogLoadTest(t, r, duration, sendWorkers, readWorkers, sendInterval, readInterval, numUsers)

	// Сохраняем результаты в файл
	saveLoadTestResults(t, "dialog_baseline_results.json", result)

	// Выводим результаты
	fmt.Printf("\n=== Dialog Load Test Baseline Results ===\n")
	fmt.Printf("Duration: %v\n", result.Duration)
	fmt.Printf("Total Send Requests: %d\n", result.SuccessfulSends+result.FailedSends)
	fmt.Printf("Total Read Requests: %d\n", result.SuccessfulReads+result.FailedReads)
	fmt.Printf("Successful Sends: %d\n", result.SuccessfulSends)
	fmt.Printf("Failed Sends: %d\n", result.FailedSends)
	fmt.Printf("Successful Reads: %d\n", result.SuccessfulReads)
	fmt.Printf("Failed Reads: %d\n", result.FailedReads)
	fmt.Printf("Send Throughput: %.2f RPS\n", result.SendThroughput)
	fmt.Printf("Read Throughput: %.2f RPS\n", result.ReadThroughput)
	fmt.Printf("Avg Send Latency: %v\n", result.AvgSendLatency)
	fmt.Printf("Avg Read Latency: %v\n", result.AvgReadLatency)
	fmt.Printf("P95 Send Latency: %v\n", result.P95SendLatency)
	fmt.Printf("P95 Read Latency: %v\n", result.P95ReadLatency)
	fmt.Printf("Max Send Latency: %v\n", result.MaxSendLatency)
	fmt.Printf("Max Read Latency: %v\n", result.MaxReadLatency)

	// Проверяем минимальные требования производительности
	require.Greater(t, result.SendThroughput, 50.0, "Send throughput should be > 50 RPS")
	require.Greater(t, result.ReadThroughput, 100.0, "Read throughput should be > 100 RPS")

	// Парсим строки обратно в time.Duration для проверок
	avgSendLatency, _ := time.ParseDuration(result.AvgSendLatency)
	avgReadLatency, _ := time.ParseDuration(result.AvgReadLatency)
	require.Less(t, avgSendLatency, 100*time.Millisecond, "Average send latency should be < 100ms")
	require.Less(t, avgReadLatency, 50*time.Millisecond, "Average read latency should be < 50ms")
}

func runDialogLoadTest(t *testing.T, r *gin.Engine, duration time.Duration, sendWorkers, readWorkers int, sendInterval, readInterval time.Duration, numUsers int64) *DialogLoadTestResult {
	var (
		sendLatencies []time.Duration
		readLatencies []time.Duration
		sendMutex     sync.Mutex
		readMutex     sync.Mutex
		sendStats     = make(map[bool]int)
		readStats     = make(map[bool]int)
	)

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var wg sync.WaitGroup
	startTime := time.Now()

	// Воркеры для отправки сообщений
	for i := 0; i < sendWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			ticker := time.NewTicker(sendInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					fromUserID := rand.Int63n(numUsers) + 1
					toUserID := rand.Int63n(numUsers) + 1
					if fromUserID == toUserID {
						toUserID = (toUserID % numUsers) + 1
					}

					start := time.Now()
					success := sendTestMessage(t, r, fromUserID, toUserID)
					latency := time.Since(start)

					sendMutex.Lock()
					sendStats[success]++
					if success {
						sendLatencies = append(sendLatencies, latency)
					}
					sendMutex.Unlock()
				}
			}
		}(i)
	}

	// Воркеры для чтения диалогов
	for i := 0; i < readWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			ticker := time.NewTicker(readInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					userID := rand.Int63n(numUsers) + 1
					otherUserID := rand.Int63n(numUsers) + 1
					if userID == otherUserID {
						otherUserID = (otherUserID % numUsers) + 1
					}

					start := time.Now()
					success := readTestDialog(t, r, userID, otherUserID)
					latency := time.Since(start)

					readMutex.Lock()
					readStats[success]++
					if success {
						readLatencies = append(readLatencies, latency)
					}
					readMutex.Unlock()
				}
			}
		}(i)
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	// Вычисляем статистики
	result := &DialogLoadTestResult{
		TotalRequests:   sendStats[true] + sendStats[false] + readStats[true] + readStats[false],
		SuccessfulSends: sendStats[true],
		FailedSends:     sendStats[false],
		SuccessfulReads: readStats[true],
		FailedReads:     readStats[false],
		Duration:        totalDuration.String(),
	}

	if len(sendLatencies) > 0 {
		result.SendThroughput = float64(result.SuccessfulSends) / totalDuration.Seconds()
		result.AvgSendLatency = fmt.Sprintf("%v", calculateAverage(sendLatencies))
		result.MaxSendLatency = fmt.Sprintf("%v", calculateMax(sendLatencies))
		result.P95SendLatency = fmt.Sprintf("%v", calculatePercentile(sendLatencies, 95))
	}

	if len(readLatencies) > 0 {
		result.ReadThroughput = float64(result.SuccessfulReads) / totalDuration.Seconds()
		result.AvgReadLatency = fmt.Sprintf("%v", calculateAverage(readLatencies))
		result.MaxReadLatency = fmt.Sprintf("%v", calculateMax(readLatencies))
		result.P95ReadLatency = fmt.Sprintf("%v", calculatePercentile(readLatencies, 95))
	}

	return result
}

func sendTestMessage(t *testing.T, r *gin.Engine, fromUserID, toUserID int64) bool {
	message := map[string]interface{}{
		"to":   toUserID,
		"text": fmt.Sprintf("Test message from %d to %d at %v", fromUserID, toUserID, time.Now()),
	}

	body, _ := json.Marshal(message)
	req, _ := http.NewRequest("POST", fmt.Sprintf("/api/v1/dialog/%d/send", toUserID), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", fmt.Sprintf("%d", fromUserID))

	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)

	return recorder.Code == http.StatusOK
}

func readTestDialog(t *testing.T, r *gin.Engine, userID, otherUserID int64) bool {
	req, _ := http.NewRequest("GET", fmt.Sprintf("/api/v1/dialog/%d/list?limit=20", otherUserID), nil)
	req.Header.Set("X-User-ID", fmt.Sprintf("%d", userID))

	recorder := httptest.NewRecorder()
	r.ServeHTTP(recorder, req)

	return recorder.Code == http.StatusOK
}

func parseInt64(s string) int64 {
	if val, err := strconv.ParseInt(s, 10, 64); err == nil {
		return val
	}
	return 0
}

func calculateAverage(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	var sum time.Duration
	for _, lat := range latencies {
		sum += lat
	}
	return sum / time.Duration(len(latencies))
}

func calculateMax(latencies []time.Duration) time.Duration {
	if len(latencies) == 0 {
		return 0
	}
	max := latencies[0]
	for _, lat := range latencies {
		if lat > max {
			max = lat
		}
	}
	return max
}

func calculatePercentile(latencies []time.Duration, percentile int) time.Duration {
	if len(latencies) == 0 {
		return 0
	}

	// Простая сортировка для вычисления перцентиля
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	index := (len(sorted) * percentile) / 100
	if index >= len(sorted) {
		index = len(sorted) - 1
	}
	return sorted[index]
}

func saveLoadTestResults(t *testing.T, filename string, result *DialogLoadTestResult) {
	data, err := json.MarshalIndent(result, "", "  ")
	require.NoError(t, err)

	err = os.WriteFile(filename, data, 0644)
	require.NoError(t, err)

	fmt.Printf("Results saved to %s\n", filename)
}
