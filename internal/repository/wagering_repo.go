package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WageringRepository struct {
	db *pgxpool.Pool
}

func NewWageringRepository(db *pgxpool.Pool) *WageringRepository {
	return &WageringRepository{db: db}
}

type TransactionDTO struct {
	ID       string
	Kind     string
	Amount   int64
	Currency string
	Status   string
}

func (r *WageringRepository) FindByExternalID(ctx context.Context, tx pgx.Tx, providerID string, externalTxID string) (TransactionDTO, error) {
	query := `
		SELECT id, kind, amount, currency, status 
		FROM wagering_transactions 
		WHERE provider_id = $1 AND external_transaction_id = $2
	`
	var t TransactionDTO
	err := tx.QueryRow(ctx, query, providerID, externalTxID).Scan(&t.ID, &t.Kind, &t.Amount, &t.Currency, &t.Status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return TransactionDTO{}, fmt.Errorf("transaction not found")
		}
		return TransactionDTO{}, fmt.Errorf("failed to find transaction by external id: %w", err)
	}
	return t, nil
}
