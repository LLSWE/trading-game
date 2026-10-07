package repository

import (
	"context"
	"fmt"

	"github.com/LLSWE/trading-game/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
)

func NewDatabasePool(lc fx.Lifecycle, cfg *config.Config) (*pgxpool.Pool, error) {
	ctx := context.Background()
	poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database url: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create db pool: %w", err)
	}

	lc.Append(fx.Hook{
		OnStart: func(c context.Context) error {
			return pool.Ping(c)
		},
		OnStop: func(c context.Context) error {
			pool.Close()
			return nil
		},
	})

	return pool, nil
}
