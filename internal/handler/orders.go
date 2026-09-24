package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/dismoralzor/go-musthave-diploma/internal/auth"
	"github.com/dismoralzor/go-musthave-diploma/internal/luhn"
	"github.com/dismoralzor/go-musthave-diploma/internal/models"
	"github.com/dismoralzor/go-musthave-diploma/internal/storage"
)

// isDigits сообщает, состоит ли s только из цифр и непусто. Используется
// вместо strconv.Atoi, чтобы не терять валидные (по алгоритму Луна) длинные
// номера заказов, переполняющие int.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// UploadOrder обрабатывает загрузку номера заказа пользователем: номер
// передаётся в теле запроса как text/plain.
func (h *Handler) UploadOrder(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	number := strings.TrimSpace(string(body))
	if number == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !isDigits(number) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !luhn.Valid(number) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		return
	}

	err = h.orders.CreateOrder(r.Context(), number, userID)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusAccepted)
	case errors.Is(err, storage.ErrOrderOwnedByUser):
		w.WriteHeader(http.StatusOK)
	case errors.Is(err, storage.ErrOrderOwnedByOther):
		w.WriteHeader(http.StatusConflict)
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// ListOrders возвращает список заказов, загруженных пользователем, от
// самых новых к самым старым.
func (h *Handler) ListOrders(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	orders, err := h.orders.GetOrdersByUser(r.Context(), userID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	resp := make([]models.OrderResponse, len(orders))
	for i, o := range orders {
		resp[i] = models.OrderResponse{
			Number:     o.Number,
			Status:     o.Status,
			Accrual:    o.Accrual,
			UploadedAt: o.UploadedAt,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}
