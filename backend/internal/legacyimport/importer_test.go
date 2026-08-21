package legacyimport

import (
	"context"
	"os"
	"strings"
	"testing"

	"oms-backend/internal/db/connection"
	db "oms-backend/internal/db/generated"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestParseDailyFixtureNormalizesAndLogsMalformedRows(t *testing.T) {
	data := "tab,name,gbl,ncm,address,phone,phone2,product,cod,status,delivery,by,remarks\n" +
		"Aug 16,Asha,,NCM, Kathmandu ,+977 981-234-5678,,Posture Belt,1000,confirmed,,,\n" +
		"Aug 16,Bad,,,,, ,Posture Belt,1000,confirmed,,,\n"
	rows, review, err := ParseDaily(strings.NewReader(data), 0)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 valid row, got %d", len(rows))
	}
	if rows[0].Phone != "+977 981-234-5678" {
		t.Fatalf("unexpected source phone: %q", rows[0].Phone)
	}
	if NormalizePhone(rows[0].Phone) != "9812345678" {
		t.Fatalf("phone normalization failed")
	}
	if len(review) != 1 {
		t.Fatalf("expected 1 review row, got %d", len(review))
	}
}

func TestConfiguredSourceURLRequiresServerConfiguration(t *testing.T) {
	t.Setenv("LEGACY_SOURCE_URL", "")
	if source := ConfiguredSourceURL(); source != "" {
		t.Fatalf("expected no implicit historical source URL, got %q", source)
	}
	t.Setenv("LEGACY_SOURCE_URL", "https://private.example/legacy/")
	if source := ConfiguredSourceURL(); source != "https://private.example/legacy/" {
		t.Fatalf("unexpected configured source URL: %q", source)
	}
}

func TestImportDailyFixtureUsesRealDatabaseAndDeduplicatesCustomers(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}
	pool, err := connection.NewPool(databaseURL)
	if err != nil {
		t.Fatalf("connect database: %v", err)
	}
	defer pool.Close()
	ctx := context.Background()

	productName := "legacy-fixture-" + uuid.NewString()
	product, err := db.New(pool).CreateProduct(ctx, db.CreateProductParams{Name: productName, Price: numeric(t, "100"), AvailableQty: 10, WarehouseQty: 10})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	defer pool.Exec(ctx, "DELETE FROM products WHERE id = $1", product.ID)

	var userID pgtype.UUID
	if err := pool.QueryRow(ctx, "SELECT id FROM users LIMIT 1").Scan(&userID); err != nil {
		t.Skip("no users available in database")
	}

	data := "tab,name,gbl,ncm,address,phone,phone2,product,cod,status,delivery,by,remarks\n" +
		"Aug 16,Asha,,NCM,Kathmandu,9812345678,," + productName + ",100,confirmed,,,\n" +
		"Aug 15,Asha,,NCM,Kathmandu,981 234 5678,," + productName + ",100,confirmed,,,\n" +
		"Bad row,,,,,,,,,,,,\n"
	rows, review, err := ParseDaily(strings.NewReader(data), 0)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if len(review) != 1 {
		t.Fatalf("expected malformed row in review, got %d", len(review))
	}

	importer := &Importer{Pool: pool, Queries: db.New(pool), CreatedBy: userID, Source: db.OrderSourcePhone, Year: 2025, Review: review}
	counts, err := importer.ImportDaily(ctx, rows)
	if err != nil {
		t.Fatalf("import fixture: %v", err)
	}
	if counts.Inserted != 2 {
		t.Fatalf("expected 2 inserted orders, got %d", counts.Inserted)
	}

	var customerCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM customers WHERE phone = $1", "9812345678").Scan(&customerCount); err != nil {
		t.Fatal(err)
	}
	if customerCount != 1 {
		t.Fatalf("expected one deduplicated customer, got %d", customerCount)
	}
	var legacyCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM orders o JOIN customers c ON c.id = o.customer_id WHERE c.phone = $1 AND o.is_legacy", "9812345678").Scan(&legacyCount); err != nil {
		t.Fatal(err)
	}
	if legacyCount != 2 {
		t.Fatalf("expected two legacy orders, got %d", legacyCount)
	}
}

func numeric(t *testing.T, value string) pgtype.Numeric {
	t.Helper()
	result := pgtype.Numeric{}
	if err := result.Scan(value); err != nil {
		t.Fatalf("numeric: %v", err)
	}
	return result
}
