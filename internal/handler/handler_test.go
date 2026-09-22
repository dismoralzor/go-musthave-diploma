package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dismoralzor/go-musthave-diploma/internal/auth"
	"github.com/dismoralzor/go-musthave-diploma/internal/models"
)

// fakeUserStore — тестовая реализация UserStore с настраиваемым поведением.
type fakeUserStore struct {
	createUser     func(ctx context.Context, login, passwordHash string) (int64, error)
	getUserByLogin func(ctx context.Context, login string) (models.User, error)
}

func (f *fakeUserStore) CreateUser(ctx context.Context, login, passwordHash string) (int64, error) {
	return f.createUser(ctx, login, passwordHash)
}

func (f *fakeUserStore) GetUserByLogin(ctx context.Context, login string) (models.User, error) {
	return f.getUserByLogin(ctx, login)
}

// fakeOrderStore — тестовая реализация OrderStore с настраиваемым поведением.
type fakeOrderStore struct {
	createOrder     func(ctx context.Context, number string, userID int64) error
	getOrdersByUser func(ctx context.Context, userID int64) ([]models.Order, error)
}

func (f *fakeOrderStore) CreateOrder(ctx context.Context, number string, userID int64) error {
	return f.createOrder(ctx, number, userID)
}

func (f *fakeOrderStore) GetOrdersByUser(ctx context.Context, userID int64) ([]models.Order, error) {
	return f.getOrdersByUser(ctx, userID)
}

// fakeBalanceStore — тестовая реализация BalanceStore с настраиваемым
// поведением.
type fakeBalanceStore struct {
	getBalance     func(ctx context.Context, userID int64) (models.Balance, error)
	withdraw       func(ctx context.Context, userID int64, order string, sum float64) error
	getWithdrawals func(ctx context.Context, userID int64) ([]models.Withdrawal, error)
}

func (f *fakeBalanceStore) GetBalance(ctx context.Context, userID int64) (models.Balance, error) {
	return f.getBalance(ctx, userID)
}

func (f *fakeBalanceStore) Withdraw(ctx context.Context, userID int64, order string, sum float64) error {
	return f.withdraw(ctx, userID, order, sum)
}

func (f *fakeBalanceStore) GetWithdrawals(ctx context.Context, userID int64) ([]models.Withdrawal, error) {
	return f.getWithdrawals(ctx, userID)
}

// newTestRouter собирает роутер с переданными зависимостями; nil-поля
// заменяются реализациями, которые паникуют при обращении — так тест сразу
// падает, если случайно дёрнул незаданную зависимость.
func newTestRouter(t *testing.T, users UserStore, orders OrderStore, balance BalanceStore) http.Handler {
	t.Helper()

	if users == nil {
		users = &fakeUserStore{}
	}
	if orders == nil {
		orders = &fakeOrderStore{}
	}
	if balance == nil {
		balance = &fakeBalanceStore{}
	}

	return NewRouter(New(users, orders, balance))
}

// authRequest создаёт запрос с валидным JWT для userID, переданным через
// заголовок Authorization, как это делает настоящий клиент.
func authRequest(t *testing.T, method, target string, userID int64, body io.Reader) *http.Request {
	t.Helper()

	token, err := auth.GenerateToken(userID)
	if err != nil {
		t.Fatalf("auth.GenerateToken() unexpected error: %v", err)
	}

	req := httptest.NewRequest(method, target, body)
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}
