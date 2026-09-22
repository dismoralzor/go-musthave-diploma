package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dismoralzor/go-musthave-diploma/internal/auth"
	"github.com/dismoralzor/go-musthave-diploma/internal/models"
	"github.com/dismoralzor/go-musthave-diploma/internal/storage"
)

// errBoom — фиктивная ошибка для тестов внутренних сбоев хранилища.
var errBoom = errors.New("boom")

func TestRegister(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		createUser func(ctx context.Context, login, passwordHash string) (int64, error)
		wantStatus int
	}{
		{
			name: "success",
			body: `{"login":"user1","password":"pass1"}`,
			createUser: func(ctx context.Context, login, passwordHash string) (int64, error) {
				return 1, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid json",
			body:       `not json`,
			createUser: func(ctx context.Context, login, passwordHash string) (int64, error) { return 0, nil },
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty login",
			body:       `{"login":"","password":"pass1"}`,
			createUser: func(ctx context.Context, login, passwordHash string) (int64, error) { return 0, nil },
			wantStatus: http.StatusBadRequest,
		},
		{
			name: "login taken",
			body: `{"login":"user1","password":"pass1"}`,
			createUser: func(ctx context.Context, login, passwordHash string) (int64, error) {
				return 0, storage.ErrLoginTaken
			},
			wantStatus: http.StatusConflict,
		},
		{
			name: "internal error",
			body: `{"login":"user1","password":"pass1"}`,
			createUser: func(ctx context.Context, login, passwordHash string) (int64, error) {
				return 0, errBoom
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &fakeUserStore{createUser: tt.createUser}
			router := newTestRouter(t, users, nil, nil)

			req := httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(tt.body))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantStatus == http.StatusOK {
				if w.Header().Get("Authorization") == "" {
					t.Error("expected Authorization header on success")
				}
				if len(w.Result().Cookies()) == 0 {
					t.Error("expected Set-Cookie on success")
				}
			}
		})
	}
}

func TestLogin(t *testing.T) {
	hash, err := auth.Hash("correct-password")
	if err != nil {
		t.Fatalf("auth.Hash() unexpected error: %v", err)
	}

	tests := []struct {
		name           string
		body           string
		getUserByLogin func(ctx context.Context, login string) (models.User, error)
		wantStatus     int
	}{
		{
			name: "success",
			body: `{"login":"user1","password":"correct-password"}`,
			getUserByLogin: func(ctx context.Context, login string) (models.User, error) {
				return models.User{ID: 1, Login: "user1", PasswordHash: hash}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name:           "invalid json",
			body:           `not json`,
			getUserByLogin: func(ctx context.Context, login string) (models.User, error) { return models.User{}, nil },
			wantStatus:     http.StatusBadRequest,
		},
		{
			name: "user not found",
			body: `{"login":"user1","password":"correct-password"}`,
			getUserByLogin: func(ctx context.Context, login string) (models.User, error) {
				return models.User{}, storage.ErrUserNotFound
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "wrong password",
			body: `{"login":"user1","password":"wrong-password"}`,
			getUserByLogin: func(ctx context.Context, login string) (models.User, error) {
				return models.User{ID: 1, Login: "user1", PasswordHash: hash}, nil
			},
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := &fakeUserStore{getUserByLogin: tt.getUserByLogin}
			router := newTestRouter(t, users, nil, nil)

			req := httptest.NewRequest(http.MethodPost, "/api/user/login", strings.NewReader(tt.body))
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestProtectedRoutes_RequireAuth(t *testing.T) {
	router := newTestRouter(t, nil, nil, nil)

	targets := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/user/orders"},
		{http.MethodGet, "/api/user/orders"},
		{http.MethodGet, "/api/user/balance"},
		{http.MethodPost, "/api/user/balance/withdraw"},
		{http.MethodGet, "/api/user/withdrawals"},
	}

	for _, target := range targets {
		t.Run(target.method+" "+target.path, func(t *testing.T) {
			req := httptest.NewRequest(target.method, target.path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want %d", w.Code, http.StatusUnauthorized)
			}
		})
	}
}
