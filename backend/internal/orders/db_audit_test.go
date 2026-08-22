package orders

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAuditDatabaseState(t *testing.T) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:sudesh@localhost:5432/oms?sslmode=disable"
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	// 1. Companies
	fmt.Println("=== COMPANIES ===")
	compRows, err := pool.Query(ctx, "SELECT id, name, created_at FROM companies ORDER BY created_at ASC")
	if err != nil {
		t.Fatalf("companies query failed: %v", err)
	}
	defer compRows.Close()
	var defaultCompanyID string
	for compRows.Next() {
		var id, name string
		var createdAt any
		if err := compRows.Scan(&id, &name, &createdAt); err != nil {
			t.Fatalf("scan company failed: %v", err)
		}
		fmt.Printf("Company: ID=%s Name=%s CreatedAt=%v\n", id, name, createdAt)
		if defaultCompanyID == "" {
			defaultCompanyID = id
		}
	}

	// 2. Counts by table for default company and overall
	tables := []string{"products", "customers", "orders", "courier_locations", "couriers", "follow_ups", "order_items", "users", "import_runs", "import_uploads"}
	fmt.Println("\n=== TABLE COUNTS ===")
	for _, tbl := range tables {
		var countTotal int
		_ = pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s", tbl)).Scan(&countTotal)
		var countCompany int
		_ = pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE company_id = $1", tbl), defaultCompanyID).Scan(&countCompany)
		fmt.Printf("Table: %-20s Total in DB: %-5d Default Company (%s): %-5d\n", tbl, countTotal, defaultCompanyID, countCompany)
	}

	// 3. List all products in detail
	fmt.Println("\n=== ALL PRODUCTS IN DATABASE ===")
	prodRows, err := pool.Query(ctx, `
		SELECT p.id, p.name, p.price, p.available_qty, p.warehouse_qty, p.created_at, p.company_id, COALESCE(c.name, '') as company_name
		FROM products p
		LEFT JOIN companies c ON c.id = p.company_id
		ORDER BY p.created_at ASC, p.name ASC
	`)
	if err != nil {
		t.Fatalf("products query failed: %v", err)
	}
	defer prodRows.Close()
	pCount := 0
	for prodRows.Next() {
		pCount++
		var id, name, compID, compName string
		var price any
		var createdAt any
		var availQty, whQty int
		if err := prodRows.Scan(&id, &name, &price, &availQty, &whQty, &createdAt, &compID, &compName); err != nil {
			t.Fatalf("scan product failed: %v", err)
		}
		fmt.Printf("[%02d] Name: %-35s Price: %-10v Avail: %-4d WH: %-4d Company: %s (%s) ID: %s Created: %v\n",
			pCount, name, price, availQty, whQty, compName, compID, id, createdAt)
	}

	// 4. Users in default company
	fmt.Println("\n=== USERS IN DATABASE ===")
	userRows, err := pool.Query(ctx, "SELECT u.id, u.name, u.phone, u.role, u.company_id, c.name FROM users u LEFT JOIN companies c ON c.id = u.company_id")
	if err == nil {
		defer userRows.Close()
		for userRows.Next() {
			var id, name, phone, role, compID, compName string
			_ = userRows.Scan(&id, &name, &phone, &role, &compID, &compName)
			fmt.Printf("User: %-20s Phone: %-15s Role: %-12s Company: %s (%s) ID: %s\n", name, phone, role, compName, compID, id)
		}
	}
}
