package orders

import (
	"context"
	"os"
	"testing"

	"oms-backend/internal/db/connection"
	db "oms-backend/internal/db/generated"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestCreateOrderWithItemsWarnsOnInsufficientStock(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	pool, err := connection.NewPool(databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()

	customerPhone := "9" + uuid.NewString()[:18]
	productName := "test-product-" + uuid.NewString()

	var customerID uuid.UUID
	err = pool.QueryRow(
		ctx,
		`INSERT INTO customers (phone, name, address)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		customerPhone,
		"Transaction Test Customer",
		"Test Address",
	).Scan(&customerID)
	if err != nil {
		t.Fatalf("create test customer: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM customers WHERE id = $1`,
			customerID,
		)
	}()

	var productID uuid.UUID
	err = pool.QueryRow(
		ctx,
		`INSERT INTO products (name, price, available_qty, warehouse_qty)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		productName,
		"100.00",
		1,
		10,
	).Scan(&productID)
	if err != nil {
		t.Fatalf("create test product: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM products WHERE id = $1`,
			productID,
		)
	}()

	var userID uuid.UUID
	err = pool.QueryRow(
		ctx,
		`SELECT id FROM users LIMIT 1`,
	).Scan(&userID)
	if err != nil {
		t.Skip("no users available in database")
	}

	var codAmount pgtype.Numeric
	if err := codAmount.Scan("750.00"); err != nil {
		t.Fatalf("create test COD amount: %v", err)
	}

	service := NewService(pool)

	input := CreateOrderInput{
		CustomerID:   customerID,
		Source:       db.OrderSourcePhone,
		Address:      "Test Address",
		CodAmount:    codAmount,
		IsStoreVisit: false,
		CreatedBy:    userID,
		Items: []CreateOrderItem{
			{
				ProductID: productID,
				Quantity:  2,
			},
		},
	}

	createdOrder, err := service.CreateOrderWithItems(ctx, input, false)
	if err != nil {
		t.Fatalf("expected order creation to succeed with insufficient stock warning: %v", err)
	}

	var orderCount int
	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
		 FROM orders
		 WHERE customer_id = $1`,
		customerID,
	).Scan(&orderCount)
	if err != nil {
		t.Fatalf("check order rollback: %v", err)
	}

	if orderCount != 1 {
		t.Fatalf("expected 1 order despite insufficient stock, got %d", orderCount)
	}

	var availableQty int32
	err = pool.QueryRow(
		ctx,
		`SELECT available_qty
		 FROM products
		 WHERE id = $1`,
		productID,
	).Scan(&availableQty)
	if err != nil {
		t.Fatalf("check product stock: %v", err)
	}

	if availableQty != 1 {
		t.Fatalf(
			"expected stock to remain 1 after rollback, got %d",
			availableQty,
		)
	}

	var itemCount int
	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*)
		 FROM order_items oi
		 JOIN orders o ON o.id = oi.order_id
		 WHERE o.customer_id = $1`,
		customerID,
	).Scan(&itemCount)
	if err != nil {
		t.Fatalf("check order item rollback: %v", err)
	}

	if itemCount != 1 {
		t.Fatalf("expected 1 order item despite insufficient stock, got %d", itemCount)
	}

	if createdOrder.ID.Bytes == [16]byte{} {
		t.Fatal("expected created order id")
	}
}

func TestCreateOrderWithItemsSucceedsAndDecrementsStock(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is not set")
	}

	pool, err := connection.NewPool(databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()

	customerPhone := "8" + uuid.NewString()[:18]
	productName := "test-product-" + uuid.NewString()

	var customerID uuid.UUID
	err = pool.QueryRow(
		ctx,
		`INSERT INTO customers (phone, name, address)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		customerPhone,
		"Successful Order Test Customer",
		"Test Address",
	).Scan(&customerID)
	if err != nil {
		t.Fatalf("create test customer: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM customers WHERE id = $1`,
			customerID,
		)
	}()

	var productID uuid.UUID
	var originalQty int32
	var productPrice string

	err = pool.QueryRow(
		ctx,
		`INSERT INTO products (name, price, available_qty, warehouse_qty)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, available_qty, price::text`,
		productName,
		"250.00",
		10,
		20,
	).Scan(&productID, &originalQty, &productPrice)
	if err != nil {
		t.Fatalf("create test product: %v", err)
	}

	defer func() {
		_, _ = pool.Exec(
			ctx,
			`DELETE FROM products WHERE id = $1`,
			productID,
		)
	}()

	var userID uuid.UUID
	err = pool.QueryRow(
		ctx,
		`SELECT id FROM users LIMIT 1`,
	).Scan(&userID)
	if err != nil {
		t.Skip("no users available in database")
	}

	var codAmount pgtype.Numeric
	if err := codAmount.Scan("750.00"); err != nil {
		t.Fatalf("create test COD amount: %v", err)
	}

	service := NewService(pool)

	input := CreateOrderInput{
		CustomerID:   customerID,
		Source:       db.OrderSourcePhone,
		Address:      "Test Address",
		CodAmount:    codAmount,
		IsStoreVisit: false,
		CreatedBy:    userID,
		Items: []CreateOrderItem{
			{
				ProductID: productID,
				Quantity:  3,
			},
		},
	}

	order, err := service.CreateOrderWithItems(ctx, input, false)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	if !order.ID.Valid {
		t.Fatal("expected created order to have a valid ID")
	}

	var availableQty int32
	err = pool.QueryRow(
		ctx,
		`SELECT available_qty
		 FROM products
		 WHERE id = $1`,
		productID,
	).Scan(&availableQty)
	if err != nil {
		t.Fatalf("check product stock: %v", err)
	}

	expectedQty := originalQty - 3

	if availableQty != expectedQty {
		t.Fatalf(
			"expected available_qty to be %d, got %d",
			expectedQty,
			availableQty,
		)
	}

	var itemCount int
	var itemQuantity int32
	var itemPrice string

	err = pool.QueryRow(
		ctx,
		`SELECT COUNT(*), COALESCE(MAX(quantity), 0), COALESCE(MAX(price)::text, '')
		 FROM order_items
		 WHERE order_id = $1`,
		order.ID,
	).Scan(
		&itemCount,
		&itemQuantity,
		&itemPrice,
	)
	if err != nil {
		t.Fatalf("check order item: %v", err)
	}

	if itemCount != 1 {
		t.Fatalf("expected 1 order item, got %d", itemCount)
	}

	if itemQuantity != 3 {
		t.Fatalf(
			"expected order item quantity to be 3, got %d",
			itemQuantity,
		)
	}

	if itemPrice != productPrice {
		t.Fatalf(
			"expected order item price %s, got %s",
			productPrice,
			itemPrice,
		)
	}

	_, err = pool.Exec(
		ctx,
		`DELETE FROM orders WHERE id = $1`,
		order.ID,
	)
	if err != nil {
		t.Fatalf("cleanup test order: %v", err)
	}
}
