package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
)

type LoadTestResult struct {
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64
	TotalDuration   time.Duration
	RequestsPerSec  float64
	TestName        string
	Timestamp       time.Time
}

type LoadTestReport struct {
	Results map[string]LoadTestResult `json:"results"`
	Summary string                    `json:"summary"`
}

var (
	firstNames = []string{
		"Александр", "Дмитрий", "Максим", "Сергей", "Андрей",
		"Алексей", "Артем", "Илья", "Кирилл", "Михаил",
		"Никита", "Матвей", "Роман", "Егор", "Арсений",
	}
	lastNames = []string{
		"Иванов", "Петров", "Сидоров", "Смирнов", "Кузнецов",
		"Попов", "Васильев", "Соколов", "Михайлов", "Новиков",
		"Федоров", "Морозов", "Волков", "Алексеев", "Лебедев",
	}
)

func TestUserGetLoadTest(t *testing.T) {
	const maxRequests = 20
	const duration = 60 * time.Second
	const workers = 2

	result := runLoadTest(t, "User Get", duration, workers, maxRequests, func() error {
		userID := gofakeit.Number(1, 100)
		reqURL := fmt.Sprintf("%s/api/v1/user/get/%d", ApiBaseUrl, userID)

		resp, err := http.Get(reqURL)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}

		return nil
	})

	// Сохраняем результаты в файл
	saveLoadTestResult(t, "user_get_load_test", result)
	printLoadTestResult(t, "User Get Load Test", result)
}

func TestUserSearchLoadTest(t *testing.T) {
	const maxRequests = 20
	const duration = 60 * time.Second
	const workers = 100

	result := runLoadTest(t, "User Search", duration, workers, maxRequests, func() error {
		params := url.Values{}

		// Случайно выбираем параметры поиска
		useFirstName := gofakeit.Bool()
		useLastName := gofakeit.Float32() > 0.3

		if useFirstName {
			firstName := firstNames[gofakeit.Number(0, len(firstNames)-1)]
			params.Add("first_name", firstName)
		}

		if useLastName {
			lastName := lastNames[gofakeit.Number(0, len(lastNames)-1)]
			params.Add("last_name", lastName)
		}

		// Если не выбрано ни одного параметра, используем first_name
		if len(params) == 0 {
			firstName := firstNames[gofakeit.Number(0, len(firstNames)-1)]
			params.Add("first_name", firstName)
		}

		// Добавляем лимит и offset
		params.Add("limit", fmt.Sprintf("%d", gofakeit.Number(10, 50)))
		params.Add("offset", fmt.Sprintf("%d", gofakeit.Number(0, 100)))

		reqURL := fmt.Sprintf("%s/api/v1/user/search?%s", ApiBaseUrl, params.Encode())

		resp, err := http.Get(reqURL)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
		}

		return nil
	})

	// Сохраняем результаты в файл
	saveLoadTestResult(t, "user_search_load_test", result)
	printLoadTestResult(t, "User Search Load Test", result)
}

func TestMixedLoadTest(t *testing.T) {
	const maxRequests = 20
	const duration = 60 * time.Second
	const workers = 100

	result := runLoadTest(t, "Mixed Load", duration, workers, maxRequests, func() error {
		if gofakeit.Bool() {
			// 50% запросов на /user/get/{id}
			userID := gofakeit.Number(1, 1000000)
			reqURL := fmt.Sprintf("%s/api/v1/user/get/%d", ApiBaseUrl, userID)

			resp, err := http.Get(reqURL)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
				return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			}
		} else {
			// 50% запросов на /user/search
			params := url.Values{}

			firstName := firstNames[gofakeit.Number(0, len(firstNames)-1)]
			lastName := lastNames[gofakeit.Number(0, len(lastNames)-1)]
			useBoth := gofakeit.Float32() > 0.7

			if useBoth || gofakeit.Bool() {
				params.Add("first_name", firstName)
			}
			if useBoth || gofakeit.Bool() {
				params.Add("last_name", lastName)
			}

			if len(params) == 0 {
				params.Add("first_name", firstName)
			}

			params.Add("limit", fmt.Sprintf("%d", gofakeit.Number(10, 50)))

			reqURL := fmt.Sprintf("%s/api/v1/user/search?%s", ApiBaseUrl, params.Encode())

			resp, err := http.Get(reqURL)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
			}
		}

		return nil
	})

	// Сохраняем результаты в файл
	saveLoadTestResult(t, "mixed_load_test", result)
	printLoadTestResult(t, "Mixed Load Test", result)
}

func runLoadTest(t *testing.T, testName string, duration time.Duration, workers int, maxRequests int64, requestFunc func() error) LoadTestResult {
	var (
		totalRequests   int64
		successRequests int64
		failedRequests  int64
		wg              sync.WaitGroup
		sem             = make(chan struct{}, workers)
		done            = make(chan struct{})
		once            sync.Once // для безопасного закрытия канала
	)

	startTime := time.Now()

	t.Logf("=== %s ===", testName)
	t.Logf("Длительность: %v", duration)
	t.Logf("Воркеры: %d", workers)
	t.Logf("Максимальное количество запросов: %d", maxRequests)
	t.Logf("Начало тестирования...")

	// Запускаем таймер для остановки тестирования
	go func() {
		time.Sleep(duration)
		once.Do(func() { close(done) })
	}()

	// Запускаем воркеры
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-done:
					return
				case sem <- struct{}{}:
					atomic.AddInt64(&totalRequests, 1)

					err := requestFunc()
					if err != nil {
						atomic.AddInt64(&failedRequests, 1)
						t.Logf("Request failed: %v", err)
					} else {
						atomic.AddInt64(&successRequests, 1)
					}

					<-sem

					// Проверяем достигнуто ли максимальное количество запросов
					if atomic.LoadInt64(&totalRequests) >= maxRequests {
						once.Do(func() { close(done) })
						return
					}
				}
			}
		}()
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	return LoadTestResult{
		TotalRequests:   totalRequests,
		SuccessRequests: successRequests,
		FailedRequests:  failedRequests,
		TotalDuration:   totalDuration,
		RequestsPerSec:  float64(totalRequests) / totalDuration.Seconds(),
	}
}

func printLoadTestResult(t *testing.T, testName string, result LoadTestResult) {
	t.Logf("=== Результаты %s ===", testName)
	t.Logf("Общее количество запросов: %d", result.TotalRequests)
	t.Logf("Успешные запросы: %d", result.SuccessRequests)
	t.Logf("Неудачные запросы: %d", result.FailedRequests)
	t.Logf("Общее время: %v", result.TotalDuration)
	t.Logf("Запросов в секунду: %.2f", result.RequestsPerSec)
	t.Logf("Процент успешных запросов: %.2f%%", float64(result.SuccessRequests)/float64(result.TotalRequests)*100)
	t.Logf("")
}

func saveLoadTestResult(t *testing.T, testName string, result LoadTestResult) {
	result.TestName = testName
	result.Timestamp = time.Now()

	// Определяем суффикс для файла (before/after replication)
	suffix := os.Getenv("LOAD_TEST_SUFFIX")
	if suffix == "" {
		suffix = "before"
	}

	filename := fmt.Sprintf("%s_%s.json", testName, suffix)

	file, err := os.Create(filename)
	if err != nil {
		t.Logf("Ошибка создания файла %s: %v", filename, err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		t.Logf("Ошибка записи результатов в файл %s: %v", filename, err)
		return
	}

	t.Logf("Результаты сохранены в файл: %s", filename)
}
