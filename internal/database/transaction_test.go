package database

import (
	"context"
	"testing"
	"time"

	"github.com/gersastas/wallets-service-api/internal/models"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransactionRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	walletRepo := NewWalletRepository(db)
	txRepo := NewTransactionRepository(db)

	wallet := &models.Wallet{
		ID: uuid.NewString(), UserID: uuid.NewString(),
		Name: "Test", Balance: 0, Currency: "USD",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, walletRepo.Create(context.Background(), wallet))

	tx := &models.Transaction{
		ID:          uuid.NewString(),
		WalletID:    wallet.ID,
		Type:        models.TransactionTypeDeposit,
		Amount:      1000,
		Currency:    "USD",
		Description: "Test deposit",
		CreatedAt:   time.Now(),
	}

	err := txRepo.Create(context.Background(), tx)
	require.NoError(t, err)
}

func TestTransactionRepository_GetByID(t *testing.T) {
	db := setupTestDB(t)
	walletRepo := NewWalletRepository(db)
	txRepo := NewTransactionRepository(db)

	wallet := &models.Wallet{
		ID: uuid.NewString(), UserID: uuid.NewString(),
		Name: "Test", Balance: 0, Currency: "USD",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, walletRepo.Create(context.Background(), wallet))

	tx := &models.Transaction{
		ID:          uuid.NewString(),
		WalletID:    wallet.ID,
		Type:        models.TransactionTypeDeposit,
		Amount:      500,
		Currency:    "USD",
		Description: "Test",
		CreatedAt:   time.Now(),
	}
	require.NoError(t, txRepo.Create(context.Background(), tx))

	got, err := txRepo.GetByID(context.Background(), tx.ID)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, tx.ID, got.ID)
	assert.Equal(t, int64(500), got.Amount)
}

func TestTransactionRepository_GetByIdempotencyKey(t *testing.T) {
	db := setupTestDB(t)
	walletRepo := NewWalletRepository(db)
	txRepo := NewTransactionRepository(db)

	wallet := &models.Wallet{
		ID: uuid.NewString(), UserID: uuid.NewString(),
		Name: "Test", Balance: 0, Currency: "USD",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, walletRepo.Create(context.Background(), wallet))

	key := "test-key-123"
	tx := &models.Transaction{
		ID:             uuid.NewString(),
		WalletID:       wallet.ID,
		Type:           models.TransactionTypeDeposit,
		Amount:         1000,
		Currency:       "USD",
		Description:    "Test",
		IdempotencyKey: &key,
		CreatedAt:      time.Now(),
	}
	require.NoError(t, txRepo.Create(context.Background(), tx))

	got, err := txRepo.GetByIdempotencyKey(context.Background(), key)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, tx.ID, got.ID)
}

func TestTransactionRepository_GetByIdempotencyKey_NotFound(t *testing.T) {
	db := setupTestDB(t)
	txRepo := NewTransactionRepository(db)

	got, err := txRepo.GetByIdempotencyKey(context.Background(), "nonexistent")
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestTransactionRepository_ListByWallet(t *testing.T) {
	db := setupTestDB(t)
	walletRepo := NewWalletRepository(db)
	txRepo := NewTransactionRepository(db)

	wallet := &models.Wallet{
		ID: uuid.NewString(), UserID: uuid.NewString(),
		Name: "Test", Balance: 0, Currency: "USD",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	require.NoError(t, walletRepo.Create(context.Background(), wallet))

	for i := 1; i <= 3; i++ {
		tx := &models.Transaction{
			ID:          uuid.NewString(),
			WalletID:    wallet.ID,
			Type:        models.TransactionTypeDeposit,
			Amount:      int64(100 * i),
			Currency:    "USD",
			Description: "Test",
			CreatedAt:   time.Now(),
		}
		require.NoError(t, txRepo.Create(context.Background(), tx))
	}

	transactions, err := txRepo.ListByWallet(context.Background(), wallet.ID, 10, 0)
	require.NoError(t, err)
	assert.Len(t, transactions, 3)
}
