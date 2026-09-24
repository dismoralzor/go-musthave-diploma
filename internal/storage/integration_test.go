package storage

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/dismoralzor/go-musthave-diploma/internal/models"
)

// newIntegrationStorage открывает Storage на реальном Postgres из
// TEST_DATABASE_URI, применяет миграции и очищает таблицы перед тестом.
// Если переменная не задана, тест тихо скипается — так CI без Postgres не
// падает, а разработчик может прогнать интеграционные тесты локально.
func newIntegrationStorage(t *testing.T) *Storage {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URI")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URI is not set, skipping integration test")
	}

	ctx := context.Background()

	db, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("Open() unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate() unexpected error: %v", err)
	}

	if _, err := db.ExecContext(ctx, "TRUNCATE TABLE withdrawals, orders, users RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate tables: %v", err)
	}

	return New(db)
}

func TestIntegration_UserAndOrderLifecycle(t *testing.T) {
	s := newIntegrationStorage(t)
	ctx := context.Background()

	userID, err := s.CreateUser(ctx, "user1", "hash")
	if err != nil {
		t.Fatalf("CreateUser() unexpected error: %v", err)
	}

	if _, err := s.CreateUser(ctx, "user1", "hash"); err == nil {
		t.Error("CreateUser() expected ErrLoginTaken on duplicate login, got nil")
	}

	u, err := s.GetUserByLogin(ctx, "user1")
	if err != nil {
		t.Fatalf("GetUserByLogin() unexpected error: %v", err)
	}
	if u.ID != userID {
		t.Errorf("GetUserByLogin() ID = %d, want %d", u.ID, userID)
	}

	if err := s.CreateOrder(ctx, "12345678903", userID); err != nil {
		t.Fatalf("CreateOrder() unexpected error: %v", err)
	}

	if err := s.CreateOrder(ctx, "12345678903", userID); !errors.Is(err, ErrOrderOwnedByUser) {
		t.Errorf("CreateOrder() re-upload by same user = %v, want ErrOrderOwnedByUser", err)
	}

	otherUserID, err := s.CreateUser(ctx, "user2", "hash")
	if err != nil {
		t.Fatalf("CreateUser() unexpected error: %v", err)
	}
	if err := s.CreateOrder(ctx, "12345678903", otherUserID); !errors.Is(err, ErrOrderOwnedByOther) {
		t.Errorf("CreateOrder() upload by another user = %v, want ErrOrderOwnedByOther", err)
	}

	claimed, err := s.ClaimOrdersForProcessing(ctx, 10)
	if err != nil {
		t.Fatalf("ClaimOrdersForProcessing() unexpected error: %v", err)
	}
	if len(claimed) != 1 || claimed[0].Number != "12345678903" {
		t.Fatalf("ClaimOrdersForProcessing() = %+v, want one claimed order", claimed)
	}

	// Заказ уже помечен PROCESSING в БД — повторный клейм не должен его
	// вернуть, пока не истечёт staleProcessingTimeout.
	claimedAgain, err := s.ClaimOrdersForProcessing(ctx, 10)
	if err != nil {
		t.Fatalf("ClaimOrdersForProcessing() unexpected error: %v", err)
	}
	if len(claimedAgain) != 0 {
		t.Errorf("ClaimOrdersForProcessing() re-claimed = %+v, want none", claimedAgain)
	}

	accrual := 700.0
	if err := s.UpdateOrderStatus(ctx, "12345678903", models.OrderStatusProcessed, &accrual, userID); err != nil {
		t.Fatalf("UpdateOrderStatus() unexpected error: %v", err)
	}

	orders, err := s.GetOrdersByUser(ctx, userID)
	if err != nil {
		t.Fatalf("GetOrdersByUser() unexpected error: %v", err)
	}
	if len(orders) != 1 || orders[0].Status != models.OrderStatusProcessed {
		t.Fatalf("GetOrdersByUser() = %+v, want one PROCESSED order", orders)
	}

	balance, err := s.GetBalance(ctx, userID)
	if err != nil {
		t.Fatalf("GetBalance() unexpected error: %v", err)
	}
	if balance.Current != 700 {
		t.Errorf("GetBalance().Current = %v, want 700", balance.Current)
	}
}

func TestIntegration_WithdrawAtomicity(t *testing.T) {
	s := newIntegrationStorage(t)
	ctx := context.Background()

	userID, err := s.CreateUser(ctx, "user1", "hash")
	if err != nil {
		t.Fatalf("CreateUser() unexpected error: %v", err)
	}
	if err := s.CreateOrder(ctx, "12345678903", userID); err != nil {
		t.Fatalf("CreateOrder() unexpected error: %v", err)
	}
	accrual := 500.0
	if err := s.UpdateOrderStatus(ctx, "12345678903", models.OrderStatusProcessed, &accrual, userID); err != nil {
		t.Fatalf("UpdateOrderStatus() unexpected error: %v", err)
	}

	if err := s.Withdraw(ctx, userID, "12345678903", 600); !errors.Is(err, ErrInsufficientFunds) {
		t.Errorf("Withdraw() over balance = %v, want ErrInsufficientFunds", err)
	}

	if err := s.Withdraw(ctx, userID, "12345678903", 200); err != nil {
		t.Fatalf("Withdraw() unexpected error: %v", err)
	}

	balance, err := s.GetBalance(ctx, userID)
	if err != nil {
		t.Fatalf("GetBalance() unexpected error: %v", err)
	}
	if balance.Current != 300 || balance.Withdrawn != 200 {
		t.Errorf("GetBalance() = %+v, want current=300 withdrawn=200", balance)
	}

	withdrawals, err := s.GetWithdrawals(ctx, userID)
	if err != nil {
		t.Fatalf("GetWithdrawals() unexpected error: %v", err)
	}
	if len(withdrawals) != 1 || withdrawals[0].Sum != 200 {
		t.Errorf("GetWithdrawals() = %+v, want one withdrawal of 200", withdrawals)
	}
}
