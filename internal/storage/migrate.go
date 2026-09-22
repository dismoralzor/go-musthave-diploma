package storage

import (
	"database/sql"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	pgxmigrate "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/dismoralzor/go-musthave-diploma/internal/storage/migrations"
)

// migrationsTable — имя служебной таблицы golang-migrate. Задаётся явно и
// отличается от значения по умолчанию ("schema_migrations"), потому что
// сервис accrual работает в той же базе данных и использует свою систему
// миграций: одинаковое имя таблицы привело бы к конфликту между сервисами.
const migrationsTable = "gophermart_schema_migrations"

// Migrate применяет все непримененные миграции схемы к базе данных db.
// Если миграции уже применены (нет изменений), это не считается ошибкой.
func Migrate(db *sql.DB) error {
	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("storage: open migrations source: %w", err)
	}

	dbDriver, err := pgxmigrate.WithInstance(db, &pgxmigrate.Config{
		MigrationsTable: migrationsTable,
	})
	if err != nil {
		return fmt.Errorf("storage: open migrations driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "pgx", dbDriver)
	if err != nil {
		return fmt.Errorf("storage: init migrator: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("storage: apply migrations: %w", err)
	}

	return nil
}
