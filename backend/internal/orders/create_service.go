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
	CompanyID    uuid.UUID
	Items        []CreateOrderItem
}

type StockWarning struct {
	ProductID    uuid.UUID
	ProductName  string
	RequestedQty int32
	AvailableQty int32
}

func (s *Service) CreateOrderWithItems(
	ctx context.Context,
	input CreateOrderInput,
	isStaff bool,
) (db.Order, error) {
	order, _, err := s.CreateOrderWithItemsAndWarnings(ctx, input, isStaff)
	return order, err
}

func (s *Service) CreateOrderWithItemsAndWarnings(
	ctx context.Context,
	input CreateOrderInput,
	isStaff bool,
) (db.Order, []StockWarning, error) {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return db.Order{}, nil, fmt.Errorf("BEGIN TRANSACTION: %w", err)
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
	companyID := pgtype.UUID{Bytes: input.CompanyID, Valid: true}

	var order db.Order
	warnings := make([]StockWarning, 0)

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
				CompanyID:    companyID,
			},
		)
		if err != nil {
			return db.Order{}, nil, fmt.Errorf("CREATE STAFF ORDER: %w", err)
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
		adminOrder, err := queries.CreateOrderForAdmin(
			ctx,
			db.CreateOrderForAdminParams{
				CustomerID:   customerID,
				Source:       input.Source,
				Status:       db.OrderStatusConfirmed,
				Address:      input.Address,
				CodAmount:    input.CodAmount,
				IsStoreVisit: input.IsStoreVisit,
				CreatedBy:    createdBy,
				CompanyID:    companyID,
			},
		)
		if err == nil {
			order = db.Order{ID: adminOrder.ID, CustomerID: adminOrder.CustomerID, Source: adminOrder.Source, Status: adminOrder.Status, CourierID: adminOrder.CourierID, LocationID: adminOrder.LocationID, Address: adminOrder.Address, CodAmount: adminOrder.CodAmount, IsStoreVisit: adminOrder.IsStoreVisit, CreatedBy: adminOrder.CreatedBy, CreatedAt: adminOrder.CreatedAt, UpdatedAt: adminOrder.UpdatedAt, IsLegacy: adminOrder.IsLegacy}
		}
		if err != nil {
			return db.Order{}, nil, fmt.Errorf("CREATE ADMIN ORDER: %w", err)
		}
	}

	if _, err := queries.CreateStatusHistory(
		ctx,
		db.CreateStatusHistoryParams{
			OrderID:    order.ID,
			FromStatus: db.NullOrderStatus{Valid: false},
			ToStatus:   db.OrderStatusConfirmed,
			ChangedBy:  createdBy,
			CompanyID:  companyID,
		},
	); err != nil {
		return db.Order{}, nil, fmt.Errorf("CREATE STATUS HISTORY: %w", err)
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
				AvailableQty: item.Quantity, CompanyID: companyID,
			},
		)
		if err != nil {
			if err == pgx.ErrNoRows {
				productDetails, fetchErr := queries.GetProductByID(ctx, db.GetProductByIDParams{ID: productID, CompanyID: companyID})
				if fetchErr != nil {
					return db.Order{}, nil, fmt.Errorf("GET PRODUCT: %w", fetchErr)
				}
				warnings = append(warnings, StockWarning{
					ProductID: item.ProductID, ProductName: productDetails.Name,
					RequestedQty: item.Quantity, AvailableQty: productDetails.AvailableQty,
				})
				productPrice := productDetails.Price
				_, err = queries.CreateOrderItem(
					ctx,
					db.CreateOrderItemParams{
						OrderID: order.ID, ProductID: productID,
						Quantity: item.Quantity, Price: productPrice,
						CompanyID: companyID,
					},
				)
				if err != nil {
					return db.Order{}, nil, fmt.Errorf("CREATE ORDER ITEM: %w", err)
				}
				continue
			} else {
				return db.Order{}, nil, fmt.Errorf("DECREASE STOCK: %w", err)
			}
		}

		_, err = queries.CreateOrderItem(
			ctx,
			db.CreateOrderItemParams{
				OrderID:   order.ID,
				ProductID: product.ID,
				Quantity:  item.Quantity,
				Price:     product.Price,
				CompanyID: companyID,
			},
		)
		if err != nil {
			return db.Order{}, nil, fmt.Errorf("CREATE ORDER ITEM: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return db.Order{}, nil, fmt.Errorf("COMMIT TRANSACTION: %w", err)
	}

	return order, warnings, nil
}
