package tests

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"
)

type SearchResult struct {
	Latency time.Duration
	Success bool
}

func userSearch(concurrency int, iterations int, query string) ([]SearchResult, float64) {
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
			reqStart := time.Now()
			resp, err := http.Get(fmt.Sprintf("%s/users/search?query=%s", ApiBaseUrl, query))
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
	iterations := 2
	query := "don" // предполагается, что такие пользователи есть

	report := make(map[string]interface{})

	for _, c := range concurrencies {
		log.Println("Running user search with concurrency:", c)
		results, throughput := userSearch(c, iterations, query)
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
