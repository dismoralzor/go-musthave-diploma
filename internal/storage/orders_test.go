package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/dismoralzor/go-musthave-diploma/internal/models"
)

func TestCreateOrder_Success(t *testing.T) {
	s, mock := newMockStorage(t)

	mock.ExpectExec("INSERT INTO orders").
		WithArgs("12345678903", int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	if err := s.CreateOrder(context.Background(), "12345678903", 1); err != nil {
		t.Fatalf("CreateOrder() unexpected error: %v", err)
	}
}

func TestCreateOrder_OwnedByUser(t *testing.T) {
	s, mock := newMockStorage(t)

	mock.ExpectExec("INSERT INTO orders").
		WithArgs("12345678903", int64(1)).
		WillReturnError(&pgconn.PgError{Code: pgerrcode.UniqueViolation})
	mock.ExpectQuery("SELECT user_id FROM orders").
		WithArgs("12345678903").
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(int64(1)))

	err := s.CreateOrder(context.Background(), "12345678903", 1)
	if !errors.Is(err, ErrOrderOwnedByUser) {
		t.Errorf("CreateOrder() error = %v, want ErrOrderOwnedByUser", err)
	}
}

func TestCreateOrder_OwnedByOther(t *testing.T) {
	s, mock := newMockStorage(t)

	mock.ExpectExec("INSERT INTO orders").
		WithArgs("12345678903", int64(1)).
		WillReturnError(&pgconn.PgError{Code: pgerrcode.UniqueViolation})
	mock.ExpectQuery("SELECT user_id FROM orders").
		WithArgs("12345678903").
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(int64(2)))

	err := s.CreateOrder(context.Background(), "12345678903", 1)
	if !errors.Is(err, ErrOrderOwnedByOther) {
		t.Errorf("CreateOrder() error = %v, want ErrOrderOwnedByOther", err)
	}
}

func TestCreateOrder_OtherError(t *testing.T) {
	s, mock := newMockStorage(t)

	mock.ExpectExec("INSERT INTO orders").
		WithArgs("12345678903", int64(1)).
		WillReturnError(errBoom)

	err := s.CreateOrder(context.Background(), "12345678903", 1)
	if err == nil {
		t.Fatal("CreateOrder() expected error, got nil")
	}
}

func TestGetOrdersByUser_Success(t *testing.T) {
	s, mock := newMockStorage(t)

	accrual := 700.0
	rows := sqlmock.NewRows([]string{"number", "user_id", "status", "accrual", "uploaded_at"}).
		AddRow("12345678903", int64(1), models.OrderStatusProcessed, &accrual, time.Now())

	mock.ExpectQuery("SELECT number, user_id, status, accrual, uploaded_at FROM orders").
		WithArgs(int64(1)).
		WillReturnRows(rows)

	orders, err := s.GetOrdersByUser(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetOrdersByUser() unexpected error: %v", err)
	}
	if len(orders) != 1 || orders[0].Number != "12345678903" || orders[0].Status != models.OrderStatusProcessed {
		t.Errorf("GetOrdersByUser() = %+v, unexpected", orders)
	}
}

func TestGetOrdersByUser_QueryError(t *testing.T) {
	s, mock := newMockStorage(t)

	mock.ExpectQuery("SELECT number, user_id, status, accrual, uploaded_at FROM orders").
		WithArgs(int64(1)).
		WillReturnError(errBoom)

	if _, err := s.GetOrdersByUser(context.Background(), 1); err == nil {
		t.Fatal("GetOrdersByUser() expected error, got nil")
	}
}

func TestClaimOrdersForProcessing(t *testing.T) {
	s, mock := newMockStorage(t)

	rows := sqlmock.NewRows([]string{"number", "user_id", "status"}).
		AddRow("12345678903", int64(1), models.OrderStatusProcessing)

	mock.ExpectQuery("UPDATE orders SET status = \\$1, updated_at = now\\(\\)").
		WithArgs(models.OrderStatusProcessing, models.OrderStatusNew, 10).
		WillReturnRows(rows)

	orders, err := s.ClaimOrdersForProcessing(context.Background(), 10)
	if err != nil {
		t.Fatalf("ClaimOrdersForProcessing() unexpected error: %v", err)
	}
	if len(orders) != 1 || orders[0].Number != "12345678903" {
		t.Errorf("ClaimOrdersForProcessing() = %+v, unexpected", orders)
	}
}

func TestClaimOrdersForProcessing_QueryError(t *testing.T) {
	s, mock := newMockStorage(t)

	mock.ExpectQuery("UPDATE orders SET status = \\$1, updated_at = now\\(\\)").
		WithArgs(models.OrderStatusProcessing, models.OrderStatusNew, 10).
		WillReturnError(errBoom)

	if _, err := s.ClaimOrdersForProcessing(context.Background(), 10); err == nil {
		t.Fatal("ClaimOrdersForProcessing() expected error, got nil")
	}
}

func TestUpdateOrderStatus_ProcessedWithAccrual(t *testing.T) {
	s, mock := newMockStorage(t)

	accrual := 700.0

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE orders SET status = \\$1, accrual = \\$2, updated_at = now\\(\\) WHERE number = \\$3").
		WithArgs(models.OrderStatusProcessed, &accrual, "12345678903").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE users SET balance = balance \\+ \\$1 WHERE id = \\$2").
		WithArgs(accrual, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := s.UpdateOrderStatus(context.Background(), "12345678903", models.OrderStatusProcessed, &accrual, 1)
	if err != nil {
		t.Fatalf("UpdateOrderStatus() unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestUpdateOrderStatus_ProcessingNoBalanceUpdate(t *testing.T) {
	s, mock := newMockStorage(t)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE orders SET status = \\$1, accrual = \\$2, updated_at = now\\(\\) WHERE number = \\$3").
		WithArgs(models.OrderStatusProcessing, nil, "12345678903").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := s.UpdateOrderStatus(context.Background(), "12345678903", models.OrderStatusProcessing, nil, 1)
	if err != nil {
		t.Fatalf("UpdateOrderStatus() unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestUpdateOrderStatus_UpdateError(t *testing.T) {
	s, mock := newMockStorage(t)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE orders SET status = \\$1, accrual = \\$2, updated_at = now\\(\\) WHERE number = \\$3").
		WillReturnError(errBoom)
	mock.ExpectRollback()

	err := s.UpdateOrderStatus(context.Background(), "12345678903", models.OrderStatusInvalid, nil, 1)
	if err == nil {
		t.Fatal("UpdateOrderStatus() expected error, got nil")
	}
}
