package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrWalletNotFound = errors.New("wallet not found")
	ErrInsufficient   = errors.New("insufficient funds")
)

type WalletRepository struct {
	db *pgxpool.Pool
}

func NewWalletRepository(db *pgxpool.Pool) *WalletRepository {
	return &WalletRepository{db: db}
}

func (r *WalletRepository) GetWalletForUpdate(ctx context.Context, tx pgx.Tx, walletID string) (balance int64, version int, err error) {
	query := `SELECT balance, version FROM wallets WHERE id = $1 FOR UPDATE`

	err = tx.QueryRow(ctx, query, walletID).Scan(&balance, &version)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, 0, ErrWalletNotFound
		}
		return 0, 0, fmt.Errorf("failed to lock wallet: %w", err)
	}

	return balance, version, nil
}

func (r *WalletRepository) UpdateBalanceAndVersion(ctx context.Context, tx pgx.Tx, walletID string, newBalance int64, currentVersion int) error {
	query := `
		UPDATE wallets 
		SET balance = $1, version = version + 1, updated_at = NOW() 
		WHERE id = $2 AND version = $3
	`
	tag, err := tx.Exec(ctx, query, newBalance, walletID, currentVersion)
	if err != nil {
		return fmt.Errorf("failed to update wallet balance: %w", err)
	}

	if tag.RowsAffected() == 0 {
		return errors.New("concurrent update conflict detected on wallet version")
	}

	return nil
}
