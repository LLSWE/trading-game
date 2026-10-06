package tests

import (
	"context"
	"net/http"
	"os"
	"testing"

	"github.com/LLSWE/trading-game/internal/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testDB *pgxpool.Pool

func TestMain(m *testing.M) {
	config.Load()
	ctx := context.Background()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://test_postgres:postgres123@localhost:5432/wagering_postgres?sslmode=disable"
	}

	var err error
	testDB, err = pgxpool.New(ctx, dsn)
	if err != nil {
		panic("failed to connect to test database: " + err.Error())
	}
	defer testDB.Close()

	os.Exit(m.Run())
}

func TestDatabaseHealth(t *testing.T) {
	ctx := context.Background()
	err := testDB.Ping(ctx)
	assert.NoError(t, err, "database should be up and reachable for integration tests")
}

func TestHealthCheckEndpoint(t *testing.T) {
	resp, err := http.Get("http://localhost:7777/health/live")
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
