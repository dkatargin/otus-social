package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
)

type RegisterRequest struct {
	Nickname  string    `json:"nickname"`
	Password  string    `json:"password"`
	Firstname string    `json:"first_name"`
	Lastname  string    `json:"last_name"`
	Birthday  time.Time `json:"birthday"`
	Sex       string    `json:"sex"`
	Interests []string  `json:"interests"`
	City      string    `json:"city"`
}

func TestProfileGenerator(t *testing.T) {
	const total = 700000
	const workers = 5 // количество одновременных запросов
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)

	for i := 0; i < total; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			name := gofakeit.FirstName()

			req := RegisterRequest{
				Nickname:  fmt.Sprintf("%s_%s", strings.ToLower(name), gofakeit.Numerify("######")),
				Firstname: name,
				Lastname:  gofakeit.LastName(),
				Password:  gofakeit.Password(true, false, true, true, false, 10),

				Birthday: gofakeit.Date(),
				Sex:      gofakeit.RandomString([]string{"male", "female"}),
				City:     gofakeit.City(),
			}
			body, _ := json.Marshal(req)
			resp, err := http.Post(fmt.Sprintf("%s/api/v1/auth/register", ApiBaseUrl), "application/json", bytes.NewBuffer(body))

			if err != nil {
				t.Errorf("HTTP request failed for request %d: %v", i, err)
				return
			}

			if resp == nil {
				t.Errorf("Received nil response for request %d", i)
				return
			}

			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				var response map[string]interface{}
				errBodyDecode := json.NewDecoder(resp.Body).Decode(&response)
				if errBodyDecode != nil {
					t.Errorf("Request failed with status %d for request %d", resp.StatusCode, i)
				} else {
					if resp.StatusCode == 400 {
						t.Logf("Request 400 %v", response)
						return
					}
					t.Errorf("Request failed with status %d for request %d: %v", resp.StatusCode, i, response)
				}
				return
			}

		}(i)
	}
	wg.Wait()
}
