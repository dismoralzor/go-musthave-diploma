package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/dismoralzor/go-musthave-diploma/internal/models"
)

// CreateUser создаёт нового пользователя с логином login и хешем пароля
// passwordHash, возвращая его идентификатор. Если логин уже занят,
// возвращается ErrLoginTaken.
func (s *Storage) CreateUser(ctx context.Context, login, passwordHash string) (int64, error) {
	const query = `INSERT INTO users (login, password_hash) VALUES ($1, $2) RETURNING id`

	var id int64
	err := s.db.QueryRowContext(ctx, query, login, passwordHash).Scan(&id)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return 0, ErrLoginTaken
		}
		return 0, fmt.Errorf("storage: create user: %w", err)
	}

	return id, nil
}

// GetUserByLogin возвращает пользователя по логину. Если пользователь не
// найден, возвращается ErrUserNotFound.
func (s *Storage) GetUserByLogin(ctx context.Context, login string) (models.User, error) {
	const query = `SELECT id, login, password_hash FROM users WHERE login = $1`

	var u models.User
	err := s.db.QueryRowContext(ctx, query, login).Scan(&u.ID, &u.Login, &u.PasswordHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.User{}, ErrUserNotFound
		}
		return models.User{}, fmt.Errorf("storage: get user by login: %w", err)
	}

	return u, nil
}
