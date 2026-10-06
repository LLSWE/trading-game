package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/LLSWE/trading-game/internal/domain"
	"github.com/LLSWE/trading-game/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrDuplicateIdempotency = errors.New("idempotency key already used with different payload")
	ErrInsufficientBalance  = errors.New("insufficient balance for wager")
)

type WagerRequest struct {
	ProviderID            string       `json:"providerId"`
	ExternalTransactionID string       `json:"externalTransactionId"`
	IdempotencyKey        string       `json:"idempotencyKey"`
	PlayerID              string       `json:"playerId"`
	WalletID              string       `json:"walletId"`
	RoundID               string       `json:"roundId"`
	GameID                string       `json:"gameId"`
	Kind                  string       `json:"kind"`
	Money                 domain.Money `json:"money"`
}

type WagerResponse struct {
	TransactionID    string       `json:"transactionId"`
	Status           string       `json:"status"`
	Balance          domain.Money `json:"balance"`
	IdempotentReplay bool         `json:"idempotentReplay"`
}

type WageringUseCase struct {
	db         *pgxpool.Pool
	walletRepo *repository.WalletRepository
}

func NewWageringUseCase(db *pgxpool.Pool, walletRepo *repository.WalletRepository) *WageringUseCase {
	return &WageringUseCase{
		db:         db,
		walletRepo: walletRepo,
	}
}

func (uc *WageringUseCase) ProcessTransaction(ctx context.Context, req WagerRequest) (WagerResponse, error) {
	payloadBytes, err := json.Marshal(map[string]any{
		"providerId":            req.ProviderID,
		"externalTransactionId": req.ExternalTransactionID,
		"playerId":              req.PlayerID,
		"walletId":              req.WalletID,
		"roundId":               req.RoundID,
		"gameId":                req.GameID,
		"kind":                  req.Kind,
		"money":                 req.Money,
	})
	if err != nil {
		return WagerResponse{}, fmt.Errorf("failed to marshal payload for hash: %w", err)
	}
	hashSum := sha256.Sum256(payloadBytes)
	payloadHash := fmt.Sprintf("%x", hashSum)

	tx, err := uc.db.Begin(ctx)
	if err != nil {
		return WagerResponse{}, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var existingID, existingStatus string
	var existingHash string
	var responsePayload []byte

	idempotencyQuery := `
		SELECT id, status, payload_hash, response_payload 
		FROM wagering_transactions 
		WHERE idempotency_key = $1 FOR UPDATE
	`
	err = tx.QueryRow(ctx, idempotencyQuery, req.IdempotencyKey).Scan(&existingID, &existingStatus, &existingHash, &responsePayload)
	if err == nil {

		if existingHash != payloadHash {
			return WagerResponse{}, ErrDuplicateIdempotency
		}

		if existingStatus == "PROCESSED" && responsePayload != nil {
			var cachedResp WagerResponse
			if err := json.Unmarshal(responsePayload, &cachedResp); err == nil {
				cachedResp.IdempotentReplay = true
				_ = tx.Commit(ctx)
				return cachedResp, nil
			}
		}
	}

	currentBalance, currentVersion, err := uc.walletRepo.GetWalletForUpdate(ctx, tx, req.WalletID)
	if err != nil {
		return WagerResponse{}, err
	}

	var newBalance int64
	var ledgerDirection string

	if req.Kind == "BET" {
		if currentBalance < req.Money.Amount() {
			return WagerResponse{}, ErrInsufficientBalance
		}
		newBalance = currentBalance - req.Money.Amount()
		ledgerDirection = "DEBIT"
	} else if req.Kind == "WIN" {
		newBalance = currentBalance + req.Money.Amount()
		ledgerDirection = "CREDIT"
	} else {
		return WagerResponse{}, fmt.Errorf("unsupported transaction kind: %s", req.Kind)
	}

	if err := uc.walletRepo.UpdateBalanceAndVersion(ctx, tx, req.WalletID, newBalance, currentVersion); err != nil {
		return WagerResponse{}, err
	}

	var transactionID string
	txInsertQuery := `
		INSERT INTO wagering_transactions (
			provider_id, external_transaction_id, idempotency_key, payload_hash, 
			player_id, wallet_id, round_id, game_id, kind, amount, currency, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id
	`
	err = tx.QueryRow(ctx, txInsertQuery,
		req.ProviderID, req.ExternalTransactionID, req.IdempotencyKey, payloadHash,
		req.PlayerID, req.WalletID, req.RoundID, req.GameID, req.Kind,
		req.Money.Amount(), req.Money.Currency(), "PROCESSED",
	).Scan(&transactionID)
	if err != nil {
		return WagerResponse{}, fmt.Errorf("failed to insert transaction: %w", err)
	}

	ledgerInsertQuery := `
		INSERT INTO wallet_ledger_entries (
			wallet_id, transaction_id, direction, amount, balance_before, balance_after
		) VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err = tx.Exec(ctx, ledgerInsertQuery,
		req.WalletID, transactionID, ledgerDirection, req.Money.Amount(), currentBalance, newBalance,
	)
	if err != nil {
		return WagerResponse{}, fmt.Errorf("failed to insert ledger entry: %w", err)
	}

	updatedMoneyBalance, err := domain.NewMoneyFromCents(newBalance, req.Money.Currency())
	if err != nil {
		return WagerResponse{}, fmt.Errorf("failed to format updated balance: %w", err)
	}

	resp := WagerResponse{
		TransactionID:    transactionID,
		Status:           "PROCESSED",
		Balance:          updatedMoneyBalance,
		IdempotentReplay: false,
	}

	responseBytes, _ := json.Marshal(resp)
	_, _ = tx.Exec(ctx, `UPDATE wagering_transactions SET response_payload = $1 WHERE id = $2`, responseBytes, transactionID)

	eventPayload, _ := json.Marshal(map[string]any{
		"eventId":     transactionID,
		"eventType":   "WagerTransactionProcessed",
		"aggregateId": req.WalletID,
		"occurredAt":  time.Now().UTC().Format(time.RFC3339),
		"version":     1,
		"data":        resp,
	})
	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events (aggregate_id, event_type, payload, status)
		VALUES ($1, $2, $3, 'PENDING')
	`, req.WalletID, "WagerTransactionProcessed", eventPayload)
	if err != nil {
		return WagerResponse{}, fmt.Errorf("failed to insert outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return WagerResponse{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return resp, nil
}
