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

func TestGetBalance(t *testing.T) {
	tests := []struct {
		name       string
		getBalance func(ctx context.Context, userID int64) (models.Balance, error)
		wantStatus int
		wantBody   string
	}{
		{
			name: "success",
			getBalance: func(ctx context.Context, userID int64) (models.Balance, error) {
				return models.Balance{Current: 700, Withdrawn: 0}, nil
			},
			wantStatus: http.StatusOK,
			wantBody:   `"current":700`,
		},
		{
			name: "internal error",
			getBalance: func(ctx context.Context, userID int64) (models.Balance, error) {
				return models.Balance{}, errBoom
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			balance := &fakeBalanceStore{getBalance: tt.getBalance}
			router := newTestRouter(t, nil, nil, balance)

			req := authRequest(t, http.MethodGet, "/api/user/balance", 1, nil)
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

func TestWithdrawBalance(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		withdraw   func(ctx context.Context, userID int64, order string, sum float64) error
		wantStatus int
	}{
		{
			name:       "success",
			body:       `{"order":"12345678903","sum":700}`,
			withdraw:   func(ctx context.Context, userID int64, order string, sum float64) error { return nil },
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid json",
			body:       `not json`,
			withdraw:   func(ctx context.Context, userID int64, order string, sum float64) error { return nil },
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "non-positive sum",
			body:       `{"order":"12345678903","sum":0}`,
			withdraw:   func(ctx context.Context, userID int64, order string, sum float64) error { return nil },
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "fails luhn check",
			body:       `{"order":"1234567890","sum":700}`,
			withdraw:   func(ctx context.Context, userID int64, order string, sum float64) error { return nil },
			wantStatus: http.StatusUnprocessableEntity,
		},
		{
			name: "insufficient funds",
			body: `{"order":"12345678903","sum":700}`,
			withdraw: func(ctx context.Context, userID int64, order string, sum float64) error {
				return storage.ErrInsufficientFunds
			},
			wantStatus: http.StatusPaymentRequired,
		},
		{
			name:       "internal error",
			body:       `{"order":"12345678903","sum":700}`,
			withdraw:   func(ctx context.Context, userID int64, order string, sum float64) error { return errBoom },
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			balance := &fakeBalanceStore{withdraw: tt.withdraw}
			router := newTestRouter(t, nil, nil, balance)

			req := authRequest(t, http.MethodPost, "/api/user/balance/withdraw", 1, strings.NewReader(tt.body))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestListWithdrawals(t *testing.T) {
	processedAt := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name           string
		getWithdrawals func(ctx context.Context, userID int64) ([]models.Withdrawal, error)
		wantStatus     int
		wantBody       string
	}{
		{
			name: "empty list",
			getWithdrawals: func(ctx context.Context, userID int64) ([]models.Withdrawal, error) {
				return nil, nil
			},
			wantStatus: http.StatusNoContent,
		},
		{
			name: "non-empty list",
			getWithdrawals: func(ctx context.Context, userID int64) ([]models.Withdrawal, error) {
				return []models.Withdrawal{{Order: "12345678903", Sum: 700, ProcessedAt: processedAt}}, nil
			},
			wantStatus: http.StatusOK,
			wantBody:   `"order":"12345678903"`,
		},
		{
			name: "internal error",
			getWithdrawals: func(ctx context.Context, userID int64) ([]models.Withdrawal, error) {
				return nil, errBoom
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			balance := &fakeBalanceStore{getWithdrawals: tt.getWithdrawals}
			router := newTestRouter(t, nil, nil, balance)

			req := authRequest(t, http.MethodGet, "/api/user/withdrawals", 1, nil)
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
