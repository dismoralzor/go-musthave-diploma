package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestGetBalance_Success(t *testing.T) {
	s, mock := newMockStorage(t)

	mock.ExpectQuery("SELECT balance, withdrawn FROM users").
		WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "withdrawn"}).AddRow(700.0, 0.0))

	b, err := s.GetBalance(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetBalance() unexpected error: %v", err)
	}
	if b.Current != 700 || b.Withdrawn != 0 {
		t.Errorf("GetBalance() = %+v, unexpected", b)
	}
}

func TestGetBalance_Error(t *testing.T) {
	s, mock := newMockStorage(t)

	mock.ExpectQuery("SELECT balance, withdrawn FROM users").
		WithArgs(int64(1)).
		WillReturnError(errBoom)

	if _, err := s.GetBalance(context.Background(), 1); err == nil {
		t.Fatal("GetBalance() expected error, got nil")
	}
}

func TestWithdraw_Success(t *testing.T) {
	s, mock := newMockStorage(t)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE users").
		WithArgs(700.0, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO withdrawals").
		WithArgs(int64(1), "12345678903", 700.0).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	err := s.Withdraw(context.Background(), 1, "12345678903", 700)
	if err != nil {
		t.Fatalf("Withdraw() unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWithdraw_InsufficientFunds(t *testing.T) {
	s, mock := newMockStorage(t)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE users").
		WithArgs(700.0, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()

	err := s.Withdraw(context.Background(), 1, "12345678903", 700)
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Errorf("Withdraw() error = %v, want ErrInsufficientFunds", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestWithdraw_InsertError(t *testing.T) {
	s, mock := newMockStorage(t)

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE users").
		WithArgs(700.0, int64(1)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO withdrawals").
		WillReturnError(errBoom)
	mock.ExpectRollback()

	err := s.Withdraw(context.Background(), 1, "12345678903", 700)
	if err == nil {
		t.Fatal("Withdraw() expected error, got nil")
	}
}

func TestGetWithdrawals_Success(t *testing.T) {
	s, mock := newMockStorage(t)

	rows := sqlmock.NewRows([]string{"order_number", "sum", "processed_at"}).
		AddRow("12345678903", 700.0, time.Now())

	mock.ExpectQuery("SELECT order_number, sum, processed_at FROM withdrawals").
		WithArgs(int64(1)).
		WillReturnRows(rows)

	withdrawals, err := s.GetWithdrawals(context.Background(), 1)
	if err != nil {
		t.Fatalf("GetWithdrawals() unexpected error: %v", err)
	}
	if len(withdrawals) != 1 || withdrawals[0].Order != "12345678903" || withdrawals[0].Sum != 700 {
		t.Errorf("GetWithdrawals() = %+v, unexpected", withdrawals)
	}
}

func TestGetWithdrawals_Error(t *testing.T) {
	s, mock := newMockStorage(t)

	mock.ExpectQuery("SELECT order_number, sum, processed_at FROM withdrawals").
		WithArgs(int64(1)).
		WillReturnError(errBoom)

	if _, err := s.GetWithdrawals(context.Background(), 1); err == nil {
		t.Fatal("GetWithdrawals() expected error, got nil")
	}
}
