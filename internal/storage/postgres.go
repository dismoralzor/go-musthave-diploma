// Package storage отвечает за подключение к PostgreSQL и применение миграций
// схемы базы данных сервиса gophermart.
package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// pingTimeout — таймаут на проверку соединения с базой данных при открытии.
const pingTimeout = 5 * time.Second

// Open открывает пул соединений с PostgreSQL по строке подключения dsn и
// проверяет его доступность через PingContext с таймаутом pingTimeout.
func Open(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("storage: open database: %w", err)
	}

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(20)
	db.SetConnMaxLifetime(time.Hour)
	db.SetConnMaxIdleTime(10 * time.Minute)

	pingCtx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("storage: ping database: %w", err)
	}

	return db, nil
}
