package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dismoralzor/go-musthave-diploma/internal/models"
	"github.com/dismoralzor/go-musthave-diploma/internal/storage"
)

func TestUploadOrder(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		createOrder func(ctx context.Context, number string, userID int64) error
		wantStatus  int
	}{
		{
			name:        "accepted",
			body:        "12345678903",
			createOrder: func(ctx context.Context, number string, userID int64) error { return nil },
			wantStatus:  http.StatusAccepted,
		},
		{
			name:        "already uploaded by this user",
			body:        "12345678903",
			createOrder: func(ctx context.Context, number string, userID int64) error { return storage.ErrOrderOwnedByUser },
			wantStatus:  http.StatusOK,
		},
		{
			name:        "already uploaded by another user",
			body:        "12345678903",
			createOrder: func(ctx context.Context, number string, userID int64) error { return storage.ErrOrderOwnedByOther },
			wantStatus:  http.StatusConflict,
		},
		{
			name:        "internal error",
			body:        "12345678903",
			createOrder: func(ctx context.Context, number string, userID int64) error { return errBoom },
			wantStatus:  http.StatusInternalServerError,
		},
		{
			name:        "empty body",
			body:        "",
			createOrder: func(ctx context.Context, number string, userID int64) error { return nil },
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "non-numeric body",
			body:        "abc",
			createOrder: func(ctx context.Context, number string, userID int64) error { return nil },
			wantStatus:  http.StatusBadRequest,
		},
		{
			name:        "fails luhn check",
			body:        "1234567890",
			createOrder: func(ctx context.Context, number string, userID int64) error { return nil },
			wantStatus:  http.StatusUnprocessableEntity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orders := &fakeOrderStore{createOrder: tt.createOrder}
			router := newTestRouter(t, nil, orders, nil)

			req := authRequest(t, http.MethodPost, "/api/user/orders", 1, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "text/plain")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestListOrders(t *testing.T) {
	accrual := 700.0
	uploadedAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name            string
		getOrdersByUser func(ctx context.Context, userID int64) ([]models.Order, error)
		wantStatus      int
		wantBody        string
	}{
		{
			name: "empty list",
			getOrdersByUser: func(ctx context.Context, userID int64) ([]models.Order, error) {
				return nil, nil
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "non-empty list",
			getOrdersByUser: func(ctx context.Context, userID int64) ([]models.Order, error) {
				return []models.Order{{
					Number:     "12345678903",
					Status:     models.OrderStatusProcessed,
					Accrual:    &accrual,
					UploadedAt: uploadedAt,
					UserID:     1,
				}}, nil
			},
			wantStatus: http.StatusOK,
			wantBody:   `"number":"12345678903"`,
		},
		{
			name: "internal error",
			getOrdersByUser: func(ctx context.Context, userID int64) ([]models.Order, error) {
				return nil, errBoom
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orders := &fakeOrderStore{getOrdersByUser: tt.getOrdersByUser}
			router := newTestRouter(t, nil, orders, nil)

			req := authRequest(t, http.MethodGet, "/api/user/orders", 1, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && !strings.Contains(w.Body.String(), tt.wantBody) {
				t.Errorf("body = %q, want it to contain %q", w.Body.String(), tt.wantBody)
			}
		})
	}
}
