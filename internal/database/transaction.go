package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/gersastas/wallets-service-api/internal/models"
	"github.com/google/uuid"
)

type TransactionRepository struct {
	db *sql.DB
}

func NewTransactionRepository(db *sql.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

func (r *TransactionRepository) Create(ctx context.Context, tx *models.Transaction) error {
	query := `
		INSERT INTO transactions (
			id, wallet_id, type, amount, currency, 
			from_wallet_id, to_wallet_id, description, 
			idempotency_key, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		tx.ID,
		tx.WalletID,
		tx.Type,
		tx.Amount,
		tx.Currency,
		tx.FromWalletID,
		tx.ToWalletID,
		tx.Description,
		tx.IdempotencyKey,
		tx.CreatedAt,
	)

	return err
}

func (r *TransactionRepository) CreateWithTx(ctx context.Context, dbTx *sql.Tx, tx *models.Transaction) error {
	query := `
		INSERT INTO transactions (
			id, wallet_id, type, amount, currency, 
			from_wallet_id, to_wallet_id, description, 
			idempotency_key, created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := dbTx.ExecContext(
		ctx,
		query,
		tx.ID,
		tx.WalletID,
		tx.Type,
		tx.Amount,
		tx.Currency,
		tx.FromWalletID,
		tx.ToWalletID,
		tx.Description,
		tx.IdempotencyKey,
		tx.CreatedAt,
	)

	return err
}

func (r *TransactionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Transaction, error) {
	query := `
		SELECT 
			id, wallet_id, type, amount, currency,
			from_wallet_id, to_wallet_id, description,
			idempotency_key, created_at
		FROM transactions
		WHERE id = $1
	`

	tx := &models.Transaction{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&tx.ID,
		&tx.WalletID,
		&tx.Type,
		&tx.Amount,
		&tx.Currency,
		&tx.FromWalletID,
		&tx.ToWalletID,
		&tx.Description,
		&tx.IdempotencyKey,
		&tx.CreatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (r *TransactionRepository) GetByIdempotencyKey(ctx context.Context, key string) (*models.Transaction, error) {
	query := `
		SELECT 
			id, wallet_id, type, amount, currency,
			from_wallet_id, to_wallet_id, description,
			idempotency_key, created_at
		FROM transactions
		WHERE idempotency_key = $1
	`

	tx := &models.Transaction{}
	err := r.db.QueryRowContext(ctx, query, key).Scan(
		&tx.ID,
		&tx.WalletID,
		&tx.Type,
		&tx.Amount,
		&tx.Currency,
		&tx.FromWalletID,
		&tx.ToWalletID,
		&tx.Description,
		&tx.IdempotencyKey,
		&tx.CreatedAt,
	)

	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return tx, nil
}

func (r *TransactionRepository) ListByWallet(ctx context.Context, walletID uuid.UUID, limit, offset int) ([]*models.Transaction, error) {
	query := `
		SELECT 
			id, wallet_id, type, amount, currency,
			from_wallet_id, to_wallet_id, description,
			idempotency_key, created_at
		FROM transactions
		WHERE wallet_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryContext(ctx, query, walletID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			_ = closeErr
		}
	}()

	var transactions []*models.Transaction
	for rows.Next() {
		tx := &models.Transaction{}
		err := rows.Scan(
			&tx.ID,
			&tx.WalletID,
			&tx.Type,
			&tx.Amount,
			&tx.Currency,
			&tx.FromWalletID,
			&tx.ToWalletID,
			&tx.Description,
			&tx.IdempotencyKey,
			&tx.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, tx)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return transactions, nil
}

func (r *TransactionRepository) GetWalletBalance(ctx context.Context, walletID uuid.UUID) (int64, error) {
	query := `
		SELECT COALESCE(
			SUM(
				CASE 
					WHEN type = 'deposit' THEN amount
					WHEN type = 'withdraw' THEN -amount
					WHEN type = 'transfer' AND to_wallet_id = $1 THEN amount
					WHEN type = 'transfer' AND from_wallet_id = $1 THEN -amount
					ELSE 0
				END
			), 
			0
		) as balance
		FROM transactions
		WHERE wallet_id = $1
	`

	var balance int64
	err := r.db.QueryRowContext(ctx, query, walletID).Scan(&balance)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate balance: %w", err)
	}

	return balance, nil
}