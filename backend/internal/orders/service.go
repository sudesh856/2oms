package orders

import (
	"context"
	"fmt"

	db "oms-backend/internal/db/generated"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	Pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{
		Pool: pool,
	}
}

func (s *Service) UpdateStatus(
	ctx context.Context,
	orderID pgtype.UUID,
	fromStatus db.OrderStatus,
	toStatus db.OrderStatus,
	userID pgtype.UUID,
	companyID pgtype.UUID,
) error {
	if err := ValidateTransition(string(fromStatus), string(toStatus)); err != nil {
		return err
	}

	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	queries := db.New(s.Pool).WithTx(tx)

	order, err := queries.UpdateOrderStatus(
		ctx,
		db.UpdateOrderStatusParams{
			ID:        orderID,
			Status:    toStatus,
			Status_2:  fromStatus,
			CompanyID: companyID,
		},
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("order not found")
		}

		return fmt.Errorf("update order status: %w", err)
	}

	_, err = queries.CreateStatusHistory(
		ctx,
		db.CreateStatusHistoryParams{
			OrderID: order.ID,
			FromStatus: db.NullOrderStatus{
				OrderStatus: fromStatus,
				Valid:       true,
			},
			ToStatus:  toStatus,
			ChangedBy: userID,
			CompanyID: companyID,
		},
	)
	if err != nil {
		return fmt.Errorf("create status history: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

func CanTransition(from, to string) bool {
	switch from {
	case "confirmed":
		return to == "pickup_complete" ||
			to == "follow_up" ||
			to == "hold" ||
			to == "cancelled"

	case "pickup_complete":
		return to == "dispatched" ||
			to == "follow_up" ||
			to == "hold" ||
			to == "redirected" ||
			to == "cancelled"

	case "dispatched":
		return to == "arrived" ||
			to == "follow_up" ||
			to == "hold" ||
			to == "redirected" ||
			to == "cancelled" ||
			to == "returned"

	case "arrived":
		return to == "delivered" ||
			to == "follow_up" ||
			to == "hold" ||
			to == "redirected" ||
			to == "cancelled" ||
			to == "returned"

	case "follow_up":
		return to == "confirmed" ||
			to == "pickup_complete" ||
			to == "hold" ||
			to == "cancelled"

	case "hold":
		return to == "confirmed" ||
			to == "pickup_complete" ||
			to == "cancelled"

	case "redirected":
		return to == "dispatched" ||
			to == "arrived" ||
			to == "cancelled"

	case "delivered", "cancelled", "returned":
		return false

	default:
		return false
	}
}

func ValidateTransition(from, to string) error {
	if !CanTransition(from, to) {
		return fmt.Errorf(
			"invalid order status transition: %s -> %s",
			from,
			to,
		)
	}

	return nil
}
