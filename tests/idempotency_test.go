package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPersistentIdempotency(t *testing.T) {
	ctx := context.Background()
	_, err := testDB.Exec(ctx, `
		INSERT INTO wallets (id, player_id, currency, balance, version)
		VALUES ('0192f291-27dd-7d3f-8071-5f8685deef37', 'player-idemp', 'BRL', 50000, 1)
		ON CONFLICT (id) DO UPDATE SET balance = 50000
	`)
	require.NoError(t, err)

	targetURL := "http://localhost:7777/wagering/transactions"
	uniqueKey := "idemp-test-" + time.Now().Format("20060102150405.000")
	idempotencyKey := "provider-a:" + uniqueKey
	externalTxID := "ext-" + uniqueKey

	payload := map[string]any{
		"providerId":            "provider-a",
		"externalTransactionId": externalTxID,
		"playerId":              "player-idemp",
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

	req1, _ := http.NewRequest("POST", targetURL, bytes.NewBuffer(body))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", TestBearerToken)
	req1.Header.Set("Idempotency-Key", idempotencyKey)

	client := &http.Client{}
	resp1, err := client.Do(req1)
	require.NoError(t, err)
	defer resp1.Body.Close()
	assert.Equal(t, http.StatusOK, resp1.StatusCode)

	req2, _ := http.NewRequest("POST", targetURL, bytes.NewBuffer(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", TestBearerToken)
	req2.Header.Set("Idempotency-Key", idempotencyKey)

	resp2, err := client.Do(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusOK, resp2.StatusCode)

	var result map[string]any
	_ = json.NewDecoder(resp2.Body).Decode(&result)
	assert.Equal(t, true, result["idempotentReplay"])
}
