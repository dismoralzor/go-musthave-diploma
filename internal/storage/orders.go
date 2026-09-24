package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/dismoralzor/go-musthave-diploma/internal/models"
)

// CreateOrder регистрирует номер заказа number за пользователем userID.
// Если номер уже загружен, возвращается ErrOrderOwnedByUser (если это
// сделал тот же пользователь) или ErrOrderOwnedByOther (если другой).
func (s *Storage) CreateOrder(ctx context.Context, number string, userID int64) error {
	const query = `INSERT INTO orders (number, user_id) VALUES ($1, $2)`

	_, err := s.db.ExecContext(ctx, query, number, userID)
	if err == nil {
		return nil
	}

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != pgerrcode.UniqueViolation {
		return fmt.Errorf("storage: create order: %w", err)
	}

	const ownerQuery = `SELECT user_id FROM orders WHERE number = $1`

	var ownerID int64
	if err := s.db.QueryRowContext(ctx, ownerQuery, number).Scan(&ownerID); err != nil {
		return fmt.Errorf("storage: lookup order owner: %w", err)
	}

	if ownerID == userID {
		return ErrOrderOwnedByUser
	}
	return ErrOrderOwnedByOther
}

// GetOrdersByUser возвращает заказы пользователя userID, отсортированные от
// самых новых к самым старым.
func (s *Storage) GetOrdersByUser(ctx context.Context, userID int64) ([]models.Order, error) {
	const query = `
		SELECT number, user_id, status, accrual, uploaded_at
		FROM orders
		WHERE user_id = $1
		ORDER BY uploaded_at DESC`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("storage: get orders by user: %w", err)
	}
	defer rows.Close()

	var orders []models.Order
	for rows.Next() {
		var o models.Order
		if err := rows.Scan(&o.Number, &o.UserID, &o.Status, &o.Accrual, &o.UploadedAt); err != nil {
			return nil, fmt.Errorf("storage: scan order: %w", err)
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: get orders by user: %w", err)
	}

	return orders, nil
}

// staleProcessingTimeout — сколько заказ может провисеть в статусе
// PROCESSING без обновления, прежде чем ClaimOrdersForProcessing сочтёт его
// зависшим (например, из-за падения инстанса воркера) и возьмёт в работу
// повторно.
const staleProcessingTimeout = time.Minute

// ClaimOrdersForProcessing атомарно выбирает до limit заказов, ещё не
// обработанных окончательно (в статусе NEW, либо зависших в PROCESSING
// дольше staleProcessingTimeout), и сразу помечает их как PROCESSING одним
// запросом с FOR UPDATE SKIP LOCKED. Это исключает выборку одного и того же
// заказа несколькими воркерами или инстансами сервиса одновременно: заказ
// считается взятым в работу сразу же, без отдельного шага подтверждения.
func (s *Storage) ClaimOrdersForProcessing(ctx context.Context, limit int) ([]models.Order, error) {
	// interval '%d seconds' подставляется как константа времени компиляции
	// запроса (staleProcessingTimeout), а не как параметр: Postgres не умеет
	// разбирать формат time.Duration.String(), а плейсхолдер типа interval
	// потребовал бы явного приведения на стороне клиента.
	query := fmt.Sprintf(`
		UPDATE orders SET status = $1, updated_at = now()
		WHERE number IN (
			SELECT number FROM orders
			WHERE status = $2 OR (status = $1 AND updated_at < now() - interval '%d seconds')
			ORDER BY uploaded_at
			FOR UPDATE SKIP LOCKED
			LIMIT $3)
		RETURNING number, user_id, status`, int(staleProcessingTimeout.Seconds()))

	rows, err := s.db.QueryContext(ctx, query,
		models.OrderStatusProcessing, models.OrderStatusNew, limit)
	if err != nil {
		return nil, fmt.Errorf("storage: claim orders for processing: %w", err)
	}
	defer rows.Close()

	var orders []models.Order
	for rows.Next() {
		var o models.Order
		if err := rows.Scan(&o.Number, &o.UserID, &o.Status); err != nil {
			return nil, fmt.Errorf("storage: scan claimed order: %w", err)
		}
		orders = append(orders, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: claim orders for processing: %w", err)
	}

	return orders, nil
}

// UpdateOrderStatus обновляет статус и начисление заказа number. Если новый
// статус — PROCESSED и accrual положителен, в той же транзакции пополняет
// баланс пользователя userID на сумму начисления: обновление заказа и
// пополнение баланса должны происходить атомарно.
func (s *Storage) UpdateOrderStatus(ctx context.Context, number, status string, accrual *float64, userID int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin update order status tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const updateOrderQuery = `UPDATE orders SET status = $1, accrual = $2, updated_at = now() WHERE number = $3`

	if _, err := tx.ExecContext(ctx, updateOrderQuery, status, accrual, number); err != nil {
		return fmt.Errorf("storage: update order status: %w", err)
	}

	if status == models.OrderStatusProcessed && accrual != nil && *accrual > 0 {
		const updateBalanceQuery = `UPDATE users SET balance = balance + $1 WHERE id = $2`

		if _, err := tx.ExecContext(ctx, updateBalanceQuery, *accrual, userID); err != nil {
			return fmt.Errorf("storage: update user balance: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit update order status tx: %w", err)
	}

	return nil
}
