package tests

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
)

type SearchResult struct {
	Latency time.Duration
	Success bool
}

func userSearch(concurrency int, iterations int) ([]SearchResult, float64) {
	var wg sync.WaitGroup
	results := make([]SearchResult, iterations)
	start := time.Now()
	sem := make(chan struct{}, concurrency)

	firstNames := []string{
		"Александр", "Дмитрий", "Максим", "Сергей", "Андрей",
		"Алексей", "Артем", "Илья", "Кирилл", "Михаил",
	}

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			// Формируем правильные параметры поиска
			params := url.Values{}
			firstName := firstNames[gofakeit.Number(0, len(firstNames)-1)]
			params.Add("first_name", firstName)
			params.Add("limit", "10")

			searchURL := fmt.Sprintf("%s/api/v1/user/search?%s", ApiBaseUrl, params.Encode())

			reqStart := time.Now()
			resp, err := http.Get(searchURL)
			latency := time.Since(reqStart)
			success := err == nil && resp.StatusCode == http.StatusOK
			results[idx] = SearchResult{Latency: latency, Success: success}
			if resp != nil {
				resp.Body.Close()
			}
		}(i)
	}
	wg.Wait()
	totalTime := time.Since(start).Seconds()
	throughput := float64(iterations) / totalTime
	return results, throughput
}

func TestUserSearchLoad(t *testing.T) {
	concurrencies := []int{1, 10, 100, 1000}
	iterations := 100

	report := make(map[string]interface{})

	for _, c := range concurrencies {
		log.Println("Running user search with concurrency:", c)
		results, throughput := userSearch(c, iterations)
		var latencies []float64
		success := 0
		for _, r := range results {
			latencies = append(latencies, r.Latency.Seconds()*1000)
			if r.Success {
				success++
			}
		}
		report[fmt.Sprintf("after_index_conc_%d", c)] = map[string]interface{}{
			"latencies_ms": latencies,
			"throughput":   throughput,
			"success":      success,
		}
	}
	f, _ := os.Create("user_search_report_after_index.json")
	defer f.Close()
	json.NewEncoder(f).Encode(report)
}

// Тест для /user/get/{id} endpoint
func userGet(concurrency int, iterations int) ([]SearchResult, float64) {
	var wg sync.WaitGroup
	results := make([]SearchResult, iterations)
	start := time.Now()
	sem := make(chan struct{}, concurrency)

	for i := 0; i < iterations; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			// Случайный ID пользователя
			userID := gofakeit.Number(1, 1000000)
			getURL := fmt.Sprintf("%s/api/v1/user/get/%d", ApiBaseUrl, userID)

			reqStart := time.Now()
			resp, err := http.Get(getURL)
			latency := time.Since(reqStart)
			success := err == nil && (resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound)
			results[idx] = SearchResult{Latency: latency, Success: success}
			if resp != nil {
				resp.Body.Close()
			}
		}(i)
	}
	wg.Wait()
	totalTime := time.Since(start).Seconds()
	throughput := float64(iterations) / totalTime
	return results, throughput
}

func TestUserGetLoad(t *testing.T) {
	concurrencies := []int{1, 10, 100, 1000}
	iterations := 100

	report := make(map[string]interface{})

	for _, c := range concurrencies {
		log.Println("Running user get with concurrency:", c)
		results, throughput := userGet(c, iterations)
		var latencies []float64
		success := 0
		for _, r := range results {
			latencies = append(latencies, r.Latency.Seconds()*1000)
			if r.Success {
				success++
			}
		}
		report[fmt.Sprintf("user_get_conc_%d", c)] = map[string]interface{}{
			"latencies_ms": latencies,
			"throughput":   throughput,
			"success":      success,
		}
	}
	f, _ := os.Create("user_get_report.json")
	defer f.Close()
	json.NewEncoder(f).Encode(report)
}
