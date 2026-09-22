package storage

import (
	"context"
	"fmt"

	"github.com/dismoralzor/go-musthave-diploma/internal/models"
)

// GetBalance возвращает текущий баланс и сумму списанных баллов
// пользователя userID.
func (s *Storage) GetBalance(ctx context.Context, userID int64) (models.Balance, error) {
	const query = `SELECT balance, withdrawn FROM users WHERE id = $1`

	var b models.Balance
	if err := s.db.QueryRowContext(ctx, query, userID).Scan(&b.Current, &b.Withdrawn); err != nil {
		return models.Balance{}, fmt.Errorf("storage: get balance: %w", err)
	}

	return b, nil
}

// Withdraw атомарно списывает sum баллов лояльности с баланса пользователя
// userID в счёт оплаты заказа order и фиксирует списание в истории.
// Списание и уменьшение баланса выполняются в одной транзакции, а условие
// balance >= $1 в UPDATE не даёт балансу уйти в минус при конкурентных
// списаниях. Если средств недостаточно, возвращается ErrInsufficientFunds.
func (s *Storage) Withdraw(ctx context.Context, userID int64, order string, sum float64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("storage: begin withdraw tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const updateQuery = `
		UPDATE users
		SET balance = balance - $1, withdrawn = withdrawn + $1
		WHERE id = $2 AND balance >= $1`

	res, err := tx.ExecContext(ctx, updateQuery, sum, userID)
	if err != nil {
		return fmt.Errorf("storage: withdraw balance: %w", err)
	}

	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("storage: withdraw rows affected: %w", err)
	}
	if affected == 0 {
		return ErrInsufficientFunds
	}

	const insertQuery = `INSERT INTO withdrawals (user_id, order_number, sum) VALUES ($1, $2, $3)`

	if _, err := tx.ExecContext(ctx, insertQuery, userID, order, sum); err != nil {
		return fmt.Errorf("storage: insert withdrawal: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit withdraw tx: %w", err)
	}

	return nil
}

// GetWithdrawals возвращает историю списаний пользователя userID от самых
// новых к самым старым.
func (s *Storage) GetWithdrawals(ctx context.Context, userID int64) ([]models.Withdrawal, error) {
	const query = `
		SELECT order_number, sum, processed_at
		FROM withdrawals
		WHERE user_id = $1
		ORDER BY processed_at DESC`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("storage: get withdrawals: %w", err)
	}
	defer rows.Close()

	var withdrawals []models.Withdrawal
	for rows.Next() {
		var wd models.Withdrawal
		if err := rows.Scan(&wd.Order, &wd.Sum, &wd.ProcessedAt); err != nil {
			return nil, fmt.Errorf("storage: scan withdrawal: %w", err)
		}
		withdrawals = append(withdrawals, wd)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("storage: get withdrawals: %w", err)
	}

	return withdrawals, nil
}
