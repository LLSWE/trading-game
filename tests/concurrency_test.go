package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const TestBearerToken = "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJjbGllbnRJZCI6InByb3ZpZGVyLWEiLCJhenAiOiJwcm92aWRlci1hIiwic3ViIjoicHJvdmlkZXItYSJ9.q6UEXcGKl_oApYBLgU01cBYlIhgU6QkkkrLfq1XZpU4"

func TestConcurrencyWagers(t *testing.T) {
	ctx := context.Background()

	_, err := testDB.Exec(ctx, `
		INSERT INTO wallets (id, player_id, currency, balance, version)
		VALUES ('0192f291-27dd-7d3f-8071-5f8685deef37', 'player-concurrency', 'BRL', 10000, 1)
		ON CONFLICT (id) DO UPDATE SET balance = 10000, version = 1
	`)
	require.NoError(t, err)

	targetURL := "http://localhost:7777/wagering/transactions"
	var wg sync.WaitGroup
	wg.Add(2)

	results := make(chan int, 2)

	bets := []string{"tx-concurrency-01", "tx-concurrency-02"}

	for i, extID := range bets {
		go func(id string, idx int) {
			defer wg.Done()

			payload := map[string]any{
				"providerId":            "provider-a",
				"externalTransactionId": id,
				"playerId":              "player-concurrency",
				"walletId":              "0192f291-27dd-7d3f-8071-5f8685deef37",
				"roundId":               "round-conc-1",
				"gameId":                "fortune-chimp",
				"kind":                  "BET",
				"money": map[string]string{
					"amount":   "80.00",
					"currency": "BRL",
				},
			}

			body, _ := json.Marshal(payload)
			req, _ := http.NewRequest("POST", targetURL, bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", TestBearerToken)
			req.Header.Set("Idempotency-Key", "provider-a:"+id)

			client := &http.Client{}
			resp, err := client.Do(req)
			if err != nil {
				results <- 0
				return
			}
			defer resp.Body.Close()

			results <- resp.StatusCode
		}(extID, i)
	}

	wg.Wait()
	close(results)

	successCount := 0
	badRequestOrConflict := 0
	for code := range results {
		if code == http.StatusOK {
			successCount++
		} else if code == http.StatusBadRequest || code == http.StatusUnprocessableEntity || code == 422 {
			badRequestOrConflict++
		}
	}

	assert.Equal(t, 1, successCount, "exactly one wager of 80.00 should succeed")
}
