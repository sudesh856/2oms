package connection

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestDatabaseConnection(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	pool, err := NewPool(databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("database ping failed: %v", err)
	}

	var exists bool
	err = pool.QueryRow(
		ctx,
		`SELECT EXISTS (
			SELECT 1
			FROM information_schema.columns
			WHERE table_name = 'products'
			AND column_name = 'warehouse_qty'
		)`,
	).Scan(&exists)

	if err != nil {
		t.Fatalf("failed to check warehouse column: %v", err)
	}

	if !exists {
		t.Fatal("products.warehouse column does not exist")
	}
}
