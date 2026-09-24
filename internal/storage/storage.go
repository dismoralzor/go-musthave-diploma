package storage

import "database/sql"

// Storage предоставляет доступ к данным сервиса в PostgreSQL.
type Storage struct {
	db *sql.DB
}

// New создаёт Storage поверх уже открытого пула соединений db.
func New(db *sql.DB) *Storage {
	return &Storage{db: db}
}
