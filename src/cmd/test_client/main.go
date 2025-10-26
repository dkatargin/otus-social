package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type SendMessageRequest struct {
	Text string `json:"text"`
}

type Stats struct {
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64
	TotalDuration   int64
}

type Config struct {
	DialogsURL     string
	Workers        int
	Duration       int
	MessageCount   int
	UserIDFrom     int64
	UserIDTo       int64
	RequestsPerSec int
}

var (
	stats Stats
)

func main() {
	config := parseFlags()

	log.Printf("Starting test client with config: %+v", config)

	done := make(chan bool)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	var wg sync.WaitGroup

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	requestsPerWorker := config.RequestsPerSec / config.Workers
	if requestsPerWorker == 0 {
		requestsPerWorker = 1
	}

	for i := 0; i < config.Workers; i++ {
		wg.Add(1)
		go worker(i, config, requestsPerWorker, done, &wg)
	}

	go printStats()

	if config.Duration > 0 {
		go func() {
			time.Sleep(time.Duration(config.Duration) * time.Second)
			close(done)
		}()
	}

	go func() {
		<-sigChan
		log.Println("\nReceived interrupt signal, shutting down...")
		close(done)
	}()

	wg.Wait()
	printFinalStats()
}

func parseFlags() Config {
	config := Config{}

	flag.StringVar(&config.DialogsURL, "url", "http://localhost:8080", "Dialogs service URL")
	flag.IntVar(&config.Workers, "workers", 10, "Number of concurrent workers")
	flag.IntVar(&config.Duration, "duration", 60, "Test duration in seconds (0 for infinite)")
	flag.IntVar(&config.MessageCount, "messages", 0, "Total messages to send (0 for infinite)")
	flag.Int64Var(&config.UserIDFrom, "user-from", 1, "Starting user ID range")
	flag.Int64Var(&config.UserIDTo, "user-to", 100, "Ending user ID range")
	flag.IntVar(&config.RequestsPerSec, "rps", 100, "Requests per second target")

	flag.Parse()
	return config
}

func worker(id int, config Config, requestsPerSec int, done chan bool, wg *sync.WaitGroup) {
	defer wg.Done()

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	ticker := time.NewTicker(time.Second / time.Duration(requestsPerSec))
	defer ticker.Stop()

	messagesSent := 0

	for {
		select {
		case <-done:
			log.Printf("Worker %d stopping, sent %d messages", id, messagesSent)
			return
		case <-ticker.C:
			if config.MessageCount > 0 && int(atomic.LoadInt64(&stats.TotalRequests)) >= config.MessageCount {
				return
			}

			operations := []string{"send_message", "get_messages", "mark_as_read", "get_stats"}
			operation := operations[rand.Intn(len(operations))]

			fromUser := randUserID(config.UserIDFrom, config.UserIDTo)
			toUser := randUserID(config.UserIDFrom, config.UserIDTo)
			for toUser == fromUser {
				toUser = randUserID(config.UserIDFrom, config.UserIDTo)
			}

			start := time.Now()
			var err error

			switch operation {
			case "send_message":
				err = sendMessage(client, config.DialogsURL, fromUser, toUser)
				if err == nil {
					messagesSent++
				}
			case "get_messages":
				err = getMessages(client, config.DialogsURL, fromUser, toUser)
			case "mark_as_read":
				err = markAsRead(client, config.DialogsURL, fromUser, toUser)
			case "get_stats":
				err = getStats(client, config.DialogsURL, fromUser, toUser)
			}

			duration := time.Since(start)

			atomic.AddInt64(&stats.TotalRequests, 1)
			atomic.AddInt64(&stats.TotalDuration, duration.Milliseconds())

			if err != nil {
				atomic.AddInt64(&stats.FailedRequests, 1)
			} else {
				atomic.AddInt64(&stats.SuccessRequests, 1)
			}
		}
	}
}

func sendMessage(client *http.Client, baseURL string, fromUser, toUser int64) error {
	url := fmt.Sprintf("%s/dialog/%d/send", baseURL, toUser)

	messages := []string{
		"Hello!",
		"How are you?",
		"Test message",
		"Lorem ipsum dolor sit amet",
		"This is a monitoring test",
		"Checking Prometheus metrics",
		"Zabbix should see this",
		"Performance testing in progress",
	}

	reqBody := SendMessageRequest{
		Text: messages[rand.Intn(len(messages))],
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", fmt.Sprintf("%d", fromUser))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func getMessages(client *http.Client, baseURL string, fromUser, toUser int64) error {
	url := fmt.Sprintf("%s/dialog/%d/list?limit=20&offset=0", baseURL, toUser)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-User-ID", fmt.Sprintf("%d", fromUser))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func markAsRead(client *http.Client, baseURL string, fromUser, toUser int64) error {
	url := fmt.Sprintf("%s/dialog/%d/read", baseURL, toUser)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-User-ID", fmt.Sprintf("%d", fromUser))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func getStats(client *http.Client, baseURL string, fromUser, toUser int64) error {
	url := fmt.Sprintf("%s/dialog/%d/stats", baseURL, toUser)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-User-ID", fmt.Sprintf("%d", fromUser))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

func randUserID(from, to int64) int64 {
	return from + rand.Int63n(to-from+1)
}

func printStats() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		total := atomic.LoadInt64(&stats.TotalRequests)
		success := atomic.LoadInt64(&stats.SuccessRequests)
		failed := atomic.LoadInt64(&stats.FailedRequests)
		totalDuration := atomic.LoadInt64(&stats.TotalDuration)

		var avgLatency int64
		if total > 0 {
			avgLatency = totalDuration / total
		}

		var successRate float64
		if total > 0 {
			successRate = float64(success) / float64(total) * 100
		}

		log.Printf("[STATS] Total: %d | Success: %d | Failed: %d | Success Rate: %.2f%% | Avg Latency: %dms",
			total, success, failed, successRate, avgLatency)
	}
}

func printFinalStats() {
	total := atomic.LoadInt64(&stats.TotalRequests)
	success := atomic.LoadInt64(&stats.SuccessRequests)
	failed := atomic.LoadInt64(&stats.FailedRequests)
	totalDuration := atomic.LoadInt64(&stats.TotalDuration)

	var avgLatency int64
	if total > 0 {
		avgLatency = totalDuration / total
	}

	var successRate float64
	if total > 0 {
		successRate = float64(success) / float64(total) * 100
	}

	log.Println("\n========== FINAL STATISTICS ==========")
	log.Printf("Total Requests:     %d", total)
	log.Printf("Successful:         %d", success)
	log.Printf("Failed:             %d", failed)
	log.Printf("Success Rate:       %.2f%%", successRate)
	log.Printf("Average Latency:    %dms", avgLatency)
	log.Println("======================================")
}
