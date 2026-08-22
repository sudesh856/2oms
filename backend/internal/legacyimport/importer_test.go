package legacyimport

import (
	"bytes"
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

func TestUploadedFileMappingAndValidation(t *testing.T) {
	table, err := ParseUploadedFile("orders.csv", []byte("Customer,Mobile,Street,Item,Amount\nAsha,9812345678,Kathmandu,Belt,1000\n"))
	if err != nil {
		t.Fatalf("parse uploaded CSV: %v", err)
	}
	rows, err := MapUploadedRows(table, map[string]string{"name": "Customer", "phone": "Mobile", "address": "Street", "product": "Item", "cod_amount": "Amount"})
	if err != nil || len(rows) != 1 || rows[0].Name != "Asha" || rows[0].COD != "1000" {
		t.Fatalf("unexpected mapped row: %#v, %v", rows, err)
	}
	if _, err := ParseUploadedFile("orders.csv", []byte("name,phone,address,product,cod\n=cmd,9812345678,Address,Item,100\n")); err == nil {
		t.Fatal("formula-like CSV value was accepted")
	}
	if _, err := ParseUploadedFile("orders.txt", []byte("name\nvalue\n")); err == nil {
		t.Fatal("unsupported upload type was accepted")
	}
	if _, err := ParseUploadedFile("orders.csv", bytes.Repeat([]byte("x"), MaxUploadSize+1)); err == nil {
		t.Fatal("oversized upload was accepted")
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
	testPhone := "98" + strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	var userID pgtype.UUID
	var companyID pgtype.UUID
	if err := pool.QueryRow(ctx, "SELECT id, company_id FROM users LIMIT 1").Scan(&userID, &companyID); err != nil {
		t.Skip("no users available in database")
	}
	product, err := db.New(pool).CreateProduct(ctx, db.CreateProductParams{Name: productName, Price: numeric(t, "100"), AvailableQty: 10, WarehouseQty: 10, CompanyID: companyID})
	if err != nil {
		t.Fatalf("create product: %v", err)
	}
	defer func() {
		statements := []struct {
			query string
			arg   any
		}{
			{`DELETE FROM status_history WHERE order_id IN (SELECT id FROM orders WHERE customer_id IN (SELECT id FROM customers WHERE phone = $1))`, testPhone},
			{`DELETE FROM follow_ups WHERE order_id IN (SELECT id FROM orders WHERE customer_id IN (SELECT id FROM customers WHERE phone = $1))`, testPhone},
			{`DELETE FROM order_items WHERE product_id = $1`, product.ID},
			{`DELETE FROM orders WHERE customer_id IN (SELECT id FROM customers WHERE phone = $1)`, testPhone},
			{`DELETE FROM products WHERE id = $1`, product.ID},
			{`DELETE FROM customers WHERE phone = $1`, testPhone},
		}
		for _, statement := range statements {
			_, err := pool.Exec(ctx, statement.query, statement.arg)
			if err != nil {
				t.Errorf("cleanup fixture: %v", err)
			}
		}
	}()

	data := "tab,name,gbl,ncm,address,phone,phone2,product,cod,status,delivery,by,remarks\n" +
		"Aug 16,Asha,,NCM,Kathmandu," + testPhone + ",," + productName + ",100,confirmed,,,\n" +
		"Aug 15,Asha,,NCM,Kathmandu," + testPhone[:3] + " " + testPhone[3:] + ",," + productName + ",100,confirmed,,,\n" +
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
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM customers WHERE phone = $1", testPhone).Scan(&customerCount); err != nil {
		t.Fatal(err)
	}
	if customerCount != 1 {
		t.Fatalf("expected one deduplicated customer, got %d", customerCount)
	}
	var legacyCount int
	if err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM orders o JOIN customers c ON c.id = o.customer_id WHERE c.phone = $1 AND o.is_legacy", testPhone).Scan(&legacyCount); err != nil {
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
