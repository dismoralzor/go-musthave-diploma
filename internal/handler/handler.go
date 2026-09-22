// Package handler содержит HTTP-обработчики сервиса gophermart и сборку
// маршрутизатора.
package handler

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

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

// Handler объединяет HTTP-обработчики сервиса и их зависимости.
type Handler struct {
	Users   UserStore
	Orders  OrderStore
	Balance BalanceStore
}

// New создаёт новый Handler с хранилищем пользователей users, хранилищем
// заказов orders и хранилищем баланса balance.
func New(users UserStore, orders OrderStore, balance BalanceStore) *Handler {
	return &Handler{Users: users, Orders: orders, Balance: balance}
}

// NewRouter собирает http.Handler со всеми маршрутами сервиса и
// middleware логирования запросов. Регистрация и аутентификация доступны
// без токена; остальные ручки защищены auth.Middleware.
func NewRouter(h *Handler) http.Handler {
	r := chi.NewRouter()
	r.Use(logger.RequestLogger)

	r.Route("/api/user", func(r chi.Router) {
		r.Post("/register", h.Register)
		r.Post("/login", h.Login)

		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware)

			r.Post("/orders", h.UploadOrder)
			r.Get("/orders", h.ListOrders)
			r.Get("/balance", h.GetBalance)
			r.Post("/balance/withdraw", h.WithdrawBalance)
			r.Get("/withdrawals", h.ListWithdrawals)
		})
	})

	return r
}
