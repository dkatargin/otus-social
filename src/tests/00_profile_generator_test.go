package tests

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/brianvoe/gofakeit/v7"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
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
	const total = 800000
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
			resp, err := http.Post(fmt.Sprintf("%s/auth/register", ApiBaseUrl), "application/json", bytes.NewBuffer(body))

			if err != nil {
				var response map[string]interface{}
				errBodyDecode := json.NewDecoder(resp.Body).Decode(&response)
				if errBodyDecode != nil {
					t.Errorf("Failed to decode response for request %d: %v", i, err)
				} else {
					t.Errorf("Request failed with error %d: %v", i, response)
				}
				return
			}
			if resp.StatusCode != http.StatusOK {
				t.Errorf("Request failed with error: %d", resp.StatusCode)
				return
			}

		}(i)
	}
	wg.Wait()
}
