package logger

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func TestInitialize(t *testing.T) {
	t.Cleanup(func() { Log = zap.NewNop() })

	if err := Initialize("info"); err != nil {
		t.Fatalf("Initialize() unexpected error: %v", err)
	}
	if Log == nil {
		t.Error("Log is nil after Initialize()")
	}

	if err := Initialize("not-a-level"); err == nil {
		t.Error("Initialize() expected error for invalid level, got nil")
	}
}

func TestRequestLogger(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("hello"))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	w := httptest.NewRecorder()

	RequestLogger(next).ServeHTTP(w, req)

	if w.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d", w.Code, http.StatusTeapot)
	}
	if w.Body.String() != "hello" {
		t.Errorf("body = %q, want %q", w.Body.String(), "hello")
	}
}
