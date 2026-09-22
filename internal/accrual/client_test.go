package accrual

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dismoralzor/go-musthave-diploma/internal/models"
)

func floatPtr(v float64) *float64 { return &v }

func TestGetOrder_OK_StatusMapping(t *testing.T) {
	tests := []struct {
		name           string
		accrualStatus  string
		wantStatus     string
		wantAccrualVal *float64
	}{
		{name: "registered maps to processing", accrualStatus: "REGISTERED", wantStatus: models.OrderStatusProcessing},
		{name: "processing stays processing", accrualStatus: "PROCESSING", wantStatus: models.OrderStatusProcessing},
		{name: "invalid maps to invalid", accrualStatus: "INVALID", wantStatus: models.OrderStatusInvalid},
		{name: "processed maps to processed", accrualStatus: "PROCESSED", wantStatus: models.OrderStatusProcessed, wantAccrualVal: floatPtr(700)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				body := `{"order":"123","status":"` + tt.accrualStatus + `"`
				if tt.wantAccrualVal != nil {
					body += `,"accrual":700`
				}
				body += `}`
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()

			c := New(srv.URL)
			result, err := c.GetOrder(context.Background(), "123")
			if err != nil {
				t.Fatalf("GetOrder() unexpected error: %v", err)
			}
			if result.Status != tt.wantStatus {
				t.Errorf("Status = %q, want %q", result.Status, tt.wantStatus)
			}
			if tt.wantAccrualVal != nil {
				if result.Accrual == nil || *result.Accrual != *tt.wantAccrualVal {
					t.Errorf("Accrual = %v, want %v", result.Accrual, *tt.wantAccrualVal)
				}
			}
		})
	}
}

func TestGetOrder_NotRegistered(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.GetOrder(context.Background(), "123")
	if !errors.Is(err, ErrNotRegistered) {
		t.Errorf("GetOrder() error = %v, want ErrNotRegistered", err)
	}
}

func TestGetOrder_TooManyRequests(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "5")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.GetOrder(context.Background(), "123")

	var tooMany *TooManyRequestsError
	if !errors.As(err, &tooMany) {
		t.Fatalf("GetOrder() error = %v, want *TooManyRequestsError", err)
	}
	if tooMany.RetryAfter != 5*time.Second {
		t.Errorf("RetryAfter = %v, want 5s", tooMany.RetryAfter)
	}
	if tooMany.Error() == "" {
		t.Error("Error() returned empty string")
	}
}

func TestGetOrder_TooManyRequests_MissingRetryAfter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.GetOrder(context.Background(), "123")

	var tooMany *TooManyRequestsError
	if !errors.As(err, &tooMany) {
		t.Fatalf("GetOrder() error = %v, want *TooManyRequestsError", err)
	}
	if tooMany.RetryAfter != time.Second {
		t.Errorf("RetryAfter = %v, want default 1s", tooMany.RetryAfter)
	}
}

func TestGetOrder_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(srv.URL)
	_, err := c.GetOrder(context.Background(), "123")
	if err == nil {
		t.Error("GetOrder() expected error for 500 response, got nil")
	}
}
