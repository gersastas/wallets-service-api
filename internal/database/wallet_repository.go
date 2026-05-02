package database

import (
	"context"
	"database/sql"

	"github.com/gersastas/wallets-service-api/internal/models"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type WalletRepository struct {
	db *sql.DB
}

func NewWalletRepository(db *sql.DB) *WalletRepository {
	return &WalletRepository{db: db}
}

func (r *WalletRepository) Create(ctx context.Context, wallet *models.Wallet) error {
	query := `
		INSERT INTO wallets (id, user_id, name, balance, currency, created_at, updated_at, deleted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.ExecContext(
		ctx,
		query,
		wallet.ID,
		wallet.UserID,
		wallet.Name,
		wallet.Balance,
		wallet.Currency,
		wallet.CreatedAt,
		wallet.UpdatedAt,
		wallet.DeletedAt,
	)

	return err
}

func (r *WalletRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Wallet, error) {
	query := `
		SELECT id, user_id, name, balance, currency, created_at, updated_at, deleted_at
		FROM wallets
		WHERE id = $1 AND deleted_at IS NULL
	`

	wallet := &models.Wallet{}
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&wallet.ID,
		&wallet.UserID,
		&wallet.Name,
		&wallet.Balance,
		&wallet.Currency,
		&wallet.CreatedAt,
		&wallet.UpdatedAt,
		&wallet.DeletedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return wallet, nil
}

func (r *WalletRepository) Update(ctx context.Context, wallet *models.Wallet) error {
	query := `
		UPDATE wallets
		SET name = $1, updated_at = $2
		WHERE id = $3 AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, wallet.Name, wallet.UpdatedAt, wallet.ID)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *WalletRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `
		UPDATE wallets
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (r *WalletRepository) List(ctx context.Context, userID uuid.UUID, limit, offset int) ([]*models.Wallet, error) {
	query := `
		SELECT id, user_id, name, balance, currency, created_at, updated_at, deleted_at
		FROM wallets
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.QueryContext(ctx, query, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			// Логируем ошибку, но не возвращаем её, так как это defer
			_ = closeErr
		}
	}()

	var wallets []*models.Wallet
	for rows.Next() {
		wallet := &models.Wallet{}
		err := rows.Scan(
			&wallet.ID,
			&wallet.UserID,
			&wallet.Name,
			&wallet.Balance,
			&wallet.Currency,
			&wallet.CreatedAt,
			&wallet.UpdatedAt,
			&wallet.DeletedAt,
		)
		if err != nil {
			return nil, err
		}
		wallets = append(wallets, wallet)
	}

	return wallets, nil
}
