package tests

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPersistentIdempotency(t *testing.T) {
	targetURL := "http://localhost:7777/wagering/transactions"
	idempotencyKey := "provider-a:idempotency-test-001"

	payload := map[string]any{
		"providerId":            "provider-a",
		"externalTransactionId": "idempotency-test-001",
		"playerId":              "player-test-1",
		"walletId":              "0192f291-27dd-7d3f-8071-5f8685deef37",
		"roundId":               "round-idemp-1",
		"gameId":                "chimp-mines",
		"kind":                  "BET",
		"money": map[string]string{
			"amount":   "25.00",
			"currency": "BRL",
		},
	}

	body, _ := json.Marshal(payload)

	// First request
	req1, _ := http.NewRequest("POST", targetURL, bytes.NewBuffer(body))
	req1.Header.Set("Content-Type", "application/json")
	// Rand bearer gen for debug, real systems would use the keycload on compose to control IAM
	req1.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJjbGllbnRJZCI6InByb3ZpZGVyLWEiLCJhenAiOiJwcm92aWRlci1hIiwic3ViIjoicHJvdmlkZXItYSJ9.q6UEXcGKl_oApYBLgU01cBYlIhgU6QkkkrLfq1XZpU4")
	req1.Header.Set("Idempotency-Key", idempotencyKey)

	client := &http.Client{}
	resp1, err := client.Do(req1)
	require.NoError(t, err)
	defer resp1.Body.Close()

	// Second request with exact same key and payload
	req2, _ := http.NewRequest("POST", targetURL, bytes.NewBuffer(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJjbGllbnRJZCI6InByb3ZpZGVyLWEiLCJhenAiOiJwcm92aWRlci1hIiwic3ViIjoicHJvdmlkZXItYSJ9.q6UEXcGKl_oApYBLgU01cBYlIhgU6QkkkrLfq1XZpU4")
	req2.Header.Set("Idempotency-Key", idempotencyKey)

	resp2, err := client.Do(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, http.StatusOK, resp2.StatusCode)

	var result map[string]any
	_ = json.NewDecoder(resp2.Body).Decode(&result)

	assert.Equal(t, true, result["idempotentReplay"], "subsequent requests with same key must return idempotentReplay: true")
}
