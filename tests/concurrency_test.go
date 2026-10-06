package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConcurrencyWagers(t *testing.T) {
	targetURL := "http://localhost:7777/wagering/transactions"
	concurrency := 10
	var wg sync.WaitGroup
	wg.Add(concurrency)

	results := make(chan int, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(idx int) {
			defer wg.Done()

			payload := map[string]any{
				"providerId":            "provider-a",
				"externalTransactionId": "concurrent-tx-" + string(rune(idx)),
				"playerId":              "player-test-1",
				"walletId":              "0192f291-27dd-7d3f-8071-5f8685deef37",
				"roundId":               "round-test-1",
				"gameId":                "fortune-chimp",
				"kind":                  "BET",
				"money": map[string]string{
					"amount":   "10.00",
					"currency": "BRL",
				},
			}

			body, _ := json.Marshal(payload)
			req, _ := http.NewRequest("POST", targetURL, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJjbGllbnRJZCI6InByb3ZpZGVyLWEiLCJhenAiOiJwcm92aWRlci1hIiwic3ViIjoicHJvdmlkZXItYSJ9.q6UEXcGKl_oApYBLgU01cBYlIhgU6QkkkrLfq1XZpU4")
			req.Header.Set("Idempotency-Key", "provider-a:concurrent-tx-"+string(rune(idx)))

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				results <- 0
				return
			}
			defer resp.Body.Close()

			results <- resp.StatusCode
		}(i)
	}

	wg.Wait()
	close(results)

	successCount := 0
	for code := range results {
		if code == http.StatusOK {
			successCount++
		}
	}

	assert.True(t, successCount > 0, "at least one request should succeed under concurrency")
}
