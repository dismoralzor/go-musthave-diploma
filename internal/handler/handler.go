// Package handler содержит HTTP-обработчики сервиса gophermart и сборку
// маршрутизатора.
package handler

import (
	"context"
	"net/http"

	"github.com/dismoralzor/go-musthave-diploma/internal/auth"
	"github.com/dismoralzor/go-musthave-diploma/internal/logger"
	"github.com/dismoralzor/go-musthave-diploma/internal/models"
)

// UserStore описывает доступ к данным пользователей, необходимый
// обработчикам аутентификации. Выделен в интерфейс, чтобы Handler не
// зависел от конкретной реализации хранилища.
type UserStore interface {
	CreateUser(ctx context.Context, login, passwordHash string) (int64, error)
	GetUserByLogin(ctx context.Context, login string) (models.User, error)
}

// OrderStore описывает доступ к данным заказов, необходимый обработчикам
// загрузки и получения заказов.
type OrderStore interface {
	CreateOrder(ctx context.Context, number string, userID int64) error
	GetOrdersByUser(ctx context.Context, userID int64) ([]models.Order, error)
}

// BalanceStore описывает доступ к данным баланса и списаний, необходимый
// обработчикам баланса.
type BalanceStore interface {
	GetBalance(ctx context.Context, userID int64) (models.Balance, error)
	Withdraw(ctx context.Context, userID int64, order string, sum float64) error
	GetWithdrawals(ctx context.Context, userID int64) ([]models.Withdrawal, error)
}

// Handler объединяет HTTP-обработчики сервиса и их зависимости. Поля
// приватны — зависимости задаются только через New, а извне Handler виден
// лишь набором методов-обработчиков.
type Handler struct {
	users   UserStore
	orders  OrderStore
	balance BalanceStore
}

// New создаёт новый Handler с хранилищем пользователей users, хранилищем
// заказов orders и хранилищем баланса balance.
func New(users UserStore, orders OrderStore, balance BalanceStore) *Handler {
	return &Handler{users: users, orders: orders, balance: balance}
}

// NewRouter собирает http.Handler со всеми маршрутами сервиса и
// middleware логирования запросов. Регистрация и аутентификация доступны
// без токена; остальные ручки защищены auth.Middleware.
func NewRouter(h *Handler) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/user/register", h.Register)
	mux.HandleFunc("POST /api/user/login", h.Login)

	mux.Handle("POST /api/user/orders", auth.Middleware(http.HandlerFunc(h.UploadOrder)))
	mux.Handle("GET /api/user/orders", auth.Middleware(http.HandlerFunc(h.ListOrders)))
	mux.Handle("GET /api/user/balance", auth.Middleware(http.HandlerFunc(h.GetBalance)))
	mux.Handle("POST /api/user/balance/withdraw", auth.Middleware(http.HandlerFunc(h.WithdrawBalance)))
	mux.Handle("GET /api/user/withdrawals", auth.Middleware(http.HandlerFunc(h.ListWithdrawals)))

	return logger.RequestLogger(mux)
}
