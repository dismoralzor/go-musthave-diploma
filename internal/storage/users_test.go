package storage

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

// errBoom — фиктивная ошибка для тестов, где неважна её конкретная природа.
var errBoom = errors.New("boom")

func newMockStorage(t *testing.T) (*Storage, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() unexpected error: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	return New(db), mock
}

func TestCreateUser_Success(t *testing.T) {
	s, mock := newMockStorage(t)

	mock.ExpectQuery("INSERT INTO users").
		WithArgs("user1", "hash").
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(1)))

	id, err := s.CreateUser(context.Background(), "user1", "hash")
	if err != nil {
		t.Fatalf("CreateUser() unexpected error: %v", err)
	}
	if id != 1 {
		t.Errorf("id = %d, want 1", id)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestCreateUser_LoginTaken(t *testing.T) {
	s, mock := newMockStorage(t)

	mock.ExpectQuery("INSERT INTO users").
		WithArgs("user1", "hash").
		WillReturnError(&pgconn.PgError{Code: pgerrcode.UniqueViolation})

	_, err := s.CreateUser(context.Background(), "user1", "hash")
	if !errors.Is(err, ErrLoginTaken) {
		t.Errorf("CreateUser() error = %v, want ErrLoginTaken", err)
	}
}

func TestCreateUser_OtherError(t *testing.T) {
	s, mock := newMockStorage(t)

	mock.ExpectQuery("INSERT INTO users").
		WithArgs("user1", "hash").
		WillReturnError(errBoom)

	_, err := s.CreateUser(context.Background(), "user1", "hash")
	if err == nil || errors.Is(err, ErrLoginTaken) {
		t.Errorf("CreateUser() error = %v, want a generic error", err)
	}
}

func TestGetUserByLogin_Success(t *testing.T) {
	s, mock := newMockStorage(t)

	mock.ExpectQuery("SELECT id, login, password_hash FROM users").
		WithArgs("user1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "login", "password_hash"}).AddRow(int64(1), "user1", "hash"))

	u, err := s.GetUserByLogin(context.Background(), "user1")
	if err != nil {
		t.Fatalf("GetUserByLogin() unexpected error: %v", err)
	}
	if u.ID != 1 || u.Login != "user1" || u.PasswordHash != "hash" {
		t.Errorf("GetUserByLogin() = %+v, unexpected", u)
	}
}

func TestGetUserByLogin_NotFound(t *testing.T) {
	s, mock := newMockStorage(t)

	mock.ExpectQuery("SELECT id, login, password_hash FROM users").
		WithArgs("user1").
		WillReturnError(sql.ErrNoRows)

	_, err := s.GetUserByLogin(context.Background(), "user1")
	if !errors.Is(err, ErrUserNotFound) {
		t.Errorf("GetUserByLogin() error = %v, want ErrUserNotFound", err)
	}
}
