package orders

import (
	"context"
	"fmt"

	db "oms-backend/internal/db/generated"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type CreateOrderItem struct {
	ProductID uuid.UUID
	Quantity  int32
}

type CreateOrderInput struct {
	CustomerID   uuid.UUID
	Source       db.OrderSource
	Address      string
	CodAmount    pgtype.Numeric
	IsStoreVisit bool
	CreatedBy    uuid.UUID
	Items        []CreateOrderItem
}

func (s *Service) CreateOrderWithItems(
	ctx context.Context,
	input CreateOrderInput,
	isStaff bool,
) (db.Order, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return db.Order{}, fmt.Errorf("BEGIN TRANSACTION: %w", err)
	}
	defer tx.Rollback(ctx)

	queries := db.New(s.Pool).WithTx(tx)

	customerID := pgtype.UUID{
		Bytes: input.CustomerID,
		Valid: true,
	}

	createdBy := pgtype.UUID{
		Bytes: input.CreatedBy,
		Valid: true,
	}

	var order db.Order

	if isStaff {
		staffOrder, err := queries.CreateOrderForStaff(
			ctx,
			db.CreateOrderForStaffParams{
				CustomerID:   customerID,
				Source:       input.Source,
				Status:       db.OrderStatusConfirmed,
				Address:      input.Address,
				IsStoreVisit: input.IsStoreVisit,
				CreatedBy:    createdBy,
			},
		)
		if err != nil {
			return db.Order{}, fmt.Errorf("CREATE STAFF ORDER: %w", err)
		}

		order = db.Order{
			ID:           staffOrder.ID,
			CustomerID:   staffOrder.CustomerID,
			Source:       staffOrder.Source,
			Status:       staffOrder.Status,
			CourierID:    staffOrder.CourierID,
			LocationID:   staffOrder.LocationID,
			Address:      staffOrder.Address,
			IsStoreVisit: staffOrder.IsStoreVisit,
			CreatedBy:    staffOrder.CreatedBy,
			CreatedAt:    staffOrder.CreatedAt,
			UpdatedAt:    staffOrder.UpdatedAt,
		}
	} else {
		order, err = queries.CreateOrderForAdmin(
			ctx,
			db.CreateOrderForAdminParams{
				CustomerID:   customerID,
				Source:       input.Source,
				Status:       db.OrderStatusConfirmed,
				Address:      input.Address,
				CodAmount:    input.CodAmount,
				IsStoreVisit: input.IsStoreVisit,
				CreatedBy:    createdBy,
			},
		)
		if err != nil {
			return db.Order{}, fmt.Errorf("CREATE ADMIN ORDER: %w", err)
		}
	}

	for _, item := range input.Items {
		productID := pgtype.UUID{
			Bytes: item.ProductID,
			Valid: true,
		}

		product, err := queries.DecreaseProductAvailableQty(
			ctx,
			db.DecreaseProductAvailableQtyParams{
				ID:           productID,
				AvailableQty: item.Quantity,
			},
		)
		if err != nil {
			if err == pgx.ErrNoRows {
				return db.Order{}, fmt.Errorf(
					"DECREASE STOCK: insufficient stock or product not found: %s",
					item.ProductID,
				)
			}

			return db.Order{}, fmt.Errorf("DECREASE STOCK: %w", err)
		}

		_, err = queries.CreateOrderItem(
			ctx,
			db.CreateOrderItemParams{
				OrderID:   order.ID,
				ProductID: product.ID,
				Quantity:  item.Quantity,
				Price:     product.Price,
			},
		)
		if err != nil {
			return db.Order{}, fmt.Errorf("CREATE ORDER ITEM: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Order{}, fmt.Errorf("COMMIT TRANSACTION: %w", err)
	}

	return order, nil
}
